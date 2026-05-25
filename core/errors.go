package core

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

var (
	ErrBaseURLEmpty   = errors.New("sandbox: baseURL is required")
	ErrAPIKeyEmpty    = errors.New("sandbox: apiKey is required")
	ErrInvalidBaseURL = errors.New("sandbox: baseURL must include scheme and host")
	ErrUserIDEmpty    = errors.New("sandbox: userID is required")
	ErrSandboxIDEmpty = errors.New("sandbox: sandboxID is required")
	ErrTemplateEmpty  = errors.New("sandbox: templateID is required")
)

type APIErrorKind string

const (
	APIErrorKindUnknown        APIErrorKind = "unknown"
	APIErrorKindAuthentication APIErrorKind = "authentication"
	APIErrorKindPermission     APIErrorKind = "permission"
	APIErrorKindNotFound       APIErrorKind = "not_found"
	APIErrorKindConflict       APIErrorKind = "conflict"
	APIErrorKindRateLimit      APIErrorKind = "rate_limit"
	APIErrorKindTimeout        APIErrorKind = "timeout"
	APIErrorKindServer         APIErrorKind = "server"
)

// ErrorDetail is the structured error payload used by Atlas standard responses.
type ErrorDetail struct {
	Code    string `json:"code"`
	Details string `json:"details,omitempty"`
}

// UsageLimitDiagnostic is the public details payload returned with quota 429s.
type UsageLimitDiagnostic struct {
	Reason        string  `json:"reason"`
	Scope         string  `json:"scope,omitempty"`
	Resource      string  `json:"resource,omitempty"`
	Metric        string  `json:"metric,omitempty"`
	Used          int     `json:"used,omitempty"`
	Limit         int     `json:"limit,omitempty"`
	Remaining     int     `json:"remaining"`
	ResetAt       *string `json:"resetAt,omitempty"`
	Retryable     bool    `json:"retryable"`
	UsageEndpoint string  `json:"usageEndpoint,omitempty"`
}

// APIError represents a non-2xx API response.
type APIError struct {
	StatusCode int
	Code       int
	Message    string
	RequestID  string
	Err        *ErrorDetail
	Details    json.RawMessage
	UsageLimit *UsageLimitDiagnostic
	Body       []byte
	Kind       APIErrorKind
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil && e.Err.Details != "" {
		return e.Err.Details
	}
	if e.Message != "" {
		return e.Message
	}
	return "sandbox: request failed"
}

func (e *APIError) Retryable() bool {
	if e == nil {
		return false
	}
	return e.Kind == APIErrorKindRateLimit || e.Kind == APIErrorKindTimeout || e.Kind == APIErrorKindServer
}

type rawAPIError struct {
	Code           int             `json:"code"`
	Message        string          `json:"message"`
	Err            json.RawMessage `json:"error,omitempty"`
	Details        json.RawMessage `json:"details,omitempty"`
	RequestID      string          `json:"request_id,omitempty"`
	RequestIDCamel string          `json:"requestID,omitempty"`
}

// DecodeAPIError converts a non-success response into APIError.
func DecodeAPIError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	apiErr := &APIError{
		StatusCode: resp.StatusCode,
		Message:    resp.Status,
		Body:       body,
		Kind:       classifyAPIError(resp.StatusCode),
	}

	var parsed rawAPIError
	if len(body) > 0 && json.Unmarshal(body, &parsed) == nil {
		if parsed.Code != 0 {
			apiErr.Code = parsed.Code
		}
		if parsed.Message != "" {
			apiErr.Message = parsed.Message
		}
		apiErr.RequestID = firstNonEmpty(parsed.RequestIDCamel, parsed.RequestID)
		apiErr.Err = decodeErrorDetail(parsed.Err)
		apiErr.Details = cloneRawMessage(parsed.Details)
		apiErr.UsageLimit = decodeUsageLimitDiagnostic(parsed.Details)
	}
	return apiErr
}

func decodeErrorDetail(raw json.RawMessage) *ErrorDetail {
	if len(raw) == 0 {
		return nil
	}
	// Some runtime routes return {"error":"not found"} instead of the standard error object.
	var detail ErrorDetail
	if json.Unmarshal(raw, &detail) == nil {
		return &detail
	}
	var message string
	if json.Unmarshal(raw, &message) == nil && message != "" {
		return &ErrorDetail{Details: message}
	}
	return nil
}

func decodeUsageLimitDiagnostic(raw json.RawMessage) *UsageLimitDiagnostic {
	if len(raw) == 0 {
		return nil
	}
	var diagnostic UsageLimitDiagnostic
	if json.Unmarshal(raw, &diagnostic) != nil || diagnostic.Reason != "usage_limit" {
		return nil
	}
	return &diagnostic
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func classifyAPIError(statusCode int) APIErrorKind {
	switch statusCode {
	case http.StatusUnauthorized:
		return APIErrorKindAuthentication
	case http.StatusForbidden:
		return APIErrorKindPermission
	case http.StatusNotFound:
		return APIErrorKindNotFound
	case http.StatusRequestTimeout:
		return APIErrorKindTimeout
	case http.StatusConflict:
		return APIErrorKindConflict
	case http.StatusTooManyRequests:
		return APIErrorKindRateLimit
	default:
		if statusCode >= http.StatusInternalServerError {
			return APIErrorKindServer
		}
		return APIErrorKindUnknown
	}
}
