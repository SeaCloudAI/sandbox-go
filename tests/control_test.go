package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SeaCloudAI/sandbox-go"
	"github.com/SeaCloudAI/sandbox-go/control"
	"github.com/SeaCloudAI/sandbox-go/core"
)

func TestCreateSandbox(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/sandboxes" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Namespace-ID"); got != "" {
			t.Fatalf("unexpected namespace header = %q", got)
		}
		if got := r.Header.Get("X-User-ID"); got != "" {
			t.Fatalf("unexpected user header = %q", got)
		}
		if got := r.Header.Get("X-Project-ID"); got != "" {
			t.Fatalf("unexpected project header = %q", got)
		}

		var req control.NewSandboxRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.TemplateID != "base" {
			t.Fatalf("templateID = %q", req.TemplateID)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"templateID":"base",
			"sandboxID":"sb-123",
			"clientID":"user-1",
			"envdVersion":"atlas-0.1.0",
			"envdAccessToken":"unit-runtime-auth",
			"envdUrl":"https://sandbox-gateway.cloud.seaart.ai",
			"trafficAccessToken":null,
			"status":"starting",
			"state":"starting",
			"startedAt":"2024-01-01T00:00:00Z",
			"endAt":"2024-01-01T01:00:00Z"
		}`))
	}))
	defer server.Close()

	client, err := sandbox.NewClient(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	resp, err := client.CreateSandbox(context.Background(), &control.NewSandboxRequest{TemplateID: "base"})
	if err != nil {
		t.Fatalf("CreateSandbox: %v", err)
	}
	if resp.SandboxID != "sb-123" {
		t.Fatalf("sandboxID = %q", resp.SandboxID)
	}
	runtime, err := resp.Runtime()
	if err != nil {
		t.Fatalf("Runtime: %v", err)
	}
	if got := runtime.BaseURL(); got != "https://sandbox-gateway.cloud.seaart.ai" {
		t.Fatalf("runtime baseURL = %q", got)
	}
}

func TestListSandboxesEncodesMetadataAndState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got := q.Get("metadata"); got != "app=prod&team=atlas" {
			t.Fatalf("metadata = %q", got)
		}
		if states := q["state"]; len(states) != 2 || states[0] != "running" || states[1] != "paused" {
			t.Fatalf("state = %#v", states)
		}
		if got := q.Get("limit"); got != "10" {
			t.Fatalf("limit = %q", got)
		}
		if got := q.Get("nextToken"); got != "20" {
			t.Fatalf("nextToken = %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	service, err := control.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = service.ListSandboxes(context.Background(), &control.ListSandboxesParams{
		Metadata:  map[string]string{"app": "prod", "team": "atlas"},
		State:     []string{"running", "paused"},
		Limit:     10,
		NextToken: "20",
	})
	if err != nil {
		t.Fatalf("ListSandboxes: %v", err)
	}
}

func TestRootListSandboxesReturnsBoundHandles(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/sandboxes" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`[{
				"sandboxID":"sb-1",
				"clientID":"user-1",
				"envdVersion":"v1",
				"status":"running"
			}]`))
		case strings.HasSuffix(r.URL.Path, "/logs"):
			_, _ = w.Write([]byte(`{"logs":[]}`))
		default:
			_, _ = w.Write([]byte(`{
				"sandboxID":"sb-1",
				"clientID":"user-1",
				"envdVersion":"atlas-0.1.0",
				"envdAccessToken":"unit-runtime-auth",
				"envdUrl":"https://sandbox-gateway.cloud.seaart.ai",
				"status":"running",
				"startedAt":"2024-01-01T00:00:00Z",
				"endAt":"2024-01-01T01:00:00Z"
			}`))
		}
	}))
	defer server.Close()

	client, err := sandbox.NewClient(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	listed, err := client.ListSandboxes(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListSandboxes: %v", err)
	}
	if len(listed) != 1 || listed[0].SandboxID != "sb-1" {
		t.Fatalf("listed = %#v", listed)
	}

	detail, err := listed[0].Reload(context.Background())
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if _, err := detail.Runtime(); err != nil {
		t.Fatalf("Runtime: %v", err)
	}
	if _, err := listed[0].Logs(context.Background(), nil); err != nil {
		t.Fatalf("Logs: %v", err)
	}
	if len(calls) != 3 {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestWrappedResponseDecoding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"code": 0,
			"message": "success",
			"data": {
				"received": true,
				"status": "healthy"
			},
			"request_id": "req-123"
		}`))
	}))
	defer server.Close()

	service, err := control.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.SendHeartbeat(context.Background(), "sb-1", &control.HeartbeatRequest{Status: "healthy"})
	if err != nil {
		t.Fatalf("SendHeartbeat: %v", err)
	}
	if resp.RequestID != "req-123" {
		t.Fatalf("requestID = %q", resp.RequestID)
	}
}

