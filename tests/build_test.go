package tests

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SeaCloudAI/sandbox-go/build"
	"github.com/SeaCloudAI/sandbox-go/core"
)

func TestDirectBuild(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/build" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Namespace-ID"); got != "" {
			t.Fatalf("unexpected namespace header = %q", got)
		}

		var req build.DirectBuildRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Project != "proj" || req.Image != "app" || req.Tag != "v1" {
			t.Fatalf("request = %#v", req)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{
			"templateID":"tpl-1",
			"buildID":"build-1",
			"imageFullName":"example-image:v1"
		}`))
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.DirectBuild(context.Background(), &build.DirectBuildRequest{
		Project:    "proj",
		Image:      "app",
		Tag:        "v1",
		Dockerfile: "FROM alpine:3.20",
	})
	if err != nil {
		t.Fatalf("DirectBuild: %v", err)
	}
	if resp.TemplateID != "tpl-1" || resp.BuildID != "build-1" {
		t.Fatalf("response = %#v", resp)
	}
}

func TestCreateTemplateUsesGatewayAuthOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req build.TemplateCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if got := r.Header.Get("X-Namespace-ID"); got != "" {
			t.Fatalf("unexpected namespace = %q", got)
		}
		if got := r.Header.Get("X-User-ID"); got != "" {
			t.Fatalf("unexpected user = %q", got)
		}
		if got := r.Header.Get("X-Role"); got != "" {
			t.Fatalf("unexpected role = %q", got)
		}
		if got := r.Header.Get("X-User-Email"); got != "" {
			t.Fatalf("unexpected email = %q", got)
		}
		if req.Name != "demo" || req.Visibility != "personal" || req.Image != "docker.io/library/alpine:3.20" {
			t.Fatalf("request = %#v", req)
		}
		if req.CPUCount == nil || *req.CPUCount != 2 {
			t.Fatalf("cpuCount = %#v", req.CPUCount)
		}
		if req.Envs["APP_ENV"] != "test" {
			t.Fatalf("envs = %#v", req.Envs)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{
			"templateID":"tpl-1",
			"buildID":"build-1",
			"public":false,
			"names":["user-1/demo"],
			"tags":["v1"],
			"aliases":["demo"]
		}`))
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.CreateTemplate(context.Background(), &build.TemplateCreateRequest{
		Name:       "demo",
		Visibility: "personal",
		Image:      "docker.io/library/alpine:3.20",
		CPUCount:   int32Ptr(2),
		Envs:       map[string]string{"APP_ENV": "test"},
	})
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}
	if len(resp.Names) != 1 || resp.Names[0] != "user-1/demo" {
		t.Fatalf("names = %#v", resp.Names)
	}
}

func TestGetTemplateDecodesFullResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "10" || r.URL.Query().Get("nextToken") != "build-1" {
			t.Fatalf("query = %#v", r.URL.Query())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"templateID":"tpl-1",
			"buildID":"build-2",
			"buildStatus":"ready",
			"public":false,
			"names":["user-1/demo"],
			"aliases":["demo"],
			"tags":["v1"],
			"name":"demo",
			"visibility":"personal",
			"image":"example-image:v1",
			"imageSource":"dockerfile",
			"envdVersion":"sandbox-builder-v1",
			"cpuCount":2,
			"memoryMB":1024,
			"diskSizeMB":5120,
			"createdBy":{"id":"user-1","email":"test-user"},
			"createdByID":"user-1",
			"projectID":"proj-1",
			"createdAt":"2026-01-01T00:00:00Z",
			"updatedAt":"2026-01-01T00:01:00Z",
			"lastSpawnedAt":"2026-01-01T00:02:00Z",
			"spawnCount":3,
			"buildCount":4,
			"storageType":"ephemeral",
			"ttlSeconds":300,
			"port":9000,
			"startCmd":"npm start",
			"readyCmd":"test-ready-command",
			"builds":[
				{
					"buildID":"build-2",
					"templateID":"tpl-1",
					"status":"ready",
					"image":"example-image:v1",
					"errorMessage":"",
					"createdAt":"2026-01-01T00:00:00Z",
					"updatedAt":"2026-01-01T00:02:00Z",
					"finishedAt":"2026-01-01T00:02:00Z"
				}
			],
			"nextToken":"build-next"
		}`))
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.GetTemplate(context.Background(), "tpl-1", &build.GetTemplateParams{
		Limit:     10,
		NextToken: "build-1",
	})
	if err != nil {
		t.Fatalf("GetTemplate: %v", err)
	}
	if resp.TemplateID != "tpl-1" || resp.BuildID != "build-2" || resp.ImageSource != "dockerfile" {
		t.Fatalf("response = %#v", resp)
	}
	if resp.CreatedBy == nil || resp.CreatedBy.Email != "test-user" {
		t.Fatalf("createdBy = %#v", resp.CreatedBy)
	}
	if len(resp.Builds) != 1 || resp.Builds[0].Status != "ready" {
		t.Fatalf("builds = %#v", resp.Builds)
	}
	if resp.NextToken != "build-next" || resp.StartCmd != "npm start" || resp.Port != 9000 {
		t.Fatalf("unexpected fields = %#v", resp)
	}
}

func TestListTemplatesEncodesQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if got := q.Get("visibility"); got != "team" {
			t.Fatalf("visibility = %q", got)
		}
		if got := q.Get("teamID"); got != "ns-1" {
			t.Fatalf("teamID = %q", got)
		}
		if got := q.Get("limit"); got != "20" {
			t.Fatalf("limit = %q", got)
		}
		if got := q.Get("offset"); got != "40" {
			t.Fatalf("offset = %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{
			"templateID":"tpl-1",
			"buildID":"build-1",
			"buildStatus":"ready",
			"public":false,
			"names":["user-1/demo"],
			"aliases":["demo"],
			"createdBy":{"id":"user-1"}
		}]`))
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.ListTemplates(context.Background(), &build.ListTemplatesParams{
		Visibility: "team",
		TeamID:     "ns-1",
		Limit:      20,
		Offset:     40,
	})
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(resp) != 1 || resp[0].TemplateID != "tpl-1" {
		t.Fatalf("response = %#v", resp)
	}
}

