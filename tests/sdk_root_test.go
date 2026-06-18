package tests

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SeaCloudAI/sandbox-go"
	"github.com/SeaCloudAI/sandbox-go/control"
	"github.com/SeaCloudAI/sandbox-go/core"
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
		if got := r.Header.Get("Authorization"); got != "Bearer unit-auth-value" {
			t.Fatalf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	t.Setenv("SEACLOUD_BASE_URL", server.URL+"/api/v1")
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

func TestPackageLevelHelpersDefaultToProductionSandboxServiceAPI(t *testing.T) {
	t.Setenv("SEACLOUD_BASE_URL", "")
	t.Setenv("SEACLOUD_API_KEY", "unit-auth-value")

	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.URL.String(); got != "https://sandbox-service.real-cloud.seaart.ai/api/v1/sandbox/sandboxes" {
			t.Fatalf("url = %q", got)
		}
		if got := r.Header.Get("X-API-Key"); got != "unit-auth-value" {
			t.Fatalf("api key = %q", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`[]`)),
			Request:    r,
		}, nil
	})}

	paginator, err := sandbox.List(context.Background(), nil, core.WithHTTPClient(httpClient))
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

func TestControlServiceUsesConfiguredBaseURLPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/sandbox/sandboxes" || r.Method != http.MethodGet {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-API-Key"); got != "unit-auth-value" {
			t.Fatalf("api key = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer unit-auth-value" {
			t.Fatalf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	service, err := control.NewService(server.URL+"/api/v1/sandbox", "unit-auth-value")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	if _, err := service.ListSandboxes(context.Background(), nil); err != nil {
		t.Fatalf("ListSandboxes: %v", err)
	}
}