func TestAPIErrorDecoding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{
			"code": 10001,
			"message": "error",
			"error": {
				"code": "INVALID_REQUEST",
				"details": "templateId is required"
			},
			"request_id": "req-456"
		}`))
	}))
	defer server.Close()

	service, err := control.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = service.StartRollingUpdate(context.Background(), &control.RollingStartRequest{TemplateID: "default"})
	if err == nil {
		t.Fatal("expected error")
	}

	apiErr, ok := err.(*core.APIError)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if apiErr.Code != 10001 {
		t.Fatalf("code = %d", apiErr.Code)
	}
	if apiErr.RequestID != "req-456" {
		t.Fatalf("requestID = %q", apiErr.RequestID)
	}
	if apiErr.Err == nil || apiErr.Err.Details != "templateId is required" {
		t.Fatalf("details = %#v", apiErr.Err)
	}
	if apiErr.Kind != core.APIErrorKindUnknown {
		t.Fatalf("kind = %q", apiErr.Kind)
	}
}

func TestAPIErrorDecodingStringDetail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer server.Close()

	service, err := control.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = service.GetSandbox(context.Background(), "sb-1")
	if err == nil {
		t.Fatal("expected error")
	}

	apiErr, ok := err.(*core.APIError)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if apiErr.Kind != core.APIErrorKindNotFound {
		t.Fatalf("kind = %q", apiErr.Kind)
	}
	if apiErr.Err == nil || apiErr.Err.Details != "not found" {
		t.Fatalf("details = %#v", apiErr.Err)
	}
	if err.Error() != "not found" {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestAPIErrorKindClassification(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"code":429,"message":"rate limited"}`))
	}))
	defer server.Close()

	service, err := control.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = service.GetSandbox(context.Background(), "sb-1")
	if err == nil {
		t.Fatal("expected error")
	}

	apiErr, ok := err.(*core.APIError)
	if !ok {
		t.Fatalf("error type = %T", err)
	}
	if apiErr.Kind != core.APIErrorKindRateLimit {
		t.Fatalf("kind = %q", apiErr.Kind)
	}
	if !apiErr.Retryable() {
		t.Fatal("expected retryable error")
	}
}

func TestSystemEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/metrics":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("metric 1\n"))
		case "/shutdown":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"message":"shutdown initiated"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service, err := control.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	metrics, err := service.Metrics(context.Background())
	if err != nil || metrics != "metric 1\n" {
		t.Fatalf("Metrics = %q, err=%v", metrics, err)
	}

	shutdown, err := service.Shutdown(context.Background())
	if err != nil || shutdown.Message != "shutdown initiated" {
		t.Fatalf("Shutdown = %#v, err=%v", shutdown, err)
	}
}

