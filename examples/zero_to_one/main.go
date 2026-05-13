package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SeaCloudAI/sandbox-go"
	"github.com/SeaCloudAI/sandbox-go/core"
)

func main() {
	ctx := context.Background()
	mustEnv("SEACLOUD_API_KEY")

	baseTemplate := env("SANDBOX_EXAMPLE_BASE_TEMPLATE", "base")
	codeTemplate := env("SANDBOX_EXAMPLE_CODE_TEMPLATE", "code-interpreter")
	frontendTemplate := env("SANDBOX_EXAMPLE_FRONTEND_TEMPLATE", codeTemplate)
	keepResources := envEnabled("SANDBOX_EXAMPLE_KEEP_RESOURCES")
	transportOpts := gatewayTransportOptions()

	var baseSandbox *sandbox.Sandbox
	var frontendSandbox *sandbox.Sandbox
	var builtTemplateID string
	var tempAppDir string

	defer func() {
		if tempAppDir != "" {
			_ = os.RemoveAll(tempAppDir)
		}
		if !keepResources && frontendSandbox != nil {
			_ = frontendSandbox.Delete(ctx)
		}
		if !keepResources && baseSandbox != nil {
			_ = baseSandbox.Delete(ctx)
		}
		if !keepResources && builtTemplateID != "" {
			_ = sandbox.DeleteTemplate(ctx, builtTemplateID, transportOpts...)
		}
	}()

	waitReady := true
	timeout := int64(1800)
	var err error
	baseSandbox, err = sandbox.Create(ctx, baseTemplate, &sandbox.CreateOptions{
		WaitReady: &waitReady,
		Timeout:   &timeout,
	}, transportOpts...)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("base sandbox: sandbox=%s domain=%s", baseSandbox.SandboxID, sandboxDomain(baseSandbox.EnvdURL))

	files, err := baseSandbox.Files()
	if err != nil {
		log.Fatal(err)
	}
	if _, err := files.Write(ctx, "/root/workspace/hello.txt", []byte("hello from a sandbox\n")); err != nil {
		log.Fatal(err)
	}
	hello, err := files.Read(ctx, "/root/workspace/hello.txt", nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("file read: %s", strings.TrimSpace(fmt.Sprint(hello)))

	commands, err := baseSandbox.Commands()
	if err != nil {
		log.Fatal(err)
	}
	command, err := commands.Run(ctx, "sh", &sandbox.CommandRunOptions{
		Args: []string{"-lc", "pwd && uname -a && ls -la /root/workspace"},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("command exit: %d", command.ExitCode)
	log.Print(strings.TrimSpace(command.Stdout))

	if err := baseSandbox.SetTimeout(ctx, 1800); err != nil {
		log.Fatal(err)
	}
	log.Printf("is running: %v", baseSandbox.IsRunning())

	paused, err := baseSandbox.Pause(ctx)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("paused: %v", paused)
	resumedSandbox, err := baseSandbox.Resume(ctx, timeout)
	if err != nil {
		log.Fatal(err)
	}
	baseSandbox = resumedSandbox
	log.Printf("resumed: %v", baseSandbox.IsRunning())

	codeSandbox, err := sandbox.Create(ctx, codeTemplate, &sandbox.CreateOptions{
		WaitReady: &waitReady,
		Timeout:   &timeout,
	}, transportOpts...)
	if err != nil {
		log.Fatal(err)
	}
	result, err := codeSandbox.RunCode(ctx, "x = 41\nx + 1", nil)
	if err != nil {
		_ = codeSandbox.Delete(ctx)
		log.Fatal(err)
	}
	log.Printf("code interpreter result: %s", result.Text())
	if !keepResources {
		_ = codeSandbox.Delete(ctx)
	}

	frontendSandbox, err = sandbox.Create(ctx, frontendTemplate, &sandbox.CreateOptions{
		WaitReady: &waitReady,
		Timeout:   &timeout,
	}, transportOpts...)
	if err != nil {
		log.Fatal(err)
	}
	frontendFiles, err := frontendSandbox.Files()
	if err != nil {
		log.Fatal(err)
	}
	if _, err := frontendFiles.MakeDir(ctx, "/root/workspace/frontend"); err != nil {
		log.Fatal(err)
	}
	if _, err := frontendFiles.Write(ctx, "/root/workspace/frontend/index.html", []byte(frontendHTML("runtime frontend"))); err != nil {
		log.Fatal(err)
	}
	frontendCommands, err := frontendSandbox.Commands()
	if err != nil {
		log.Fatal(err)
	}
	if _, err := frontendCommands.Start(ctx, "python3", &sandbox.CommandRunOptions{
		Args: []string{"-m", "http.server", "3000", "--bind", "0.0.0.0"},
		CWD:  "/root/workspace/frontend",
		OnStdout: func(data string) {
			fmt.Print(data)
		},
		OnStderr: func(data string) {
			fmt.Fprint(os.Stderr, data)
		},
	}); err != nil {
		log.Fatal(err)
	}
	frontendURL, err := frontendSandbox.GetHost(3000)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("frontend url: %s", frontendURL)

	tempAppDir, err = os.MkdirTemp("", "sandbox-frontend-")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tempAppDir, "index.html"), []byte(frontendHTML("template frontend")), 0o644); err != nil {
		log.Fatal(err)
	}

	wait := true
	built, err := sandbox.BuildTemplate(
		ctx,
		sandbox.NewTemplate().
			FromTemplate(baseTemplate).
			Copy(tempAppDir, "/workspace/frontend", &sandbox.TemplateCopyOptions{ForceUpload: true}).
			SetStartCmd(
				"cd /workspace/frontend && python3 -m http.server 3000 --bind 0.0.0.0",
				sandbox.WaitForPort(3000),
			),
		fmt.Sprintf("go-local-frontend-%d:v1", time.Now().UnixNano()),
		&sandbox.TemplateBuildOptions{
			Wait:         &wait,
			PollInterval: 2 * time.Second,
		},
		transportOpts...,
	)
	if err != nil {
		log.Fatal(err)
	}
	builtTemplateID = built.TemplateID
	log.Printf("built template: template=%s build=%s", built.TemplateID, built.BuildID)

	if keepResources {
		log.Printf(
			"kept resources: baseSandbox=%s frontendSandbox=%s builtTemplate=%s",
			baseSandbox.SandboxID,
			frontendSandbox.SandboxID,
			builtTemplateID,
		)
	}
}

func frontendHTML(title string) string {
	return fmt.Sprintf(`<!doctype html>
<html>
  <head><meta charset="utf-8"><title>%s</title></head>
  <body>
    <h1>%s</h1>
    <p>Served from a SeaCloudAI sandbox.</p>
  </body>
</html>
`, title, title)
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func mustEnv(name string) string {
	value := env(name, "")
	if value == "" {
		log.Fatalf("%s is required", name)
	}
	return value
}

func envEnabled(name string) bool {
	switch strings.ToLower(env(name, "")) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func gatewayTransportOptions() []core.TransportOption {
	return []core.TransportOption{core.WithTimeout(180 * time.Second)}
}

func sandboxDomain(value *string) string {
	if value == nil {
		return ""
	}
	trimmed := strings.TrimSpace(*value)
	trimmed = strings.TrimPrefix(trimmed, "https://")
	trimmed = strings.TrimPrefix(trimmed, "http://")
	return strings.TrimRight(trimmed, "/")
}
