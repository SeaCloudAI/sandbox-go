package cmd

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/SeaCloudAI/sandbox-go/core"
)

const (
	ConnectProtocolVersion = "1"
)

type Service struct {
	baseURL     *url.URL
	accessToken string
	httpClient  *http.Client
	userAgent   string
	logger      core.DiagnosticLogger
}

type ServiceOption func(*Service)

var diagnosticURLPattern = regexp.MustCompile(`https?://[^\s"'<>]+`)

func WithLogger(logger core.DiagnosticLogger) ServiceOption {
	return func(c *Service) {
		c.logger = logger
	}
}

func WithDebugLogger() ServiceOption {
	return WithLogger(func(event core.DiagnosticEvent) {
		log.Printf("seacloudai-sandbox-cmd type=%s method=%s path=%s request_id=%s status=%d duration_ms=%d error_kind=%s retryable=%v error=%s",
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

func NewService(baseURL, accessToken string, opts ...ServiceOption) (*Service, error) {
	if strings.TrimSpace(baseURL) == "" {
		return nil, ErrBaseURLEmpty
	}

	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, &url.Error{Op: "parse", URL: baseURL, Err: ErrInvalidBaseURL}
	}

	service := &Service{
		baseURL:     parsed,
		accessToken: strings.TrimSpace(accessToken),
		httpClient:  &http.Client{Timeout: 30 * time.Second},
		userAgent:   core.UserAgent("seacloudai-sandbox-go-cmd"),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service, nil
}

func (c *Service) BaseURL() string {
	return c.baseURL.String()
}

func (c *Service) newRequest(
	ctx context.Context,
	method, path string,
	query url.Values,
	body io.Reader,
	contentType string,
	accept string,
	opts *RequestOptions,
) (*http.Request, error) {
	endpoint, err := c.resolve(path, query)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}

	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("User-Agent", c.userAgent)
	if c.accessToken != "" {
		req.Header.Set("X-Access-Token", c.accessToken)
	}
	ensureRequestID(req.Header)
	if opts != nil {
		for key, values := range opts.Headers {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
	}
	return req, nil
}

func (c *Service) doJSON(
	ctx context.Context,
	method, path string,
	query url.Values,
	body any,
	out any,
	contentType string,
	accept string,
	opts *RequestOptions,
	expectedStatus ...int,
) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}
	if contentType == "" && body != nil {
		contentType = "application/json"
	}
	if accept == "" {
		accept = "application/json"
	}

	resp, err := c.do(ctx, method, path, query, reader, contentType, accept, opts, expectedStatus...)
	if err != nil {
		return nil, err
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return resp, nil
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Service) do(
	ctx context.Context,
	method, path string,
	query url.Values,
	body io.Reader,
	contentType string,
	accept string,
	opts *RequestOptions,
	expectedStatus ...int,
) (*http.Response, error) {
	req, err := c.newRequest(ctx, method, path, query, body, contentType, accept, opts)
	if err != nil {
		return nil, err
	}

	started := time.Now()
	c.emitDiagnostic(core.DiagnosticEvent{
		Type:      "request",
		Method:    req.Method,
		Path:      sanitizeDiagnosticPath(req.URL),
		RequestID: req.Header.Get("X-Request-ID"),
	})
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.emitDiagnostic(core.DiagnosticEvent{
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
		c.emitDiagnostic(core.DiagnosticEvent{
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
	err = core.DecodeAPIError(resp)
	c.emitAPIError(req, err, time.Since(started))
	return nil, err
}

func (c *Service) emitAPIError(req *http.Request, err error, duration time.Duration) {
	event := core.DiagnosticEvent{
		Type:      "error",
		Method:    req.Method,
		Path:      sanitizeDiagnosticPath(req.URL),
		RequestID: req.Header.Get("X-Request-ID"),
		Duration:  duration,
		Error:     sanitizeDiagnosticError(err.Error()),
	}
	if apiErr, ok := err.(*core.APIError); ok {
		if apiErr.RequestID != "" {
			event.RequestID = apiErr.RequestID
		}
		event.StatusCode = apiErr.StatusCode
		event.ErrorKind = apiErr.Kind
		event.Retryable = apiErr.Retryable()
	}
	c.emitDiagnostic(event)
}

func (c *Service) emitDiagnostic(event core.DiagnosticEvent) {
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

func (c *Service) resolve(path string, query url.Values) (string, error) {
	normalized := strings.TrimSpace(path)
	if normalized == "" {
		normalized = "/"
	}
	if !strings.HasPrefix(normalized, "/") {
		normalized = "/" + normalized
	}

	base := *c.baseURL
	base.Path = joinBasePath(c.baseURL.Path, normalized)
	base.RawPath = ""
	base.RawQuery = ""
	base.Fragment = ""
	if len(query) > 0 {
		base.RawQuery = query.Encode()
	}
	return base.String(), nil
}

func statusAllowed(statusCode int, expected []int) bool {
	for _, code := range expected {
		if statusCode == code {
			return true
		}
	}
	return false
}

func withConnectRPC(opts *RequestOptions) *RequestOptions {
	return cloneOptionsWithHeaders(opts, http.Header{
		"Connect-Protocol-Version": []string{ConnectProtocolVersion},
	})
}

func withBasicUsername(opts *RequestOptions) *RequestOptions {
	if opts == nil || strings.TrimSpace(opts.Username) == "" {
		return opts
	}
	cloned := cloneOptionsWithHeaders(opts, nil)
	if cloned.Headers.Get("Authorization") == "" {
		cloned.Headers.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(opts.Username)+":")))
	}
	return cloned
}

func fileQuery(path string, opts *RequestOptions) (url.Values, error) {
	if strings.TrimSpace(path) == "" {
		return nil, ErrPathEmpty
	}
	query := make(url.Values)
	query.Set("path", path)
	if opts == nil {
		return query, nil
	}
	if username := strings.TrimSpace(opts.Username); username != "" {
		query.Set("username", username)
	}
	if signature := strings.TrimSpace(opts.Signature); signature != "" {
		query.Set("signature", signature)
	}
	if opts.SignatureExpiration != nil {
		query.Set("signature_expiration", strconv.FormatInt(*opts.SignatureExpiration, 10))
	}
	return query, nil
}

func cloneOptionsWithHeaders(opts *RequestOptions, extra http.Header) *RequestOptions {
	if opts == nil && extra == nil {
		return nil
	}

	cloned := &RequestOptions{}
	if opts != nil {
		*cloned = *opts
	}
	cloned.Headers = make(http.Header)
	if opts != nil {
		for key, values := range opts.Headers {
			copied := append([]string(nil), values...)
			cloned.Headers[key] = copied
		}
	}
	for key, values := range extra {
		for _, value := range values {
			cloned.Headers.Add(key, value)
		}
	}
	return cloned
}

func queryFromOptions(opts *RequestOptions) url.Values {
	query := make(url.Values)
	if opts == nil {
		return query
	}
	if username := strings.TrimSpace(opts.Username); username != "" {
		query.Set("username", username)
	}
	if signature := strings.TrimSpace(opts.Signature); signature != "" {
		query.Set("signature", signature)
	}
	if opts.SignatureExpiration != nil {
		query.Set("signature_expiration", strconv.FormatInt(*opts.SignatureExpiration, 10))
	}
	return query
}

func joinBasePath(basePath, reqPath string) string {
	trimmedBase := strings.TrimRight(strings.TrimSpace(basePath), "/")
	trimmedReq := strings.TrimLeft(strings.TrimSpace(reqPath), "/")

	switch {
	case trimmedBase == "" && trimmedReq == "":
		return "/"
	case trimmedBase == "":
		return "/" + trimmedReq
	case trimmedReq == "":
		return trimmedBase
	default:
		return trimmedBase + "/" + trimmedReq
	}
}

func gzipBytes(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeMultipart(parts []MultipartFile) (*bytes.Buffer, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for _, part := range parts {
		fieldName := strings.TrimSpace(part.FieldName)
		if fieldName == "" {
			fieldName = "file"
		}

		var w io.Writer
		if strings.TrimSpace(part.FileName) != "" {
			header := make(textproto.MIMEHeader)
			header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, fieldName, part.FileName))
			if strings.TrimSpace(part.ContentType) != "" {
				header.Set("Content-Type", part.ContentType)
			}
			var err error
			w, err = writer.CreatePart(header)
			if err != nil {
				_ = writer.Close()
				return nil, "", err
			}
		} else {
			var err error
			w, err = writer.CreateFormField(fieldName)
			if err != nil {
				_ = writer.Close()
				return nil, "", err
			}
		}

		if _, err := w.Write(part.Data); err != nil {
			_ = writer.Close()
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return &buf, writer.FormDataContentType(), nil
}
