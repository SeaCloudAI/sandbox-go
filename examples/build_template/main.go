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
		Name:  name,
		Alias: name,
	})
	if err != nil {
		log.Fatal(err)
	}

	aliased, err := client.Build.GetTemplateByAlias(ctx, name)
	if err != nil {
		log.Fatal(err)
	}
	resolved, err := client.Build.ResolveTemplateRef(ctx, name)
	if err != nil {
		log.Fatal(err)
	}

	requestedBuildID := fmt.Sprintf("build-%x", time.Now().UnixNano())
	if _, err := client.Build.CreateBuild(
		ctx,
		created.TemplateID,
		requestedBuildID,
		build.NewTemplateBuildBuilder().
			FromImage(image).
			Run("echo 'hello from go build example' >/tmp/built-by-go-example.txt", nil).
			Request(),
	); err != nil {
		log.Fatal(err)
	}
	buildID := requestedBuildID

	log.Printf(
		"created template=%s alias=%s aliasLookup=%s resolved=%s build=%s",
		created.TemplateID,
		name,
		aliased.TemplateID,
		resolved.TemplateID,
		buildID,
	)

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

	status, err := waitForBuildReady(ctx, client.Build, created.TemplateID, buildID)
	if err != nil {
		log.Fatal(err)
	}
	buildDetail, err := client.Build.GetBuild(ctx, created.TemplateID, buildID)
	if err != nil {
		log.Fatal(err)
	}
	history, err := client.Build.ListBuilds(ctx, created.TemplateID)
	if err != nil {
		log.Fatal(err)
	}

	detail, err := client.Build.GetTemplate(ctx, created.TemplateID, nil)
	if err != nil {
		log.Fatal(err)
	}
	visibility := ""
	if detail.Extensions != nil && detail.Extensions.Seacloud != nil {
		visibility = detail.Extensions.Seacloud.Visibility
	}
	log.Printf(
		"detail template=%s names=%v builds=%d visibility=%s status=%s image=%s history=%d",
		detail.TemplateID,
		detail.Names,
		len(detail.Builds),
		visibility,
		status.Status,
		buildDetail.Image,
		history.Total,
	)
}

func waitForBuildReady(
	ctx context.Context,
	service *build.Service,
	templateID string,
	buildID string,
) (*build.BuildStatusResponse, error) {
	deadline := time.Now().Add(3 * time.Minute)
	limit := 20
	var last *build.BuildStatusResponse

	for time.Now().Before(deadline) {
		status, err := service.GetBuildStatus(ctx, templateID, buildID, &build.BuildStatusParams{Limit: &limit})
		if err != nil {
			return nil, err
		}
		last = status
		if status.Status == "ready" {
			return status, nil
		}
		if status.Status == "error" {
			return nil, fmt.Errorf("build failed: %#v", status)
		}
		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("build did not complete before deadline: %#v", last)
}