func TestCreateBuildCompatResponseIsMarkedEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/templates/tpl-1/builds" {
			t.Fatalf("path = %s", r.URL.Path)
		}

		var req build.BuildRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.BuildID != "build-abc" {
			t.Fatalf("buildID = %q", req.BuildID)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.CreateBuild(context.Background(), "tpl-1", &build.BuildRequest{
		BuildID:      "build-abc",
		FromTemplate: "base",
	})
	if err != nil {
		t.Fatalf("CreateBuild: %v", err)
	}
	if !resp.Empty {
		t.Fatalf("expected empty compat response, got %#v", resp)
	}
}

func TestCreateBuildUsesNativeResponseAndEmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if len(body) != 0 {
			t.Fatalf("expected empty body, got %q", string(body))
		}
		if got := r.Header.Get("Content-Type"); got != "" {
			t.Fatalf("unexpected content-type = %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{
			"buildID":"build-1",
			"templateID":"tpl-1",
			"status":"uploaded",
			"image":"example-image:v1",
			"errorMessage":"",
			"createdAt":"2026-01-01T00:00:00Z",
			"updatedAt":"2026-01-01T00:00:01Z",
			"finishedAt":"2026-01-01T00:00:02Z"
		}`))
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.CreateBuild(context.Background(), "tpl-1", nil)
	if err != nil {
		t.Fatalf("CreateBuild: %v", err)
	}
	if resp.Empty || resp.BuildID != "build-1" || resp.Status != "uploaded" {
		t.Fatalf("response = %#v", resp)
	}
}

func TestCreateBuildEncodesSupportedFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req build.BuildRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.FromImage != "docker.io/library/node:20" || req.FilesHash != strings.Repeat("a", 64) {
			t.Fatalf("request = %#v", req)
		}
		if req.StartCmd != "npm start" || req.ReadyCmd != "test-ready-command" {
			t.Fatalf("commands = %#v", req)
		}
		if len(req.Steps) != 1 || req.Steps[0].Type != "files" || req.Steps[0].FilesHash != strings.Repeat("a", 64) {
			t.Fatalf("steps = %#v", req.Steps)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.CreateBuild(context.Background(), "tpl-1", &build.BuildRequest{
		FromImage: "docker.io/library/node:20",
		FilesHash: strings.Repeat("a", 64),
		Steps: []build.BuildStep{
			{Type: "files", FilesHash: strings.Repeat("a", 64)},
		},
		StartCmd: "npm start",
		ReadyCmd: "test-ready-command",
	})
	if err != nil {
		t.Fatalf("CreateBuild: %v", err)
	}
	if !resp.Empty {
		t.Fatalf("expected compat empty response, got %#v", resp)
	}
}

func TestCreateBuildRejectsUnsupportedFields(t *testing.T) {
	service, err := build.NewService("https://sandbox-gateway.cloud.seaart.ai", "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = service.CreateBuild(context.Background(), "tpl-1", &build.BuildRequest{
		FromImageRegistry: "docker.io/library/node:20",
	})
	if err == nil || !strings.Contains(err.Error(), "fromImageRegistry") {
		t.Fatalf("CreateBuild error = %v", err)
	}
}

func TestBuildValidationErrors(t *testing.T) {
	service, err := build.NewService("https://sandbox-gateway.cloud.seaart.ai", "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	testCases := []struct {
		name string
		fn   func() error
		want string
	}{
		{
			name: "invalid build id",
			fn: func() error {
				_, err := service.CreateBuild(context.Background(), "tpl-1", &build.BuildRequest{
					BuildID: "Build-Uppercase",
				})
				return err
			},
			want: "buildID must be",
		},
		{
			name: "invalid files hash",
			fn: func() error {
				_, err := service.CreateBuild(context.Background(), "tpl-1", &build.BuildRequest{
					FilesHash: "abc",
				})
				return err
			},
			want: "filesHash must be",
		},
		{
			name: "missing step type",
			fn: func() error {
				_, err := service.CreateBuild(context.Background(), "tpl-1", &build.BuildRequest{
					Steps: []build.BuildStep{{FilesHash: strings.Repeat("a", 64)}},
				})
				return err
			},
			want: "steps[0].type is required",
		},
		{
			name: "step args unsupported",
			fn: func() error {
				_, err := service.CreateBuild(context.Background(), "tpl-1", &build.BuildRequest{
					Steps: []build.BuildStep{{Type: "files", FilesHash: strings.Repeat("a", 64), Args: []string{"x"}}},
				})
				return err
			},
			want: "steps[0].args is not supported",
		},
		{
			name: "multiple hashes unsupported",
			fn: func() error {
				_, err := service.CreateBuild(context.Background(), "tpl-1", &build.BuildRequest{
					FilesHash: strings.Repeat("a", 64),
					Steps:     []build.BuildStep{{Type: "files", FilesHash: strings.Repeat("b", 64)}},
				})
				return err
			},
			want: "multiple different filesHash",
		},
		{
			name: "invalid status limit",
			fn: func() error {
				_, err := service.GetBuildStatus(context.Background(), "tpl-1", "build-1", &build.BuildStatusParams{Limit: intPtr(101)})
				return err
			},
			want: "build status limit",
		},
		{
			name: "invalid logs direction",
			fn: func() error {
				_, err := service.GetBuildLogs(context.Background(), "tpl-1", "build-1", &build.BuildLogsParams{Direction: "sideways"})
				return err
			},
			want: "build logs direction",
		},
		{
			name: "invalid template list offset",
			fn: func() error {
				_, err := service.ListTemplates(context.Background(), &build.ListTemplatesParams{Offset: -1})
				return err
			},
			want: "template list offset",
		},
		{
			name: "invalid template detail limit",
			fn: func() error {
				_, err := service.GetTemplate(context.Background(), "tpl-1", &build.GetTemplateParams{Limit: 101})
				return err
			},
			want: "template build history limit",
		},
		{
			name: "empty alias",
			fn: func() error {
				_, err := service.GetTemplateByAlias(context.Background(), " ")
				return err
			},
			want: "alias is required",
		},
		{
			name: "invalid hash query",
			fn: func() error {
				_, err := service.GetBuildFile(context.Background(), "tpl-1", "bad")
				return err
			},
			want: "hash must be",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fn()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestGetBuildStatusAllowsAnonymousPollingAndLegacyLogsShape(t *testing.T) {
	logsOffset := 5
	limit := 10

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Namespace-ID"); got != "" {
			t.Fatalf("unexpected namespace header = %q", got)
		}
		q := r.URL.Query()
		if got := q.Get("logsOffset"); got != "5" {
			t.Fatalf("logsOffset = %q", got)
		}
		if got := q.Get("limit"); got != "10" {
			t.Fatalf("limit = %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"buildID":"build-1",
			"templateID":"tpl-1",
			"status":"building",
			"logs":[
				{
					"timestamp":"2026-01-01T00:00:00Z",
					"level":"info",
					"step":"build",
					"message":"building image"
				}
			],
			"reason":null,
			"createdAt":"2026-01-01T00:00:00Z",
			"updatedAt":"2026-01-01T00:00:01Z"
		}`))
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.GetBuildStatus(context.Background(), "tpl-1", "build-1", &build.BuildStatusParams{
		LogsOffset: &logsOffset,
		Limit:      &limit,
	})
	if err != nil {
		t.Fatalf("GetBuildStatus: %v", err)
	}
	if len(resp.LogEntries) != 1 || resp.LogEntries[0].Message != "building image" {
		t.Fatalf("logEntries = %#v", resp.LogEntries)
	}
}

