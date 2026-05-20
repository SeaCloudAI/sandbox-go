package tests

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/SeaCloudAI/sandbox-go/control"
	"github.com/SeaCloudAI/sandbox-go/core"
)

func TestIntegrationSandboxMetrics(t *testing.T) {
	if os.Getenv("SANDBOX_RUN_INTEGRATION") != "1" {
		t.Skip("set SANDBOX_RUN_INTEGRATION=1 to run integration tests")
	}

	baseURL := strings.TrimSpace(os.Getenv("SANDBOX_TEST_BASE_URL"))
	apiKey := strings.TrimSpace(os.Getenv("SANDBOX_TEST_API_KEY"))
	sandboxID := strings.TrimSpace(os.Getenv("SANDBOX_TEST_SANDBOX_ID"))
	if baseURL == "" || apiKey == "" || sandboxID == "" {
		t.Skip("SANDBOX_TEST_BASE_URL, SANDBOX_TEST_API_KEY and SANDBOX_TEST_SANDBOX_ID are required")
	}

	service, err := control.NewService(baseURL, apiKey, core.WithTimeout(60*time.Second))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	ctx := context.Background()
	single, err := service.GetSandboxMetrics(ctx, sandboxID)
	if err != nil {
		t.Fatalf("GetSandboxMetrics: %v", err)
	}
	if single.SandboxID != sandboxID {
		t.Fatalf("single sandbox id = %q, want %q", single.SandboxID, sandboxID)
	}

	batch, err := service.ListSandboxMetrics(ctx, &control.SandboxMetricsParams{
		SandboxIDs: []string{sandboxID},
		Limit:      1,
	})
	if err != nil {
		t.Fatalf("ListSandboxMetrics: %v", err)
	}
	if len(batch.Items) != 1 || batch.Items[0].SandboxID != sandboxID {
		t.Fatalf("batch items = %#v", batch.Items)
	}
}
