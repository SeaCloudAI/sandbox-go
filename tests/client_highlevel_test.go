package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/SeaCloudAI/sandbox-go"
	"github.com/SeaCloudAI/sandbox-go/core"
)

func TestClientHighLevelHelpersReuseStoredConfig(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/sandboxes" && r.Method == http.MethodPost:
			if got := r.Header.Get("X-Project-ID"); got != "project-1" {
				t.Fatalf("project header = %q", got)
			}
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode sandbox request: %v", err)
			}
			if req["templateID"] != "base" {
				t.Fatalf("templateID = %#v", req["templateID"])
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"sandboxID":"sb-high",
				"clientID":"user-1",
				"envdVersion":"atlas-0.1.0",
				"envdAccessToken":"unit-runtime-auth",
				"envdUrl":"https://sandbox-gateway.cloud.seaart.ai",
				"status":"running",
				"startedAt":"2026-01-01T00:00:00Z",
				"endAt":"2026-01-01T01:00:00Z"
			}`))
		case r.URL.Path == "/api/v1/sandboxes/sb-high" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{
				"sandboxID":"sb-high",
				"clientID":"user-1",
				"envdVersion":"atlas-0.1.0",
				"envdAccessToken":"unit-runtime-auth",
				"envdUrl":"https://sandbox-gateway.cloud.seaart.ai",
				"status":"running",
				"startedAt":"2026-01-01T00:00:00Z",
				"endAt":"2026-01-01T01:00:00Z"
			}`))
		case r.URL.Path == "/api/v1/templates" && r.Method == http.MethodPost:
			if got := r.Header.Get("X-Project-ID"); got != "project-1" {
				t.Fatalf("project header = %q", got)
			}
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode template request: %v", err)
			}
			if req["name"] != "demo" {
				t.Fatalf("name = %#v", req["name"])
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"templateID":"tpl-1","buildID":"server-build-id","public":false,"names":["demo"],"tags":["v1"],"aliases":[]}`))
		case strings.Contains(r.URL.Path, "/builds/") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{}`))
		case strings.HasSuffix(r.URL.Path, "/status"):
			_, _ = w.Write([]byte(`{"buildID":"build-1","templateID":"tpl-1","status":"ready","logs":[],"logEntries":[]}`))
		case r.URL.Path == "/api/v1/templates/tpl-1" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"templateID":"tpl-1","buildStatus":"ready","public":false,"aliases":[],"names":["demo"],"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:01Z","spawnCount":0}`))
		case strings.Contains(r.URL.Path, "/builds/") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"buildID":"build-1","templateID":"tpl-1","status":"ready","image":"demo:v1","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:01Z"}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := newSDKClient(t, server.URL, core.WithProjectID("project-1"))

	waitReady := true
	created, err := client.Create(context.Background(), "base", &sandbox.CreateOptions{WaitReady: &waitReady})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	detail, err := created.GetInfo(context.Background())
	if err != nil {
		t.Fatalf("GetInfo: %v", err)
	}
	if created.SandboxID != "sb-high" || detail.SandboxID != "sb-high" {
		t.Fatalf("sandbox = %#v detail = %#v", created, detail)
	}

	template := sandbox.NewTemplate().FromBaseImage().RunCmd("echo hello", nil)
	built, err := client.BuildTemplate(context.Background(), template, "demo:v1", &sandbox.TemplateBuildOptions{})
	if err != nil {
		t.Fatalf("BuildTemplate: %v", err)
	}
	if built.TemplateID != "tpl-1" || built.Status != "ready" {
		t.Fatalf("built = %#v", built)
	}
}

func TestPackageLevelHelpersUseEnvFirstConfigAndTemplateFacade(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/sandboxes" && r.Method == http.MethodPost:
			if got := r.Header.Get("X-Project-ID"); got != "project-env" {
				t.Fatalf("project header = %q", got)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"sandboxID":"sb-env",
				"clientID":"user-1",
				"envdVersion":"atlas-0.1.0",
				"envdAccessToken":"unit-runtime-auth",
				"envdUrl":"https://sandbox-gateway.cloud.seaart.ai",
				"status":"running",
				"startedAt":"2026-01-01T00:00:00Z",
				"endAt":"2026-01-01T01:00:00Z"
			}`))
		case r.URL.Path == "/api/v1/templates" && r.Method == http.MethodPost:
			if got := r.Header.Get("X-Project-ID"); got != "project-env" {
				t.Fatalf("project header = %q", got)
			}
			var req map[string]any
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode template request: %v", err)
			}
			if req["name"] != "demo" {
				t.Fatalf("name = %#v", req["name"])
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"templateID":"tpl-env","buildID":"server-build-id","public":false,"names":["demo"],"tags":["v1"],"aliases":[]}`))
		case strings.Contains(r.URL.Path, "/builds/") && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{}`))
		case strings.HasSuffix(r.URL.Path, "/status"):
			_, _ = w.Write([]byte(`{"buildID":"build-1","templateID":"tpl-env","status":"ready","logs":[],"logEntries":[]}`))
		case r.URL.Path == "/api/v1/templates/tpl-env" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"templateID":"tpl-env","buildStatus":"ready","public":false,"aliases":[],"names":["demo"],"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:01Z","spawnCount":0}`))
		case strings.Contains(r.URL.Path, "/builds/") && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"buildID":"build-1","templateID":"tpl-env","status":"ready","image":"demo:v1","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:01Z"}`))
		case r.URL.Path == "/api/v1/templates/resolve/demo" && r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`{"templateID":"tpl-env","public":false}`))
		case r.URL.Path == "/api/v1/templates/tpl-env" && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	previousBaseURL := os.Getenv("E2B_DOMAIN")
	previousAPIKey := os.Getenv("E2B_API_KEY")
	previousProjectID := os.Getenv("SEACLOUD_PROJECT_ID")
	t.Setenv("E2B_DOMAIN", server.URL)
	t.Setenv("E2B_API_KEY", "unit-auth-value")
	t.Setenv("SEACLOUD_PROJECT_ID", "project-env")
	defer func() {
		_ = os.Setenv("E2B_DOMAIN", previousBaseURL)
		_ = os.Setenv("E2B_API_KEY", previousAPIKey)
		_ = os.Setenv("SEACLOUD_PROJECT_ID", previousProjectID)
	}()

	waitReady := true
	created, err := sandbox.Create(context.Background(), "", &sandbox.CreateOptions{WaitReady: &waitReady})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.SandboxID != "sb-env" {
		t.Fatalf("sandbox = %#v", created)
	}

	template := sandbox.NewTemplate().FromBaseImage().RunCmd("echo hello", nil)
	built, err := sandbox.BuildTemplate(context.Background(), template, "demo:v1", &sandbox.TemplateBuildOptions{})
	if err != nil {
		t.Fatalf("BuildTemplate: %v", err)
	}
	if built.TemplateID != "tpl-env" || built.Status != "ready" {
		t.Fatalf("built = %#v", built)
	}

	exists, err := sandbox.TemplateExists(context.Background(), "demo")
	if err != nil {
		t.Fatalf("TemplateExists: %v", err)
	}
	if !exists {
		t.Fatal("expected template to exist")
	}

	if err := sandbox.DeleteTemplate(context.Background(), "demo"); err != nil {
		t.Fatalf("DeleteTemplate: %v", err)
	}
}

func TestClientCreateAllowsMissingTemplateID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sandboxes" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode sandbox request: %v", err)
		}
		if _, ok := req["templateID"]; ok {
			t.Fatalf("templateID should be omitted, got %#v", req["templateID"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"sandboxID":"sb-default","templateID":"base","status":"running","startedAt":"2026-01-01T00:00:00Z","endAt":"2026-01-01T01:00:00Z"}`))
	}))
	defer server.Close()

	client := newSDKClient(t, server.URL)

	created, err := client.Create(context.Background(), "", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.TemplateID != "base" {
		t.Fatalf("templateID = %q", created.TemplateID)
	}
}
