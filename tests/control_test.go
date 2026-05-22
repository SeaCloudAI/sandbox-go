package tests

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
		if got := r.Header.Get("X-Project-ID"); got != "project-1" {
			t.Fatalf("project header = %q", got)
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
			"envdAccessToken":"unit-runtime-auth",
			"envdUrl":"https://sandbox-gateway.cloud.seaart.ai",
			"status":"starting",
			"state":"starting",
			"startedAt":"2024-01-01T00:00:00Z",
			"activatedAt":"2024-01-01T00:00:05Z",
			"endAt":"2024-01-01T01:00:00Z"
		}`))
	}))
	defer server.Close()

	client := newSDKClient(t, server.URL, core.WithProjectID("project-1"))

	resp, err := client.CreateSandbox(context.Background(), &control.NewSandboxRequest{TemplateID: "base"})
	if err != nil {
		t.Fatalf("CreateSandbox: %v", err)
	}
	if resp.SandboxID != "sb-123" {
		t.Fatalf("sandboxID = %q", resp.SandboxID)
	}
	if resp.ActivatedAt == nil || resp.ActivatedAt.Format(time.RFC3339) != "2024-01-01T00:00:05Z" {
		t.Fatalf("activatedAt = %#v", resp.ActivatedAt)
	}
	runtime, err := resp.Runtime()
	if err != nil {
		t.Fatalf("Runtime: %v", err)
	}
	if got := runtime.BaseURL(); got != "https://sandbox-gateway.cloud.seaart.ai" {
		t.Fatalf("runtime baseURL = %q", got)
	}
}

func TestCreateSandboxRequiresTemplateID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req["templateID"] != "base" {
			t.Fatalf("templateID = %#v", req["templateID"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"sandboxID":"sb-234","clientID":"user-1","status":"starting","startedAt":"2024-01-01T00:00:00Z","endAt":"2024-01-01T01:00:00Z"}`))
	}))
	defer server.Close()

	service, err := control.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	if _, err := service.CreateSandbox(context.Background(), &control.NewSandboxRequest{WaitReady: boolPtr(true)}); err == nil {
		t.Fatal("expected missing templateID to be rejected")
	}

	resp, err := service.CreateSandbox(context.Background(), &control.NewSandboxRequest{TemplateID: "base", WaitReady: boolPtr(true)})
	if err != nil {
		t.Fatalf("CreateSandbox: %v", err)
	}
	if resp.SandboxID != "sb-234" {
		t.Fatalf("sandboxID = %q", resp.SandboxID)
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

func TestGatewayDiagnosticsIncludeRequestIDAndRedactSensitiveQuery(t *testing.T) {
	var events []core.DiagnosticEvent
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Request-ID"); strings.TrimSpace(got) == "" {
			t.Fatal("missing X-Request-ID")
		}
		if got := r.URL.Query().Get("nextToken"); got != "secret-page" {
			t.Fatalf("nextToken = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	service, err := control.NewService(server.URL, "unit-auth-value", core.WithLogger(func(event core.DiagnosticEvent) {
		events = append(events, event)
	}))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = service.ListSandboxes(context.Background(), &control.ListSandboxesParams{NextToken: "secret-page"})
	if err != nil {
		t.Fatalf("ListSandboxes: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %#v", events)
	}
	if events[0].Type != "request" || events[0].Path != "/api/v1/sandboxes?nextToken=%3Credacted%3E" {
		t.Fatalf("request event = %#v", events[0])
	}
	if events[0].RequestID == "" || events[1].RequestID != events[0].RequestID {
		t.Fatalf("request ids = %#v", events)
	}
	if strings.Contains(events[0].Path, "secret-page") {
		t.Fatalf("path leaked token: %s", events[0].Path)
	}
}

