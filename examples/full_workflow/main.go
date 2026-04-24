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
	"github.com/SeaCloudAI/sandbox-go/cmd"
	"github.com/SeaCloudAI/sandbox-go/control"
	"github.com/SeaCloudAI/sandbox-go/core"
)

func main() {
	ctx := context.Background()

	baseURL := mustEnv("SEACLOUD_BASE_URL")
	apiKey := mustEnv("SEACLOUD_API_KEY")
	runtimeBaseImage := mustEnv("SANDBOX_EXAMPLE_RUNTIME_BASE_IMAGE")
	keepResources := envEnabled("SANDBOX_EXAMPLE_KEEP_RESOURCES")

	client, err := sandbox.NewClient(
		baseURL,
		apiKey,
		core.WithTimeout(180*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	logMetricLine("control", func() (string, error) {
		return client.Metrics(ctx)
	})
	logMetricLine("build", func() (string, error) {
		return client.Build.Metrics(ctx)
	})

	templateName := fmt.Sprintf("go-full-workflow-%d", time.Now().UnixNano())
	templateResp, err := client.Build.CreateTemplate(ctx, &build.TemplateCreateRequest{
		Name:       templateName,
		Visibility: "personal",
		Dockerfile: dockerfile(runtimeBaseImage),
	})
	if err != nil {
		log.Fatal(err)
	}

	templateID := templateResp.TemplateID
	buildID := templateResp.BuildID
	log.Printf("template created: template=%s build=%s", templateID, buildID)

	if !keepResources {
		defer func() {
			if err := client.Build.DeleteTemplate(ctx, templateID); err != nil {
				log.Printf("delete template warning: %v", err)
				return
			}
			log.Printf("deleted template=%s", templateID)
		}()
	}

	if buildID == "" {
		templateDetail, err := client.Build.GetTemplate(ctx, templateID, nil)
		if err != nil {
			log.Fatal(err)
		}
		buildID = templateDetail.BuildID
	}
	if buildID == "" {
		log.Fatal("buildID is empty")
	}

	buildStatus, err := waitForBuildReady(ctx, client.Build, templateID, buildID)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("build ready: template=%s build=%s status=%s", templateID, buildID, buildStatus.Status)

	buildDetail, err := client.Build.GetBuild(ctx, templateID, buildID)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("build detail: status=%s image=%s", buildDetail.Status, buildDetail.Image)

	buildLogsLimit := 10
	buildLogs, err := client.Build.GetBuildLogs(ctx, templateID, buildID, &build.BuildLogsParams{Limit: &buildLogsLimit})
	if err != nil {
		log.Printf("build logs warning: %v", err)
	} else {
		log.Printf("build logs: count=%d last=%q", len(buildLogs.Logs), latestBuildLog(buildLogs, buildStatus))
	}

	templateDetail, err := client.Build.GetTemplate(ctx, templateID, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("template detail: name=%s imageSource=%s buildStatus=%s", templateDetail.Name, templateDetail.ImageSource, templateDetail.BuildStatus)

	waitReady := true
	timeout := int32(1800)
	createdSandbox, err := client.CreateSandbox(ctx, &control.NewSandboxRequest{
		TemplateID: templateID,
		WaitReady:  &waitReady,
		Timeout:    &timeout,
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("sandbox created: sandbox=%s status=%s", createdSandbox.SandboxID, createdSandbox.Status)

	if !keepResources {
		defer func() {
			if err := createdSandbox.Delete(ctx); err != nil {
				log.Printf("delete sandbox warning: %v", err)
				return
			}
			log.Printf("deleted sandbox=%s", createdSandbox.SandboxID)
		}()
	}

	sandboxDetail, err := createdSandbox.Reload(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("sandbox detail: state=%s status=%s", sandboxDetail.State, sandboxDetail.Status)

	sandboxLogsLimit := 10
	sandboxLogs, err := sandboxDetail.Logs(ctx, &control.SandboxLogsParams{Limit: &sandboxLogsLimit})
	if err != nil {
		log.Printf("sandbox logs warning: %v", err)
	} else {
		log.Printf("sandbox logs: count=%d last=%q", len(sandboxLogs.Logs), latestSandboxLog(sandboxLogs))
	}

	connected, err := sandboxDetail.Connect(ctx, &control.ConnectSandboxRequest{Timeout: timeout})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("sandbox connected: statusCode=%d sandbox=%s", connected.StatusCode, connected.Sandbox.SandboxID)

	runtime, err := connected.Sandbox.Runtime()
	if err != nil {
		log.Fatal(err)
	}

	runtimeMetrics, err := runtime.Metrics(ctx)
	if err != nil {
		log.Printf("runtime metrics warning: %v", err)
	} else {
		log.Printf(
			"runtime metrics: cpu=%.2f%% mem=%d/%d MiB disk=%d/%d",
			runtimeMetrics.CPUUsedPct,
			runtimeMetrics.MemUsedMiB,
			runtimeMetrics.MemTotalMiB,
			runtimeMetrics.DiskUsed,
			runtimeMetrics.DiskTotal,
		)
	}

	listing, err := runtime.ListDir(ctx, &cmd.ListDirRequest{Path: "/workspace"}, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("workspace entries=%d", len(listing.Entries))

	runResp, err := runtime.Run(ctx, &cmd.AgentRunRequest{
		Cmd:  "sh",
		Args: []string{"-lc", "cat /workspace/built-by-template.txt && echo workflow-ok"},
	}, nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("run exit=%d stdout=%q stderr=%q", runResp.ExitCode, runResp.Stdout, runResp.Stderr)

	if keepResources {
		log.Printf("kept resources: template=%s sandbox=%s", templateID, createdSandbox.SandboxID)
	}
}

func mustEnv(name string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		log.Fatalf("%s is required", name)
	}
	return value
}

func envEnabled(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func dockerfile(runtimeBaseImage string) string {
	return fmt.Sprintf(
		"FROM %s\nRUN mkdir -p /workspace && printf 'hello from go full workflow\\n' >/workspace/built-by-template.txt\n",
		runtimeBaseImage,
	)
}

func firstNonEmptyLine(input string) string {
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func logMetricLine(name string, fn func() (string, error)) {
	value, err := fn()
	if err != nil {
		log.Printf("%s metrics warning: %v", name, err)
		return
	}
	log.Printf("%s metrics: %s", name, firstNonEmptyLine(value))
}

func latestBuildLog(logs *build.BuildLogsResponse, status *build.BuildStatusResponse) string {
	if logs != nil && len(logs.Logs) > 0 {
		return logs.Logs[len(logs.Logs)-1].Message
	}
	if status != nil && len(status.LogEntries) > 0 {
		return status.LogEntries[len(status.LogEntries)-1].Message
	}
	if status != nil && len(status.Logs) > 0 {
		return status.Logs[len(status.Logs)-1]
	}
	return ""
}

func latestSandboxLog(logs *control.SandboxLogsResponse) string {
	if logs == nil || len(logs.Logs) == 0 {
		return ""
	}
	return logs.Logs[len(logs.Logs)-1].Message
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

		switch status.Status {
		case "ready":
			return status, nil
		case "error":
			return nil, fmt.Errorf("build failed: status=%s reason=%v", status.Status, status.Reason)
		}

		time.Sleep(2 * time.Second)
	}

	return nil, fmt.Errorf("build did not complete before deadline: %#v", last)
}
