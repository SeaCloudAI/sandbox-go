package tests

import (
	"testing"

	"github.com/SeaCloudAI/sandbox-go"
)

func TestRootClientInitializesBuildAndRuntime(t *testing.T) {
	client, err := sandbox.NewClient("https://hermes-gateway.sandbox.cloud.vtrix.ai", "unit-auth-value")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client.Build == nil {
		t.Fatal("build handle is nil")
	}

	runtime, err := client.Runtime("https://hermes-gateway.sandbox.cloud.vtrix.ai", "unit-runtime-auth")
	if err != nil {
		t.Fatalf("Runtime: %v", err)
	}
	if got := runtime.BaseURL(); got != "https://hermes-gateway.sandbox.cloud.vtrix.ai" {
		t.Fatalf("runtime baseURL = %q", got)
	}
}
