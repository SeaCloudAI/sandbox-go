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

func TestPackageLevelHelpersUseSeaCloudGatewayEnv(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sandboxes" || r.Method != http.MethodGet {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-API-Key"); got != "unit-auth-value" {
			t.Fatalf("api key = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_BASE_URL", server.URL)
	t.Setenv("SEACLOUD_API_KEY", "unit-auth-value")

	paginator, err := sandbox.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	listed, err := paginator.NextPage(context.Background())
	if err != nil {
		t.Fatalf("NextPage: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("listed = %#v", listed)
	}
}
