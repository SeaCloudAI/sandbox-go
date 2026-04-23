package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/SeaCloudAI/sandbox-go"
	"github.com/SeaCloudAI/sandbox-go/build"
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

	client, err := sandbox.NewClient(
		baseURL,
		apiKey,
	)
	if err != nil {
		log.Fatal(err)
	}

	name := fmt.Sprintf("go-build-example-%d", time.Now().UnixNano())
	image := strings.TrimSpace(os.Getenv("SANDBOX_EXAMPLE_BUILD_IMAGE"))
	if image == "" {
		image = "docker.io/library/alpine:3.20"
	}
	created, err := client.Build.CreateTemplate(ctx, &build.TemplateCreateRequest{
		Name:       name,
		Visibility: "personal",
		Image:      image,
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("created template=%s build=%s names=%v", created.TemplateID, created.BuildID, created.Names)

	keepResources := strings.TrimSpace(strings.ToLower(os.Getenv("SANDBOX_EXAMPLE_KEEP_RESOURCES")))
	if keepResources != "1" && keepResources != "true" && keepResources != "yes" {
		defer func() {
			if err := client.Build.DeleteTemplate(ctx, created.TemplateID); err != nil {
				log.Printf("delete template warning: %v", err)
				return
			}
			log.Printf("deleted template=%s", created.TemplateID)
		}()
	}

	detail, err := client.Build.GetTemplate(ctx, created.TemplateID, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("detail template=%s image=%s visibility=%s", detail.TemplateID, detail.Image, detail.Visibility)

	builds, err := client.Build.ListBuilds(ctx, created.TemplateID)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("build history count=%d", len(builds.Builds))
}
