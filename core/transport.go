package core

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Transport is the shared HTTP entry point for the SDK.
type Transport struct {
	baseURL    *url.URL
	apiKey     string
	httpClient *http.Client
	userAgent  string
	projectID  string
	logger     DiagnosticLogger
}

type TransportOption func(*Transport)

// DiagnosticEvent is emitted by optional SDK diagnostic loggers.
// It intentionally excludes headers and bodies to avoid leaking credentials.
type DiagnosticEvent struct {
	Type       string
	Method     string
	Path       string
	RequestID  string
	StatusCode int
	Duration   time.Duration
	Error      string
	ErrorKind  APIErrorKind
	Retryable  bool
}

// DiagnosticLogger receives sanitized request lifecycle events.
type DiagnosticLogger func(DiagnosticEvent)

var diagnosticURLPattern = regexp.MustCompile(`https?://[^\s"'<>]+`)

// WithHTTPClient replaces the default HTTP client for custom transport, proxy, or timeout control.
func WithHTTPClient(httpClient *http.Client) TransportOption {
	return func(c *Transport) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// WithTimeout extends client-side request deadlines for long waitReady or polling calls.
func WithTimeout(timeout time.Duration) TransportOption {
	return func(c *Transport) {
		if timeout > 0 {
			c.httpClient.Timeout = timeout
		}
	}
}

// WithProjectID injects X-Project-ID on every control/build-plane request.
func WithProjectID(projectID string) TransportOption {
	return func(c *Transport) {
		c.projectID = strings.TrimSpace(projectID)
	}
}

// WithLogger enables sanitized request diagnostics.
func WithLogger(logger DiagnosticLogger) TransportOption {
	return func(c *Transport) {
		c.logger = logger
	}
}

// WithDebugLogger writes sanitized request diagnostics with the standard logger.
func WithDebugLogger() TransportOption {
	return WithLogger(func(event DiagnosticEvent) {
		log.Printf("seacloudai-sandbox type=%s method=%s path=%s request_id=%s status=%d duration_ms=%d error_kind=%s retryable=%v error=%s",
			event.Type,
			event.Method,
			event.Path,
			event.RequestID,
			event.StatusCode,
			event.Duration.Milliseconds(),
			event.ErrorKind,
			event.Retryable,
			event.Error,
		)
	})
}

// NewTransport creates a shared authenticated transport for X-API-Key requests.
func NewTransport(baseURL, apiKey string, opts ...TransportOption) (*Transport, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, ErrBaseURLEmpty
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, ErrAPIKeyEmpty
	}

	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, &url.Error{Op: "parse", URL: baseURL, Err: ErrInvalidBaseURL}
	}

	transport := &Transport{
		baseURL:    parsed,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		userAgent:  UserAgent("seacloudai-sandbox-go"),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(transport)
		}
	}
	return transport, nil
}

// BaseURL returns the normalized base URL.
func (c *Transport) BaseURL() string {
	return c.baseURL.String()
}

// NewRequest prepares an authenticated request against the configured API host.
func (c *Transport) NewRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	endpoint, err := c.resolve(path)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("User-Agent", c.userAgent)
	if c.projectID != "" {
		req.Header.Set("X-Project-ID", c.projectID)
	}
	ensureRequestID(req.Header)
	return req, nil
}

// Do sends a prepared request with the shared HTTP transport.
func (c *Transport) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

// DoJSON sends a JSON request, validates the response status, and decodes the body.
func (c *Transport) DoJSON(
	ctx context.Context,
	method, path string,
	headers http.Header,
	query url.Values,
	body any,
	out any,
	expectedStatus ...int,
) (*http.Response, error) {
	resp, err := c.DoRequest(ctx, method, path, headers, query, body, expectedStatus...)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if out == nil || resp.StatusCode == http.StatusNoContent {
		return resp, nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return nil, err
	}
	return resp, nil
}

