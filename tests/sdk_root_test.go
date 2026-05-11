package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SeaCloudAI/sandbox-go"
)

func TestNewRuntimeInitializesBaseURL(t *testing.T) {
	runtime, err := sandbox.NewRuntime("https://sandbox-gateway.cloud.seaart.ai", "unit-runtime-auth")
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	if got := runtime.BaseURL(); got != "https://sandbox-gateway.cloud.seaart.ai" {
		t.Fatalf("runtime baseURL = %q", got)
	}
}

func TestPackageLevelHelpersUseE2BGatewayEnv(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sandboxes" || r.Method != http.MethodGet {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-API-Key"); got != "unit-auth-from-e2b" {
			t.Fatalf("api key = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	t.Setenv("E2B_DOMAIN", server.URL)
	t.Setenv("E2B_API_KEY", "unit-auth-from-e2b")

	listed, err := sandbox.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("listed = %#v", listed)
	}
}

func TestPackageLevelHelpersIgnoreSeaCloudCompatibilityEnvVars(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sandboxes" || r.Method != http.MethodGet {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	t.Setenv("E2B_DOMAIN", server.URL)
	t.Setenv("SEACLOUD_BASE_URL", "https://seacloud.example.test")
	t.Setenv("E2B_API_KEY", "unit-auth-from-e2b")
	t.Setenv("SEACLOUD_API_KEY", "unit-auth-from-seacloud")

	if _, err := sandbox.List(context.Background(), nil); err != nil {
		t.Fatalf("List: %v", err)
	}
}