func TestSandboxLifecyclePaths(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.URL.Path == "/api/v1/sandboxes/sb-1":
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			_, _ = w.Write([]byte(`{"sandboxID":"sb-1"}`))
		case strings.HasSuffix(r.URL.Path, "/logs"):
			_, _ = w.Write([]byte(`{"logs":[]}`))
		case strings.HasSuffix(r.URL.Path, "/pause"),
			strings.HasSuffix(r.URL.Path, "/timeout"),
			strings.HasSuffix(r.URL.Path, "/refreshes"):
			w.WriteHeader(http.StatusNoContent)
		case strings.HasSuffix(r.URL.Path, "/connect"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"sandboxID":"sb-1"}`))
		case strings.HasSuffix(r.URL.Path, "/heartbeat"):
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"received":true,"status":"healthy"},"request_id":"req-1"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service, err := control.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if _, err := service.GetSandbox(context.Background(), "sb-1"); err != nil {
		t.Fatalf("GetSandbox: %v", err)
	}
	zero := int64(0)
	ten := 10
	if _, err := service.GetSandboxLogs(context.Background(), "sb-1", &control.SandboxLogsParams{
		Cursor:    &zero,
		Limit:     &ten,
		Direction: "forward",
		Level:     "info",
		Search:    "health",
	}); err != nil {
		t.Fatalf("GetSandboxLogs: %v", err)
	}
	if err := service.PauseSandbox(context.Background(), "sb-1"); err != nil {
		t.Fatalf("PauseSandbox: %v", err)
	}
	if _, err := service.ConnectSandbox(context.Background(), "sb-1", &control.ConnectSandboxRequest{Timeout: 1200}); err != nil {
		t.Fatalf("ConnectSandbox: %v", err)
	}
	if err := service.SetSandboxTimeout(context.Background(), "sb-1", &control.TimeoutRequest{Timeout: 1200}); err != nil {
		t.Fatalf("SetSandboxTimeout: %v", err)
	}
	refresh := int32(60)
	if err := service.RefreshSandbox(context.Background(), "sb-1", &control.RefreshSandboxRequest{Duration: &refresh}); err != nil {
		t.Fatalf("RefreshSandbox: %v", err)
	}
	if err := service.RefreshSandbox(context.Background(), "sb-1", nil); err != nil {
		t.Fatalf("RefreshSandbox nil: %v", err)
	}
	if _, err := service.SendHeartbeat(context.Background(), "sb-1", &control.HeartbeatRequest{Status: "healthy"}); err != nil {
		t.Fatalf("SendHeartbeat: %v", err)
	}
	if err := service.DeleteSandbox(context.Background(), "sb-1"); err != nil {
		t.Fatalf("DeleteSandbox: %v", err)
	}

	if len(calls) != 9 {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestBoundSandboxHelpersUseStoredClient(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/sandboxes":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"sandboxID":"sb-1",
				"clientID":"user-1",
				"envdVersion":"atlas-0.1.0",
				"envdAccessToken":"unit-runtime-auth",
				"envdUrl":"https://sandbox-gateway.cloud.seaart.ai",
				"status":"running",
				"startedAt":"2024-01-01T00:00:00Z",
				"endAt":"2024-01-01T01:00:00Z"
			}`))
		case strings.HasSuffix(r.URL.Path, "/logs"):
			_, _ = w.Write([]byte(`{"logs":[]}`))
		default:
			_, _ = w.Write([]byte(`{
				"sandboxID":"sb-1",
				"clientID":"user-1",
				"envdVersion":"atlas-0.1.0",
				"envdAccessToken":"unit-runtime-auth",
				"envdUrl":"https://sandbox-gateway.cloud.seaart.ai",
				"status":"running",
				"startedAt":"2024-01-01T00:00:00Z",
				"endAt":"2024-01-01T01:00:00Z"
			}`))
		}
	}))
	defer server.Close()

	client, err := sandbox.NewClient(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	created, err := client.CreateSandbox(context.Background(), &control.NewSandboxRequest{TemplateID: "base"})
	if err != nil {
		t.Fatalf("CreateSandbox: %v", err)
	}

	detail, err := created.Reload(context.Background())
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
	if detail.SandboxID != "sb-1" {
		t.Fatalf("detail sandboxID = %q", detail.SandboxID)
	}

	if _, err := created.Logs(context.Background(), nil); err != nil {
		t.Fatalf("Logs: %v", err)
	}
	if len(calls) != 3 {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestValidationThroughPublicAPIs(t *testing.T) {
	service, err := control.NewService("https://sandbox-gateway.cloud.seaart.ai", "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	if _, err := service.ConnectSandbox(context.Background(), "sb", &control.ConnectSandboxRequest{Timeout: -1}); err == nil {
		t.Fatal("expected timeout validation error")
	}

	tooLong := int32(3601)
	if err := service.RefreshSandbox(context.Background(), "sb", &control.RefreshSandboxRequest{Duration: &tooLong}); err == nil {
		t.Fatal("expected refresh validation error")
	}

	if _, err := service.SendHeartbeat(context.Background(), "sb", &control.HeartbeatRequest{Status: "bad"}); err == nil {
		t.Fatal("expected heartbeat validation error")
	}

	negativeCursor := int64(-1)
	if _, err := service.GetSandboxLogs(context.Background(), "sb", &control.SandboxLogsParams{Cursor: &negativeCursor}); err == nil {
		t.Fatal("expected cursor validation error")
	}

	if _, err := service.GetSandboxLogs(context.Background(), "sb", &control.SandboxLogsParams{Direction: "sideways"}); err == nil {
		t.Fatal("expected direction validation error")
	}

	longSearch := strings.Repeat("x", 257)
	if _, err := service.GetSandboxLogs(context.Background(), "sb", &control.SandboxLogsParams{Search: longSearch}); err == nil {
		t.Fatal("expected search validation error")
	}
}