func TestGetBuildStatusPrefersExplicitLogEntries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"buildID":"build-1",
			"templateID":"tpl-1",
			"status":"building",
			"logs":["raw-line"],
			"logEntries":[
				{
					"timestamp":"2026-01-01T00:00:00Z",
					"level":"info",
					"step":"build",
					"message":"structured log"
				}
			],
			"reason":"queued",
			"createdAt":"2026-01-01T00:00:00Z",
			"updatedAt":"2026-01-01T00:00:01Z"
		}`))
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	resp, err := service.GetBuildStatus(context.Background(), "tpl-1", "build-1", nil)
	if err != nil {
		t.Fatalf("GetBuildStatus: %v", err)
	}
	if len(resp.LogEntries) != 1 || resp.LogEntries[0].Message != "structured log" || len(resp.Logs) != 1 {
		t.Fatalf("response = %#v", resp)
	}
}

func TestBuildListGetAndLogsEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/templates/tpl-1/builds":
			_, _ = w.Write([]byte(`{
				"builds":[
					{
						"buildID":"build-1",
						"templateID":"tpl-1",
						"status":"ready",
						"image":"example-image:v1",
						"errorMessage":"",
						"createdAt":"2026-01-01T00:00:00Z",
						"updatedAt":"2026-01-01T00:02:00Z",
						"finishedAt":"2026-01-01T00:02:00Z"
					}
				],
				"total":1
			}`))
		case r.URL.Path == "/api/v1/templates/tpl-1/builds/build-1":
			_, _ = w.Write([]byte(`{
				"buildID":"build-1",
				"templateID":"tpl-1",
				"status":"ready",
				"image":"example-image:v1",
				"errorMessage":"",
				"createdAt":"2026-01-01T00:00:00Z",
				"updatedAt":"2026-01-01T00:02:00Z",
				"finishedAt":"2026-01-01T00:02:00Z"
			}`))
		case r.URL.Path == "/api/v1/templates/tpl-1/builds/build-1/logs":
			q := r.URL.Query()
			if q.Get("cursor") != "0" || q.Get("limit") != "10" || q.Get("direction") != "forward" || q.Get("level") != "info" || q.Get("source") != "persistent" {
				t.Fatalf("query = %#v", q)
			}
			_, _ = w.Write([]byte(`{
				"logs":[
					{
						"timestamp":"2026-01-01T00:00:00Z",
						"level":"info",
						"step":"build",
						"message":"building image"
					}
				]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	history, err := service.ListBuilds(context.Background(), "tpl-1")
	if err != nil {
		t.Fatalf("ListBuilds: %v", err)
	}
	if history.Total != 1 || len(history.Builds) != 1 {
		t.Fatalf("history = %#v", history)
	}

	buildResp, err := service.GetBuild(context.Background(), "tpl-1", "build-1")
	if err != nil {
		t.Fatalf("GetBuild: %v", err)
	}
	if buildResp.Status != "ready" || buildResp.Image == "" {
		t.Fatalf("build = %#v", buildResp)
	}

	cursor := int64(0)
	limit := 10
	logs, err := service.GetBuildLogs(context.Background(), "tpl-1", "build-1", &build.BuildLogsParams{
		Cursor:    &cursor,
		Limit:     &limit,
		Direction: "forward",
		Level:     "info",
		Source:    "persistent",
	})
	if err != nil {
		t.Fatalf("GetBuildLogs: %v", err)
	}
	if len(logs.Logs) != 1 || logs.Logs[0].Message != "building image" {
		t.Fatalf("logs = %#v", logs)
	}
}

func TestDeleteTemplateAllowsAnonymousDirectBuildCleanup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("method = %s", r.Method)
		}
		if got := r.Header.Get("X-User-ID"); got != "" {
			t.Fatalf("unexpected user header = %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	if err := service.DeleteTemplate(context.Background(), "tpl-1"); err != nil {
		t.Fatalf("DeleteTemplate: %v", err)
	}
}

func TestBuildAPIErrorDecoding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{
			"code": 400,
			"message": "validation failed",
			"error": {
				"code": "INVALID_HASH",
				"details": "hash must be sha256"
			},
			"request_id": "req-build-1"
		}`))
	}))
	defer server.Close()

	service, err := build.NewService(server.URL, "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	_, err = service.GetBuildFile(context.Background(), "tpl-1", strings.Repeat("a", 64))
	if err == nil {
		t.Fatal("expected error")
	}

	var apiErr *core.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T", err)
	}
	if apiErr.RequestID != "req-build-1" {
		t.Fatalf("requestID = %q", apiErr.RequestID)
	}
}

func int32Ptr(v int32) *int32 {
	return &v
}
