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

	apiKey := mustEnv("SEACLOUD_API_KEY")
	gatewayBaseURL := firstNonEmpty(strings.TrimSpace(os.Getenv("SEACLOUD_BASE_URL")), "https://sandbox-gateway.cloud.seaart.ai")
	runtimeBaseImage := mustEnv("SANDBOX_EXAMPLE_RUNTIME_BASE_IMAGE")
	keepResources := envEnabled("SANDBOX_EXAMPLE_KEEP_RESOURCES")
	transportOpts := gatewayTransportOptions()

	controlService, err := control.NewService(gatewayBaseURL, apiKey, transportOpts...)
	if err != nil {
		log.Fatal(err)
	}
	buildService, err := build.NewService(gatewayBaseURL, apiKey, transportOpts...)
	if err != nil {
		log.Fatal(err)
	}

	logMetricLine("control", func() (string, error) {
		return controlService.Metrics(ctx)
	})
	logMetricLine("build", func() (string, error) {
		return buildService.Metrics(ctx)
	})

	templateName := fmt.Sprintf("go-full-workflow-%d", time.Now().UnixNano())
	buildLogCount := 0
	built, err := sandbox.BuildTemplate(
		ctx,
		sandbox.NewTemplate().
			FromImage(runtimeBaseImage).
			RunCmd("mkdir -p /workspace && printf 'hello from go full workflow\\n' >/workspace/built-by-template.txt", nil).
			SetReadyCmd(sandbox.WaitForFile("/workspace/built-by-template.txt")),
		templateName,
		&sandbox.TemplateBuildOptions{
			PollInterval: 2 * time.Second,
			OnBuildLog: func(entry sandbox.LogEntry) {
				buildLogCount++
				log.Println(entry.String())
			},
		},
		transportOpts...,
	)
	if err != nil {
		log.Fatal(err)
	}

	templateID := built.TemplateID
	buildID := built.BuildID
	log.Printf("build ready: template=%s build=%s status=%s", templateID, buildID, built.Status)
	log.Printf("build detail: status=%s image=%s", built.Build.Status, built.Build.Image)

	if !keepResources {
		defer func() {
			if err := sandbox.DeleteTemplate(ctx, templateID, transportOpts...); err != nil {
				log.Printf("delete template warning: %v", err)
				return
			}
			log.Printf("deleted template=%s", templateID)
		}()
	}

	buildStatus, err := sandbox.GetTemplateBuildStatus(ctx, templateID, buildID, &sandbox.TemplateBuildStatusOptions{
		Limit: intPtr(20),
	}, transportOpts...)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("build logs: count=%d last=%q", buildLogCount, latestBuildLog(buildStatus))

	templateDetail, err := sandbox.GetTemplate(ctx, templateID, nil, transportOpts...)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("template detail: names=%v buildCount=%v", templateDetail.Names, templateDetail.BuildCount)

	waitReady := true
	timeout := int64(1800)
	createdSandbox, err := sandbox.Create(ctx, templateID, &sandbox.CreateOptions{
		WaitReady: &waitReady,
		Timeout:   &timeout,
	}, transportOpts...)
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

func firstNonEmptyLine(input string) string {
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
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

func latestBuildLog(status *build.BuildStatusResponse) string {
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

func intPtr(value int) *int {
	return &value
}

func gatewayTransportOptions() []core.TransportOption {
	return []core.TransportOption{core.WithTimeout(180 * time.Second)}
}
