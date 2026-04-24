package main

import (
	"context"
	"io"
	"log"
	"os"
	"strings"

	"github.com/SeaCloudAI/sandbox-go"
	"github.com/SeaCloudAI/sandbox-go/cmd"
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
	defer func() {
		if keepResources := strings.TrimSpace(strings.ToLower(os.Getenv("SANDBOX_EXAMPLE_KEEP_RESOURCES"))); keepResources == "1" || keepResources == "true" || keepResources == "yes" {
			return
		}
		if err := created.Delete(ctx); err != nil {
			log.Printf("delete sandbox warning: %v", err)
		}
	}()

	runtime, err := created.Runtime()
	if err != nil {
		log.Fatal(err)
	}

	root := "/tmp"
	filePath := root + "/go-cmd-example.txt"

	if err := runtime.WriteFile(ctx, &cmd.UploadBytesRequest{
		Path: filePath,
		Data: []byte("hello from go example"),
	}, nil); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote file=%s", filePath)

	read, err := runtime.ReadFile(ctx, &cmd.FileRequest{Path: filePath}, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer read.Body.Close()
	body, err := io.ReadAll(read.Body)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("file content=%q", body)

	listing, err := runtime.ListDir(ctx, &cmd.ListDirRequest{Path: root}, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("directory entries=%d", len(listing.Entries))

	run, err := runtime.Run(ctx, &cmd.AgentRunRequest{
		Cmd:  "sh",
		Args: []string{"-lc", "cat " + filePath},
	}, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("run exit=%d stdout=%q stderr=%q", run.ExitCode, run.Stdout, run.Stderr)
}