// DoRequest sends a request with optional JSON body and query string.
func (c *Transport) DoRequest(
	ctx context.Context,
	method, path string,
	headers http.Header,
	query url.Values,
	body any,
	expectedStatus ...int,
) (*http.Response, error) {
	endpoint := path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}

	req, err := c.NewRequest(ctx, method, endpoint, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	started := time.Now()
	c.emitDiagnostic(DiagnosticEvent{
		Type:      "request",
		Method:    req.Method,
		Path:      sanitizeDiagnosticPath(req.URL),
		RequestID: req.Header.Get("X-Request-ID"),
	})
	resp, err := c.Do(req)
	if err != nil {
		c.emitDiagnostic(DiagnosticEvent{
			Type:      "error",
			Method:    req.Method,
			Path:      sanitizeDiagnosticPath(req.URL),
			RequestID: req.Header.Get("X-Request-ID"),
			Duration:  time.Since(started),
			Error:     sanitizeDiagnosticError(err.Error()),
		})
		return nil, err
	}
	if statusAllowed(resp.StatusCode, expectedStatus) {
		c.emitDiagnostic(DiagnosticEvent{
			Type:       "response",
			Method:     req.Method,
			Path:       sanitizeDiagnosticPath(req.URL),
			RequestID:  req.Header.Get("X-Request-ID"),
			StatusCode: resp.StatusCode,
			Duration:   time.Since(started),
		})
		return resp, nil
	}

	defer resp.Body.Close()
	err = DecodeAPIError(resp)
	c.emitAPIError(req, err, time.Since(started))
	return nil, err
}

func (c *Transport) resolve(path string) (string, error) {
	normalized := strings.TrimSpace(path)
	if normalized == "" {
		normalized = "/"
	}
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}

	ref, err := url.Parse(normalized)
	if err != nil {
		return "", err
	}
	return c.baseURL.ResolveReference(ref).String(), nil
}

func (c *Transport) emitAPIError(req *http.Request, err error, duration time.Duration) {
	event := DiagnosticEvent{
		Type:      "error",
		Method:    req.Method,
		Path:      sanitizeDiagnosticPath(req.URL),
		RequestID: req.Header.Get("X-Request-ID"),
		Duration:  duration,
		Error:     sanitizeDiagnosticError(err.Error()),
	}
	if apiErr, ok := err.(*APIError); ok {
		event.RequestID = firstNonEmpty(apiErr.RequestID, event.RequestID)
		event.StatusCode = apiErr.StatusCode
		event.ErrorKind = apiErr.Kind
		event.Retryable = apiErr.Retryable()
	}
	c.emitDiagnostic(event)
}

func (c *Transport) emitDiagnostic(event DiagnosticEvent) {
	if c.logger != nil {
		defer func() {
			_ = recover()
		}()
		c.logger(event)
	}
}

func ensureRequestID(headers http.Header) string {
	if value := strings.TrimSpace(headers.Get("X-Request-ID")); value != "" {
		return value
	}
	value := generateRequestID()
	headers.Set("X-Request-ID", value)
	return value
}

func generateRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return "sdk-" + time.Now().Format("20060102150405.000000000")
}

func sanitizeDiagnosticPath(u *url.URL) string {
	if u == nil {
		return ""
	}
	clone := *u
	query := clone.Query()
	for key := range query {
		if isSensitiveQueryKey(key) {
			query.Set(key, "<redacted>")
		}
	}
	clone.RawQuery = query.Encode()
	if clone.RawQuery == "" {
		return clone.EscapedPath()
	}
	return clone.EscapedPath() + "?" + clone.RawQuery
}

func sanitizeDiagnosticError(message string) string {
	return diagnosticURLPattern.ReplaceAllStringFunc(message, func(raw string) string {
		parsed, err := url.Parse(raw)
		if err != nil {
			return "<redacted-url>"
		}
		return sanitizeDiagnosticPath(parsed)
	})
}

func isSensitiveQueryKey(key string) bool {
	normalized := strings.ToLower(key)
	return strings.Contains(normalized, "token") || strings.Contains(normalized, "signature") || normalized == "api_key"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func statusAllowed(statusCode int, expected []int) bool {
	for _, code := range expected {
		if statusCode == code {
			return true
		}
	}
	return false
}
