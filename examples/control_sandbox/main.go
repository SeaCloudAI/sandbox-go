package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/SeaCloudAI/sandbox-go"
	"github.com/SeaCloudAI/sandbox-go/control"
)

func main() {
	ctx := context.Background()
	baseURL := strings.TrimSpace(os.Getenv("SEACLOUD_BASE_URL"))
	if baseURL == "" {
		log.Fatal("SEACLOUD_BASE_URL is required")
	}

	apiKey := strings.TrimSpace(os.Getenv("SEACLOUD_API_KEY"))
	if apiKey == "" {
		log.Fatal("SEACLOUD_API_KEY is required")
	}

	templateID := strings.TrimSpace(os.Getenv("SANDBOX_EXAMPLE_TEMPLATE_ID"))
	if templateID == "" {
		log.Fatal("SANDBOX_EXAMPLE_TEMPLATE_ID is required")
	}

	client, err := sandbox.NewClient(
		baseURL,
		apiKey,
	)
	if err != nil {
		log.Fatal(err)
	}

	waitReady := true
	timeout := int32(1800)
	created, err := client.CreateSandbox(ctx, &control.NewSandboxRequest{
		TemplateID: templateID,
		WaitReady:  &waitReady,
		Timeout:    &timeout,
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("created sandbox=%s status=%s envd=%v", created.SandboxID, created.Status, created.EnvdURL)
	if created.EnvdURL != nil {
		runtime, err := created.Runtime()
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("bound runtime baseURL=%s", runtime.BaseURL())
	}

	keepResources := strings.TrimSpace(strings.ToLower(os.Getenv("SANDBOX_EXAMPLE_KEEP_RESOURCES")))
	if keepResources != "1" && keepResources != "true" && keepResources != "yes" {
		defer func() {
			if err := created.Delete(ctx); err != nil {
				log.Printf("delete sandbox warning: %v", err)
				return
			}
			log.Printf("deleted sandbox=%s", created.SandboxID)
		}()
	}

	detail, err := created.Reload(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("detail sandbox=%s state=%s status=%s", detail.SandboxID, detail.State, detail.Status)
}
