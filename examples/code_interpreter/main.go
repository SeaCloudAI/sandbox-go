package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/SeaCloudAI/sandbox-go"
	"github.com/SeaCloudAI/sandbox-go/core"
)

func main() {
	ctx := context.Background()
	mustEnv("E2B_API_KEY")

	templateID := mustEnv("SANDBOX_EXAMPLE_TEMPLATE_ID")
	keepResources := envEnabled("SANDBOX_EXAMPLE_KEEP_RESOURCES")
	if looksLikeBaseTemplate(templateID) {
		log.Printf("warning: code_interpreter expects a code-interpreter template; base is usually not enough")
	}
	waitReady := true
	timeout := int32(1800)

	sbx, err := sandbox.Create(ctx, templateID, &sandbox.CreateOptions{
		WaitReady: &waitReady,
		Timeout:   &timeout,
	}, core.WithTimeout(180*time.Second))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("sandbox created: %s %s", sbx.SandboxID, sbx.Status)

	if !keepResources {
		defer func() {
			if err := sbx.Delete(ctx); err != nil {
				log.Printf("delete sandbox warning: %v", err)
			}
		}()
	}

	python1, err := sbx.RunCode(ctx, "x = 41\nx", nil)
	if err != nil {
		log.Fatal(err)
	}
	python2, err := sbx.RunCode(ctx, "x + 1", nil)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("default python context: %s -> %s", strings.TrimSpace(python1.Text()), strings.TrimSpace(python2.Text()))

	pythonContextTimeout := 30
	pythonContext, err := sbx.CreateCodeContext(ctx, &sandbox.CodeContextCreateOptions{
		Language: "python",
		CWD:      "/workspace",
		Timeout:  &pythonContextTimeout,
	})
	if err != nil {
		log.Fatal(err)
	}
	if _, err := sbx.RunCode(ctx, "name = 'go-sdk'", &sandbox.RunCodeOptions{Context: pythonContext}); err != nil {
		log.Fatal(err)
	}
	pythonIsolated, err := sbx.RunCode(ctx, "name.upper()", &sandbox.RunCodeOptions{Context: pythonContext})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("explicit python context: %s", strings.TrimSpace(pythonIsolated.Text()))

	bashContextTimeout := 10
	bashContext, err := sbx.CreateCodeContext(ctx, &sandbox.CodeContextCreateOptions{
		Language: "bash",
		CWD:      "/workspace",
		Timeout:  &bashContextTimeout,
	})
	if err != nil {
		log.Fatal(err)
	}
	bashRun, err := sbx.RunCode(ctx, "pwd && echo bash-ok", &sandbox.RunCodeOptions{Context: bashContext})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("bash profile output: %#v", bashRun.Logs.Stdout)

	for _, contextDef := range sbx.ListCodeContexts() {
		log.Printf("context: id=%s language=%s cwd=%s", contextDef.ContextID, contextDef.Language, contextDef.CWD)
	}

	if _, err := sbx.RestartCodeContext(ctx, pythonContext); err != nil {
		log.Fatal(err)
	}
	if err := sbx.RemoveCodeContext(ctx, bashContext); err != nil {
		log.Fatal(err)
	}
	if err := sbx.RemoveCodeContext(ctx, pythonContext); err != nil {
		log.Fatal(err)
	}

	fmt.Println("done")
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

func looksLikeBaseTemplate(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return normalized == "base" || strings.HasPrefix(normalized, "tpl-base")
}
