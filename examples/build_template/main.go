package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/SeaCloudAI/sandbox-go"
)

func main() {
	ctx := context.Background()
	if os.Getenv("SEACLOUD_API_KEY") == "" {
		log.Fatal("SEACLOUD_API_KEY is required")
	}

	image := strings.TrimSpace(os.Getenv("SANDBOX_EXAMPLE_BUILD_IMAGE"))
	if image == "" {
		image = "docker.io/library/alpine:3.20"
	}
	name := "go-build-example-" + strconv.FormatInt(time.Now().UnixNano(), 10) + ":v1"
	built, err := sandbox.BuildTemplate(
		ctx,
		sandbox.NewTemplate().
			FromImage(image).
			RunCmd("echo 'hello from go build example' >/tmp/built-by-go-example.txt", nil).
			SetReadyCmd(sandbox.WaitForFile("/tmp/built-by-go-example.txt")),
		name,
		&sandbox.TemplateBuildOptions{
			OnBuildLog: func(entry sandbox.LogEntry) {
				log.Println(entry.String())
			},
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	keepResources := strings.TrimSpace(strings.ToLower(os.Getenv("SANDBOX_EXAMPLE_KEEP_RESOURCES")))
	if keepResources != "1" && keepResources != "true" && keepResources != "yes" {
		defer func() {
			if err := sandbox.DeleteTemplate(ctx, built.TemplateID); err != nil {
				log.Printf("delete template warning: %v", err)
				return
			}
			log.Printf("deleted template=%s", built.TemplateID)
		}()
	}
	log.Printf(
		"detail template=%s names=%v builds=%d status=%s image=%s build=%s",
		built.Template.TemplateID,
		built.Template.Names,
		len(built.Template.Builds),
		built.Status,
		built.Build.Image,
		built.BuildID,
	)
}