func TestGatewayDiagnosticsIncludeAPIErrorDetails(t *testing.T) {
	var events []core.DiagnosticEvent
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"code":429,"message":"quota exceeded","request_id":"server-req"}`))
	}))
	defer server.Close()

	service, err := control.NewService(server.URL, "unit-auth-value", core.WithLogger(func(event core.DiagnosticEvent) {
		events = append(events, event)
	}))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = service.ListSandboxes(context.Background(), nil)
	if err == nil {
		t.Fatal("expected API error")
	}
	last := events[len(events)-1]
	if last.Type != "error" || last.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("error event = %#v", last)
	}
	if last.RequestID != "server-req" || last.ErrorKind != core.APIErrorKindRateLimit || !last.Retryable {
		t.Fatalf("error detail = %#v", last)
	}
}

func TestSandboxMetricsEndpoints(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.String())
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/sandboxes/sb-1/metrics":
			_, _ = w.Write([]byte(`{
				"sandboxID":"sb-1",
				"collectedAt":"2026-05-20T00:00:00Z",
				"cpuCount":2,
				"cpuUsedPct":12.5,
				"load1":0.4,
				"cpuUserRate":0.2,
				"memTotal":2147483648,
				"memUsed":1073741824,
				"memTotalMiB":2048,
				"memUsedMiB":1024,
				"memCache":128,
				"memoryUsagePercent":50,
				"diskUsed":1024,
				"diskTotal":2048,
				"diskReadBytesPerSecond":4096,
				"netRxBytes":10,
				"netTxBytes":20,
				"networkRecvBytesPerSecond":100,
				"taskCurrent":3
			}`))
		case "/api/v1/sandboxes/metrics":
			if got := r.URL.Query().Get("sandbox_ids"); got != "sb-1,sb-2" {
				t.Fatalf("sandbox_ids = %q", got)
			}
			if got := r.URL.Query().Get("limit"); got != "2" {
				t.Fatalf("limit = %q", got)
			}
			_, _ = w.Write([]byte(`{
				"collectedAt":"2026-05-20T00:00:00Z",
				"items":[{
					"sandboxID":"sb-1",
					"collectedAt":"2026-05-20T00:00:00Z",
					"cpuCount":2,
					"cpuUsedPct":12.5,
					"memTotal":1,
					"memUsed":1,
					"memTotalMiB":1,
					"memUsedMiB":1,
					"memCache":0,
					"diskUsed":1,
					"diskTotal":1,
					"netRxBytes":1,
					"netTxBytes":1
				}],
				"sandboxes":{}
			}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	service, err := control.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	single, err := service.GetSandboxMetrics(context.Background(), "sb-1")
	if err != nil {
		t.Fatalf("GetSandboxMetrics: %v", err)
	}
	if single.Load1 == nil || *single.Load1 != 0.4 {
		t.Fatalf("load1 = %#v", single.Load1)
	}
	if single.MemoryUsagePercent == nil || *single.MemoryUsagePercent != 50 {
		t.Fatalf("memoryUsagePercent = %#v", single.MemoryUsagePercent)
	}
	if single.DiskReadBytesPerSecond == nil || *single.DiskReadBytesPerSecond != 4096 {
		t.Fatalf("diskReadBytesPerSecond = %#v", single.DiskReadBytesPerSecond)
	}
	if single.NetworkRecvBytesPerSecond == nil || *single.NetworkRecvBytesPerSecond != 100 {
		t.Fatalf("networkRecvBytesPerSecond = %#v", single.NetworkRecvBytesPerSecond)
	}
	if single.TaskCurrent == nil || *single.TaskCurrent != 3 {
		t.Fatalf("taskCurrent = %#v", single.TaskCurrent)
	}

	batch, err := service.ListSandboxMetrics(context.Background(), &control.SandboxMetricsParams{
		SandboxIDs: []string{"sb-1", " ", "sb-2"},
		Limit:      2,
	})
	if err != nil {
		t.Fatalf("ListSandboxMetrics: %v", err)
	}
	if len(batch.Items) != 1 || batch.Items[0].SandboxID != "sb-1" {
		t.Fatalf("batch items = %#v", batch.Items)
	}
	if len(calls) != 2 {
		t.Fatalf("calls = %#v", calls)
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
				"status":"running"
			}]`))
		case strings.HasSuffix(r.URL.Path, "/logs"):
			_, _ = w.Write([]byte(`{"logs":[]}`))
		default:
			_, _ = w.Write([]byte(`{
				"sandboxID":"sb-1",
				"clientID":"user-1",
				"envdAccessToken":"unit-runtime-auth",
				"envdUrl":"https://sandbox-gateway.cloud.seaart.ai",
				"status":"running",
				"startedAt":"2024-01-01T00:00:00Z",
				"endAt":"2024-01-01T01:00:00Z"
			}`))
		}
	}))
	defer server.Close()

	client := newSDKClient(t, server.URL)

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

func TestAdminControlEndpoints(t *testing.T) {
	var calls []struct {
		path   string
		method string
		body   map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if r.Body != nil && r.ContentLength != 0 {
			_ = json.NewDecoder(r.Body).Decode(&body)
		}
		calls = append(calls, struct {
			path   string
			method string
			body   map[string]any
		}{path: r.URL.Path, method: r.Method, body: body})
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/admin/pool/status":
			_, _ = w.Write([]byte(`{"code":0,"data":{"total":10,"warm":2,"active":3,"creating":1,"stopped":1,"deleting":1,"deleted":2,"utilization":0.5},"request_id":"req-pool"}`))
		case "/admin/rolling/start":
			_, _ = w.Write([]byte(`{"code":0,"data":{"phase":"running","progress":0.25,"warm_total":4,"warm_updated":1,"duration":"10s"},"request_id":"req-start"}`))
		case "/admin/rolling/status":
			_, _ = w.Write([]byte(`{"code":0,"data":{"phase":"running","progress":0.5,"warm_total":4,"warm_updated":2,"duration":"20s"},"request_id":"req-status"}`))
		case "/admin/rolling/cancel":
			_, _ = w.Write([]byte(`{"code":0,"data":{"phase":"cancelled","progress":0.5,"warm_total":4,"warm_updated":2,"duration":"21s"},"request_id":"req-cancel"}`))
		default:
			t.Fatalf("path = %s", r.URL.Path)
		}
	}))
	defer server.Close()

	service, err := control.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	pool, err := service.GetPoolStatus(context.Background())
	if err != nil || pool.RequestID != "req-pool" {
		t.Fatalf("GetPoolStatus = %#v, %v", pool, err)
	}
	started, err := service.StartRollingUpdate(context.Background(), &control.RollingStartRequest{TemplateID: "tpl-1"})
	if err != nil || started.RequestID != "req-start" {
		t.Fatalf("StartRollingUpdate = %#v, %v", started, err)
	}
	status, err := service.GetRollingUpdateStatus(context.Background())
	if err != nil || status.RequestID != "req-status" {
		t.Fatalf("GetRollingUpdateStatus = %#v, %v", status, err)
	}
	cancelled, err := service.CancelRollingUpdate(context.Background())
	if err != nil || cancelled.RequestID != "req-cancel" {
		t.Fatalf("CancelRollingUpdate = %#v, %v", cancelled, err)
	}
	if calls[1].path != "/admin/rolling/start" || calls[1].body["templateId"] != "tpl-1" {
		t.Fatalf("start call = %#v", calls[1])
	}
	if _, err := service.StartRollingUpdate(context.Background(), &control.RollingStartRequest{TemplateID: " "}); err == nil {
		t.Fatal("expected templateId validation error")
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
	type lifecycleCall struct {
		Method string
		Path   string
		Body   string
	}
	var calls []lifecycleCall
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		calls = append(calls, lifecycleCall{
			Method: r.Method,
			Path:   r.URL.String(),
			Body:   string(body),
		})
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
	var pauseCall *lifecycleCall
	var refreshNilCall *lifecycleCall
	for i := range calls {
		call := &calls[i]
		if call.Path == "/api/v1/sandboxes/sb-1/pause" {
			pauseCall = call
		}
		if call.Path == "/api/v1/sandboxes/sb-1/refreshes" && call.Body == "" {
			refreshNilCall = call
		}
	}
	if pauseCall == nil || pauseCall.Body != "" {
		t.Fatalf("pause call = %#v", pauseCall)
	}
	if refreshNilCall == nil {
		t.Fatalf("refresh nil call not found: %#v", calls)
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
				"envdAccessToken":"unit-runtime-auth",
				"envdUrl":"https://sandbox-gateway.cloud.seaart.ai",
				"status":"running",
				"startedAt":"2024-01-01T00:00:00Z",
				"endAt":"2024-01-01T01:00:00Z"
			}`))
		case strings.HasSuffix(r.URL.Path, "/logs"):
			_, _ = w.Write([]byte(`{"logs":[]}`))
		case strings.HasSuffix(r.URL.Path, "/metrics"):
			_, _ = w.Write([]byte(`{
				"sandboxID":"sb-1",
				"collectedAt":"2026-05-20T00:00:00Z",
				"cpuCount":1,
				"cpuUsedPct":1,
				"memTotal":1,
				"memUsed":1,
				"memTotalMiB":1,
				"memUsedMiB":1,
				"memCache":0,
				"diskUsed":1,
				"diskTotal":1,
				"netRxBytes":1,
				"netTxBytes":1
			}`))
		default:
			_, _ = w.Write([]byte(`{
				"sandboxID":"sb-1",
				"clientID":"user-1",
				"envdAccessToken":"unit-runtime-auth",
				"envdUrl":"https://sandbox-gateway.cloud.seaart.ai",
				"status":"running",
				"startedAt":"2024-01-01T00:00:00Z",
				"endAt":"2024-01-01T01:00:00Z"
			}`))
		}
	}))
	defer server.Close()

	client := newSDKClient(t, server.URL)

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
	metrics, err := created.Metrics(context.Background())
	if err != nil {
		t.Fatalf("Metrics: %v", err)
	}
	if metrics.SandboxID != "sb-1" {
		t.Fatalf("metrics sandboxID = %q", metrics.SandboxID)
	}
	if len(calls) != 4 {
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

func TestBoundaryValuesThroughPublicAPIs(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("ReadAll: %v", err)
		}
		calls = append(calls, r.Method+" "+r.URL.String()+" "+string(body))
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/connect"):
			_, _ = w.Write([]byte(`{"sandboxID":"sb"}`))
		case strings.HasSuffix(r.URL.Path, "/heartbeat"):
			_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"received":true,"status":"healthy"},"request_id":"req-boundary"}`))
		case strings.HasSuffix(r.URL.Path, "/logs"):
			_, _ = w.Write([]byte(`{"logs":[]}`))
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()

	service, err := control.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	zero := int64(0)
	limit := 1000
	if _, err := service.GetSandboxLogs(context.Background(), "sb", &control.SandboxLogsParams{
		Cursor:    &zero,
		Limit:     &limit,
		Direction: "backward",
		Search:    strings.Repeat("x", 256),
	}); err != nil {
		t.Fatalf("GetSandboxLogs boundary: %v", err)
	}
	if _, err := service.ConnectSandbox(context.Background(), "sb", &control.ConnectSandboxRequest{Timeout: 0}); err != nil {
		t.Fatalf("ConnectSandbox boundary: %v", err)
	}
	if err := service.SetSandboxTimeout(context.Background(), "sb", &control.TimeoutRequest{Timeout: 86_400}); err != nil {
		t.Fatalf("SetSandboxTimeout boundary: %v", err)
	}
	zeroRefresh := int32(0)
	if err := service.RefreshSandbox(context.Background(), "sb", &control.RefreshSandboxRequest{Duration: &zeroRefresh}); err != nil {
		t.Fatalf("RefreshSandbox zero boundary: %v", err)
	}
	maxRefresh := int32(3600)
	if err := service.RefreshSandbox(context.Background(), "sb", &control.RefreshSandboxRequest{Duration: &maxRefresh}); err != nil {
		t.Fatalf("RefreshSandbox max boundary: %v", err)
	}
	if _, err := service.SendHeartbeat(context.Background(), "sb", &control.HeartbeatRequest{Status: "healthy"}); err != nil {
		t.Fatalf("SendHeartbeat boundary: %v", err)
	}
	if len(calls) != 6 {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestEmptySandboxIDValidation(t *testing.T) {
	service, err := control.NewService("https://sandbox-gateway.cloud.seaart.ai", "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if _, err := service.GetSandbox(context.Background(), " "); err == nil {
		t.Fatal("expected sandbox id validation error")
	}
	if err := service.PauseSandbox(context.Background(), " "); err == nil {
		t.Fatal("expected sandbox id validation error")
	}
	if _, err := service.ConnectSandbox(context.Background(), " ", &control.ConnectSandboxRequest{Timeout: 1}); err == nil {
		t.Fatal("expected sandbox id validation error")
	}
	if err := service.SetSandboxTimeout(context.Background(), " ", &control.TimeoutRequest{Timeout: 1}); err == nil {
		t.Fatal("expected sandbox id validation error")
	}
	if err := service.RefreshSandbox(context.Background(), " ", &control.RefreshSandboxRequest{}); err == nil {
		t.Fatal("expected sandbox id validation error")
	}
	if _, err := service.SendHeartbeat(context.Background(), " ", &control.HeartbeatRequest{Status: "healthy"}); err == nil {
		t.Fatal("expected sandbox id validation error")
	}
}
