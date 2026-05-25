# SeaCloudAI Sandbox Go SDK

Run code, agent workflows, and lightweight services in isolated cloud sandboxes from Go. The SDK gives you a Go-native E2B-style workflow for sandbox lifecycle, files, commands, PTY, code execution, service proxying, and reusable template builds.

## Why SeaCloudAI Sandbox

- **Cloud sandboxes without infrastructure work**: create disposable isolated runtimes without managing containers, runtime tokens, or service proxies yourself.
- **One object for the full workflow**: create a sandbox, write files, run commands, open a PTY, clone git repos, expose a web service, inspect logs, and clean up from the same SDK.
- **Official templates for fast starts**: use `base` for files, commands, git, and PTY; use `code-interpreter` for multi-language code execution; use agent templates such as `claude` or `codex` when your environment publishes them.
- **Reusable environments**: prototype in a running sandbox, then bake stable setup into a custom `tpl-...` template that can be pinned and reused in production.
- **Familiar migration path**: lifecycle, filesystem, commands, PTY, code interpreter, and template helpers follow E2B-style patterns while using SeaCloudAI gateway and runtime configuration.
- **Same concepts across SDKs**: Node, Python, and Go expose the same core sandbox, runtime, and template-building model.

## Install

```bash
go get github.com/SeaCloudAI/sandbox-go
```

## Configure

Use environment variables for gateway configuration in all examples and quick starts:

- `SEACLOUD_BASE_URL`: preferred gateway entrypoint
- `SEACLOUD_API_KEY`: preferred API key

Set them once in your shell:

```bash
export SEACLOUD_BASE_URL="https://sandbox-gateway.cloud.seaart.ai"
export SEACLOUD_API_KEY="..."
```

Default production gateway:

```text
https://sandbox-gateway.cloud.seaart.ai
```

The SeaCloudAI production gateway is currently hosted under the `seaart.ai` domain.

High-level create helpers require an explicit template reference. Pass a concrete template ID such as `tpl-...` or a stable official template type such as `base`, `code-interpreter`, `claude`, or `codex` when your environment publishes those official templates.

## 60-Second Quickstart

```go
package main

import (
	"context"
	"log"

	sandbox "github.com/SeaCloudAI/sandbox-go"
)

func main() {
	ctx := context.Background()
	ready := true
	timeout := int64(1800)

	sbx, err := sandbox.Create(ctx, "base", &sandbox.CreateOptions{
		WaitReady: &ready,
		Timeout:   &timeout,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer sbx.Delete(ctx)

	files, err := sbx.Files()
	if err != nil {
		log.Fatal(err)
	}
	_, _ = files.Write(ctx, "/root/workspace/hello.txt", []byte("hello from SeaCloudAI\n"))

	commands, err := sbx.Commands()
	if err != nil {
		log.Fatal(err)
	}
	result, err := commands.Run(ctx, "sh", &sandbox.CommandRunOptions{
		Args: []string{"-lc", "cat /root/workspace/hello.txt && uname -a"},
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Print(result.Stdout)
}
```

That is the core loop: create an isolated cloud runtime, move files in, run real commands, and clean it up. Use `GetHost(port)` when you start an HTTP service and want a public proxy URL.

## Main Entrypoints

- `sandbox.Create(...)`, `sandbox.Connect(...)`, `sandbox.List(...)`: create and manage sandboxes.
- `Commands()`, `Files()`, `Git()`, `Pty()`: work inside a running sandbox.
- `RunCode(...)`: execute code in `code-interpreter` templates.
- `sandbox.NewTemplate()`, `sandbox.BuildTemplate(...)`, `sandbox.BuildTemplateInBackground(...)`: build reusable templates.
- `control.NewService(...)`, `build.NewService(...)`: low-level transports for advanced integrations.

Runtime access is derived from sandbox create/detail/connect responses. Do not hardcode runtime endpoints or tokens. The SDK stays Go-native with `context.Context` and `(..., error)` returns while keeping the same concepts as the Node and Python SDKs.

## E2B-Style Workflow

The high-level lifecycle, filesystem, command, git, PTY, code interpreter, and template APIs are designed to feel familiar to E2B users. Snapshot APIs are not exposed yet because the underlying platform does not support them. Python code contexts are stateful; non-Python code contexts currently behave as reusable execution profiles over isolated one-shot processes.

## Guided Walkthrough

This section is the recommended path for first-time users. It starts from environment setup, then creates sandboxes from official templates, runs commands, exposes a frontend through `envdUrl`, and finally builds a reusable custom template from local code.

### 1. Configure Environment

```bash
export SEACLOUD_BASE_URL="https://sandbox-gateway.cloud.seaart.ai"
export SEACLOUD_API_KEY="..."
```

Run the examples from `packages/go`:

```bash
go run ./examples/zero_to_one
```

### 2. Create A Base Sandbox

Use `base` for normal sandbox lifecycle, files, commands, git, and PTY. It is the right starting point for command execution and filesystem work.

```go
ctx := context.Background()
ready := true
timeout := int64(1800)

sbx, err := sandbox.Create(ctx, "base", &sandbox.CreateOptions{
	WaitReady: &ready,
	Timeout:   &timeout,
})
if err != nil {
	log.Fatal(err)
}
defer sbx.Delete(ctx)

files, err := sbx.Files()
if err != nil {
	log.Fatal(err)
}
_, _ = files.Write(ctx, "/root/workspace/hello.txt", []byte("hello\n"))
body, _ := files.Read(ctx, "/root/workspace/hello.txt", nil)
log.Print(body)

commands, err := sbx.Commands()
if err != nil {
	log.Fatal(err)
}
result, err := commands.Run(ctx, "sh", &sandbox.CommandRunOptions{
	Args: []string{"-lc", "pwd && uname -a && ls -la /root/workspace"},
})
if err != nil {
	log.Fatal(err)
}
log.Printf("exit=%d stdout=%s", result.ExitCode, result.Stdout)
```

### 3. Pick Official Templates By Workload

| Workload | Template | Use it for |
| --- | --- | --- |
| Basic shell, files, git, PTY, and lightweight services | `base` | General sandbox lifecycle and filesystem/command workflows. |
| Multi-language code execution | `code-interpreter` | `RunCode(...)` for Python, JavaScript, TypeScript, Bash, R, and Java. Python contexts are stateful. |
| Agent CLI workflows | `claude` / `codex` | Environments where those official agent templates are published with the CLIs preinstalled. |
| Reproducible production workloads | `tpl-...` | A concrete custom or official template ID pinned from config. |

```go
codeSandbox, err := sandbox.Create(ctx, "code-interpreter", &sandbox.CreateOptions{
	WaitReady: &ready,
})
if err != nil {
	log.Fatal(err)
}
defer codeSandbox.Delete(ctx)

execution, err := codeSandbox.RunCode(ctx, "x = 41\nx + 1", nil)
if err != nil {
	log.Fatal(err)
}
log.Print(execution.Text())
```

### 4. Manage Lifecycle

Lifecycle `Timeout` values are seconds. Runtime command `TimeoutMS` values are milliseconds.

```go
info, err := sbx.GetInfo(ctx)
if err != nil {
	log.Fatal(err)
}
log.Printf("sandbox=%s state=%s", info.SandboxID, info.State)

if err := sbx.SetTimeout(ctx, 3600); err != nil {
	log.Fatal(err)
}

paused, err := sbx.Pause(ctx)
if err != nil {
	log.Fatal(err)
}
log.Printf("paused=%v", paused)

sbx, err = sbx.Resume(ctx, 1800)
if err != nil {
	log.Fatal(err)
}
log.Printf("running=%v", sbx.IsRunning())
```

### 5. Deploy A Frontend And Open It Through `envdUrl`

Use a template that has Python or Node available. `code-interpreter` is a convenient default for this static frontend example because it can run `python3 -m http.server`.

```go
app, err := sandbox.Create(ctx, "code-interpreter", &sandbox.CreateOptions{
	WaitReady: &ready,
	Timeout:   &timeout,
})
if err != nil {
	log.Fatal(err)
}
defer app.Delete(ctx)

files, _ := app.Files()
_, _ = files.MakeDir(ctx, "/root/workspace/frontend")
_, _ = files.Write(ctx, "/root/workspace/frontend/index.html", []byte("<h1>Hello from sandbox</h1>"))

commands, _ := app.Commands()
_, err = commands.Start(ctx, "python3", &sandbox.CommandRunOptions{
	Args: []string{"-m", "http.server", "3000", "--bind", "0.0.0.0"},
	CWD:  "/root/workspace/frontend",
})
if err != nil {
	log.Fatal(err)
}

url, _ := app.GetHost(3000)
log.Print("open ", url)
```

`GetHost(3000)` derives a public proxy URL from the sandbox `EnvdURL`. Keep `EnvdAccessToken` / `TrafficAccessToken` private; they are sandbox-scoped secrets.

Service access notes:

- Bind HTTP services to `0.0.0.0`, not `127.0.0.1`, so the runtime proxy can reach them.
- Use `GetHost(port)` instead of constructing proxy URLs manually.
- If the URL does not open, check that the process is still running, the port matches, and the selected template exposes runtime access fields.

### 6. Upload Local Code Files

There are two common upload paths:

- Runtime upload to an existing sandbox: use `Files().Write(...)` / `WriteFiles(...)` when you want to place generated files into a running sandbox.
- Template build upload: use `Template.Copy(...)` when you want local files or directories baked into a reusable template image.

Upload a local file into a running sandbox:

```go
data, err := os.ReadFile("./my-frontend/index.html")
if err != nil {
	log.Fatal(err)
}
_, err = files.Write(ctx, "/root/workspace/frontend/index.html", data)
if err != nil {
	log.Fatal(err)
}
```

Upload one local file into a template build:

```go
sandbox.NewTemplate().
	FromTemplate("base").
	Copy("./package.json", "/workspace/app/package.json", &sandbox.TemplateCopyOptions{
		ForceUpload: true,
	})
```

Upload a local directory recursively:

```go
mode := 0o755
sandbox.NewTemplate().
	FromTemplate("base").
	Copy("./my-frontend", "/workspace/frontend", &sandbox.TemplateCopyOptions{
		ForceUpload:     true,
		Mode:            &mode,
		ResolveSymlinks: true,
	})
```

The first argument is a local path on your machine. The second argument is the destination path inside the template filesystem. `ForceUpload: true` is useful during development when the local files change frequently and you want the SDK to re-upload them instead of reusing a cached content hash.

Template build contexts are packed as `tar.gz` archives and must fit the server-side `maxContextBytes` limit returned by the build file handshake. The SDK validates the archive size before upload and sends the required GCS content-length-range header.

### 7. Build Your Own Template From Local Code

This uploads a local directory into the build context with `Copy(...)`, builds a reusable template image, and sets a startup command for future sandboxes created from that template.

```go
wait := true
built, err := sandbox.BuildTemplate(
	ctx,
	sandbox.NewTemplate().
		FromNodeImage("20-alpine").
		Copy("./my-frontend", "/app", &sandbox.TemplateCopyOptions{
			ForceUpload: true,
		}).
		RunCmd("cd /app && npm install", nil).
		SetStartCmd("cd /app && npm start", sandbox.WaitForPort(3000)),
	"my-frontend:v1",
	&sandbox.TemplateBuildOptions{
		Wait:         &wait,
		PollInterval: 2 * time.Second,
	},
)
if err != nil {
	log.Fatal(err)
}

log.Print(built.TemplateID, built.BuildID)
```

Use `FromTemplate("base")` when you want to inherit a published SeaCloud template, or `FromNodeImage(...)`, `FromPythonImage(...)`, and other image helpers when a public base image is enough. Advanced storage options such as NFS, block volumes, and object storage are documented later in the build reference.

Create a sandbox from the new template:

```go
customSandbox, err := sandbox.Create(ctx, built.TemplateID, &sandbox.CreateOptions{
	WaitReady: &ready,
})
if err != nil {
	log.Fatal(err)
}
defer customSandbox.Delete(ctx)

url, _ := customSandbox.GetHost(3000)
log.Print(url)
```

### 8. Recommended Production Flow

1. Prototype with an official template such as `base` or `code-interpreter`.
2. Upload local files to a running sandbox for fast iteration.
3. Move stable setup into `Copy(...)`, `RunCmd(...)`, `SetStartCmd(...)`, and `SetReadyCmd(...)`.
4. Build and pin the resulting `tpl-...` value in application config.
5. Keep sandbox cleanup in `defer` calls or a lifecycle manager, and set explicit lifecycle `Timeout` values for each workload.

## Troubleshooting

- `401` / `403`: verify `SEACLOUD_API_KEY` and that the process sees the environment variable.
- Requests go to the wrong gateway: check `SEACLOUD_BASE_URL`; include the `https://` scheme.
- Runtime APIs return `404`: use a template that supports managed runtime access and returns `EnvdURL` / `EnvdAccessToken`.
- `waitReady` or builds time out: increase lifecycle `Timeout` and SDK HTTP timeout through `core.WithTimeout(...)` for long starts or image builds.
- Frontend URL is unreachable: bind to `0.0.0.0`, confirm the port passed to `GetHost(...)`, and inspect whether the background process exited.
- Build with local files fails: make sure `Copy(...)` points to an existing local path and use `ForceUpload: true` while iterating.

## Diagnostics

The SDK is quiet by default. Use `core.WithDebugLogger()` for standard-log request diagnostics, or `core.WithLogger(...)` to receive structured, sanitized lifecycle events from control/build clients. Runtime CMD clients use the matching `cmd.WithDebugLogger()` and `cmd.WithLogger(...)` options. Every SDK request carries `X-Request-ID`; response and error events include the same ID when available.

```go
service, err := control.NewService(
	baseURL,
	apiKey,
	core.WithLogger(func(event core.DiagnosticEvent) {
		log.Printf("%s %s %s request_id=%s status=%d", event.Type, event.Method, event.Path, event.RequestID, event.StatusCode)
	}),
)
if err != nil {
	log.Fatal(err)
}
_, _ = service.ListSandboxes(context.Background(), nil)
```

Diagnostic events include method, path, request ID, status, duration, error kind, and retryability. They intentionally exclude request/response bodies and credential headers; sensitive query values such as tokens, signatures, and `api_key` are redacted, including when a transport error embeds a URL. Logger failures are ignored so diagnostics cannot change request behavior.

## Production Readiness

- Package-level helpers are fine for simple env-first flows. For repeated low-level workflows, initialize one `control.Service` and/or `build.Service` and reuse them.
- Treat every quick start as creating billable or quota-bound resources unless it explicitly cleans them up.
- Prefer explicit template references from configuration over hardcoded example values.
- In SeaCloudAI environments, prefer official template types such as `base`, `code-interpreter`, `claude`, or `codex` when you want a stable platform-managed entrypoint.
- Template semantics matter: `base` is the minimal runtime template for lifecycle, files, commands, git, and PTY. It does not imply a multi-language execution environment. Use `code-interpreter` for `RunCode(...)`, and use agent-specific templates such as `claude` or `codex` when you need those CLIs preinstalled.
- Use longer SDK HTTP timeouts for `waitReady` flows and image builds.
- Derive runtime access from sandbox responses instead of storing runtime endpoints or tokens in config.

## Compatibility

- Go: see `go.mod` for the supported toolchain version used by this SDK.
- API model: this SDK targets the unified SeaCloudAI sandbox gateway and keeps public template APIs limited to user-facing fields.
- Stability: operator/admin routes may exist on the gateway, but they are not part of the public SDK workflow described in this README.
- Retry model: treat create/delete/build operations as remote control-plane actions; add idempotency and retry policy in your application layer according to your workload.
- Timeout semantics: sandbox lifecycle uses E2B-style `Timeout` seconds. Commands, PTY, git, and code execution helpers use `TimeoutMS` milliseconds. `core.WithTimeout(...)` controls the SDK HTTP client timeout.

## Additional Examples

### Control Plane

```go
package main

import (
	"context"
	"log"

	"github.com/SeaCloudAI/sandbox-go"
)

func main() {
	ready := true
	timeout := int64(1800)
	createdSandbox, err := sandbox.Create(context.Background(), "base", &sandbox.CreateOptions{
		WaitReady: &ready,
		Timeout:   &timeout,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer createdSandbox.Delete(context.Background())

	log.Printf("sandbox=%s envd=%v", createdSandbox.SandboxID, createdSandbox.EnvdURL)
	if createdSandbox.EnvdURL != nil {
		runtime, err := createdSandbox.Runtime()
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("runtime=%s", runtime.BaseURL())
	}
}
```

### Bound Sandbox Workflow

```go
listed, err := sandbox.List(context.Background(), nil)
if err != nil {
	log.Fatal(err)
}

for _, sandbox := range listed {
	detail, err := sandbox.Reload(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("sandbox=%s status=%s", detail.SandboxID, detail.Status)
}
```

### Template Build

```go
template := sandbox.NewTemplate().
	FromImage("docker.io/library/alpine:3.20").
	RunCmd("echo hello-from-go >/tmp/hello.txt", nil).
	SetReadyCmd(sandbox.WaitForFile("/tmp/hello.txt"))

built, err := sandbox.BuildTemplate(context.Background(), template, "demo:v1", nil)
if err != nil {
	log.Fatal(err)
}

log.Printf("template=%s build=%s status=%s", built.TemplateID, built.BuildID, built.Status)
```

High-level template helpers currently include:

- lifecycle and status: `sandbox.BuildTemplate`, `sandbox.BuildTemplateInBackground`, `sandbox.TemplateExists`, `sandbox.GetTemplateBuildStatus`, `sandbox.ListTemplates`, `sandbox.GetTemplate`, `sandbox.DeleteTemplate`
  tag helpers: `sandbox.AssignTemplateTags`, `sandbox.GetTemplateTags`, `sandbox.RemoveTemplateTags`
- serialization: `sandbox.TemplateToJSON`, `sandbox.TemplateToDockerfile`
- base images and registries: `FromDockerfile`, `FromBaseImage`, `FromNodeImage`, `FromPythonImage`, `FromBunImage`, `FromUbuntuImage`, `FromDebianImage`, `FromAWSRegistry`, `FromGCPRegistry`
- build-step helpers: `Copy`, `CopyItems`, `SkipCache`, `AptInstall`, `GitClone`, `MakeDir`, `MakeSymlink`, `NpmInstall`, `PipInstall`, `BunInstall`, `Remove`, `Rename`, `RunCmd`, `RunCmds`
- execution and config helpers: `SetEnvs`, `SetWorkdir`, `SetUser`, `SetStartCmd`, `SetReadyCmd`, `FilesHash`
- supported local copy options: `ForceUpload`, `Mode`, `ResolveSymlinks`, `User`
- supported command and path options: `RunCmd(..., &sandbox.TemplateCommandOptions{User: ...})`, `GitClone(..., &sandbox.TemplateGitCloneOptions{User: ...})`, `MakeDir(..., &sandbox.TemplateMakeDirOptions{User: ...})`, `MakeSymlink(..., &sandbox.TemplateMakeSymlinkOptions{User: ...})`, `Remove(..., &sandbox.TemplateRemoveOptions{...User: ...})`, `Rename(..., &sandbox.TemplateRenameOptions{...User: ...})`
- intentionally not exposed yet: MCP server helpers and devcontainer helpers

### Low-Level Build Plane Through `build.NewService(...)`

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/SeaCloudAI/sandbox-go/build"
)

func main() {
	service, err := build.NewService(
		os.Getenv("SEACLOUD_BASE_URL"),
		os.Getenv("SEACLOUD_API_KEY"),
	)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := service.CreateTemplate(context.Background(), &build.TemplateCreateRequest{
		Name: "demo",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer service.DeleteTemplate(context.Background(), resp.TemplateID)

	buildID := "build-demo"
	_, err = service.CreateBuild(
		context.Background(),
		resp.TemplateID,
		buildID,
		build.NewTemplateBuildBuilder().
			FromImage("docker.io/library/alpine:3.20").
			Run("echo hello-from-go >/tmp/hello.txt", nil).
			Request(),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("template=%s build=%s", resp.TemplateID, buildID)
}
```

### Runtime Modules

```go
package main

import (
	"context"
	"log"

	"github.com/SeaCloudAI/sandbox-go"
)

func main() {
	ready := true
	createdSandbox, err := sandbox.Create(context.Background(), "base", &sandbox.CreateOptions{
		WaitReady: &ready,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer createdSandbox.Delete(context.Background())

	files, err := createdSandbox.Files()
	if err != nil {
		log.Fatal(err)
	}
	if _, err := files.Write(context.Background(), "/root/workspace/hello.txt", []byte("hello from go")); err != nil {
		log.Fatal(err)
	}

	bodyValue, err := files.Read(context.Background(), "/root/workspace/hello.txt", nil)
	if err != nil {
		log.Fatal(err)
	}
	body, _ := bodyValue.(string)
	log.Printf("%s", body)
}
```

### Code Interpreter

Use a template that actually includes the code-interpreter environment here. In SeaCloudAI environments, prefer an official `code-interpreter` template or a concrete `tpl-code-interpreter-...` template ID. Do not use `base` for this example.

```go
execution, err := createdSandbox.RunCode(context.Background(), `
import pandas as pd

df = pd.DataFrame([{"name": "Ada", "score": 99}])
display(df)
99
`, &sandbox.RunCodeOptions{
	OnStdout: func(chunk sandbox.CodeOutputChunk) {
		log.Printf("stdout: %s", chunk.Line)
	},
	OnStderr: func(chunk sandbox.CodeOutputChunk) {
		log.Printf("stderr: %s", chunk.Line)
	},
	OnResult: func(result sandbox.CodeExecutionResult) {
		log.Printf("result: %+v", result)
	},
})
if err != nil {
	log.Fatal(err)
}
log.Printf("text=%s", execution.Text())
```

For Python, repeated `RunCode(...)` calls reuse the sandbox's default code context. You can create additional Python contexts with `CreateCodeContext(...)` when you need isolated state. For other languages, `CreateCodeContext(...)` returns a reusable execution profile that supplies default `Language`, `CWD`, and `TimeoutMS` values, but each run still executes in a fresh one-shot process.

Bound sandbox helpers currently include:

- lifecycle: `Reload`, `Connect`, `Resume`, `GetInfo`, `GetFullInfo`, `Logs`, `Pause`, `Kill`, `Delete`, `Refresh`, `SetTimeout`, `IsRunning`
  `SandboxInfo`, `Sandbox`, and `SandboxDetail` expose `TrafficAccessToken` as an E2B-style alias of the runtime access token returned by the gateway.
- runtime conveniences: `GetMetrics`, `GetHost`, `DownloadURL`, `UploadURL`, `Proxy`
- code interpreter: `RunCode`, `CreateCodeContext`, `ListCodeContexts`, `RestartCodeContext`, `RemoveCodeContext`
- commands module: `Run`, `Exec`, `Start`, `Wait`, `List`, `Connect`, `Kill`, `SendStdin`
  `Run` / `Exec` / `Start` accept `TimeoutMS`, `Stdin`, `StdinOpen`, `OnStdout`, `OnStderr`, and `User`; callbacks and open-stdin mode use the runtime streaming protocol.
  `Connect` accepts `CommandConnectOptions{OnStdout, OnStderr}` for attaching output callbacks to an existing process stream. `CommandHandle` exposes both `SendStdin(...)` and the E2B-style `SendInput(...)` alias.
- filesystem module: `Exists`, `GetInfo`, `List`, `MakeDir`, `Read`, `Write`, `WriteFiles`, `Remove`, `Rename`, `WatchDir`
  `GetInfo()` / `List()` / `Rename()` return normalized high-level entries with `Type: sandbox.FileType` and `ModifiedTime *time.Time`.
  `Write()` / `WriteFiles()` return E2B-style write info with `Name`, `Path`, and `Type`.
  `WithOptions` variants accept `User`; `MakeDir` returns `false` when the path already exists. `WatchDirWithOptions` supports `User`, `TimeoutMS`, and `OnExit`.
- git module: `Clone`, `Pull`, `Checkout`, `Status`
- pty module: `Create`, `Connect`, `Kill`, `SendStdin`, `SendInput`, `Resize`
  `Pty.Connect` accepts `PtyConnectOptions{OnStdout, OnStderr}` when reconnecting to a PTY.

## Recommended Usage

For most integrations, prefer the env-first high-level flow:

- set `SEACLOUD_BASE_URL` and `SEACLOUD_API_KEY`
- create sandboxes with `sandbox.Create(...)`
- continue through `Commands()/Files()/Git()/Pty()`
- build templates with `sandbox.BuildTemplate(...)` and `sandbox.BuildTemplateInBackground(...)`
- switch to `control.NewService(...)` or `build.NewService(...)` only when you need low-level request/response control

Low-level methods remain available when you need tighter request control:

- use `control.NewService(...)` for `CreateSandbox`, `ListSandboxes`, `GetSandbox`, `ConnectSandbox`, and related control APIs
- continue from the returned sandbox object with `Reload()`, `Connect()`, `Resume()`, `GetInfo()`, `GetFullInfo()`, `GetMetrics()`, `GetHost()`, `Logs()`, `Pause()`, `Refresh()`, `SetTimeout()`, `Kill()`, `Delete()`, and `IsRunning()`
- only switch to runtime with `Runtime()` when you need file/process/stream operations
- use `sandbox.BuildTemplate(...)`, `sandbox.BuildTemplateInBackground(...)`, `sandbox.TemplateExists(...)`, `sandbox.GetTemplateBuildStatus(...)`, `sandbox.ListTemplates(...)`, `sandbox.GetTemplate(...)`, and `sandbox.DeleteTemplate(...)` for the preferred template workflow
- use `build.NewService(...)` only for raw template/build workflows
- use `build.NewTemplateBuildBuilder()` when you want a small fluent helper that expands into `BuildRequest`

Low-level domain packages remain available when you need direct request/response models or tighter transport control.

## API Surface

### Control Plane APIs

Preferred high-level lifecycle path:

- `sandbox.Create`, `sandbox.Connect`, `sandbox.List`, `sandbox.Get`
- follow-up control actions from the returned object: `Reload()`, `Connect()`, `Resume()`, `GetInfo()`, `GetFullInfo()`, `GetMetrics()`, `GetHost()`, `Logs()`, `Pause()`, `Refresh()`, `SetTimeout()`, `Kill()`, `Delete()`, `IsRunning()`
- runtime actions from objects that include `EnvdURL`: `Runtime()`

Low-level control APIs live in `control.Service`:

- system: `Metrics`, `Shutdown`
- sandboxes: `CreateSandbox`, `ListSandboxes`, `GetSandbox`, `DeleteSandbox`
- sandbox operations: `GetSandboxMetrics`, `ListSandboxMetrics`, `GetSandboxLogs`, `PauseSandbox`, `ConnectSandbox`, `SetSandboxTimeout`, `RefreshSandbox`, `SendHeartbeat`

### Monitoring And Metrics

The SDK exposes two different metrics surfaces:

- **Control-plane sandbox metrics** use Atlas through the gateway. Prefer these for dashboards and fleet monitoring because they can include Grafana/Kata enriched fields such as load average, CPU breakdown, memory pressure, disk I/O, network throughput, and task counts.
- **Runtime metrics** call the sandbox runtime `/metrics` endpoint through `EnvdURL`. Use these when you are already connected to one runtime and only need the direct in-sandbox snapshot. The runtime payload includes CPU, memory, disk, and cumulative network byte counters; derived rates and enriched fields are available from the control-plane metrics surface.

Control-plane metrics:

```go
ctx := context.Background()

service, err := control.NewService(
	os.Getenv("SEACLOUD_BASE_URL"),
	os.Getenv("SEACLOUD_API_KEY"),
)
if err != nil {
	log.Fatal(err)
}

single, err := service.GetSandboxMetrics(ctx, "sandbox-abc")
if err != nil {
	log.Fatal(err)
}
log.Printf("cpu=%.2f load1=%v memory=%v", single.CPUUsedPct, single.Load1, single.MemoryUsagePercent)
log.Printf("network sent=%v disk write=%v", single.NetworkSentBytesPerSecond, single.DiskWriteBytesPerSecond)

batch, err := service.ListSandboxMetrics(ctx, &control.SandboxMetricsParams{
	SandboxIDs: []string{"sandbox-abc", "sandbox-def"},
	Limit:      2,
})
if err != nil {
	log.Fatal(err)
}
for _, item := range batch.Items {
	log.Print(item.SandboxID)
}
```

Control-plane snapshot fields include:

- identity and status: `SandboxID`, `CollectedAt`, `Error`
- CPU: `CPUCount`, `CPUUsedPct`, `Load1`, `Load5`, `Load15`, `CPUUserRate`, `CPUSystemRate`, `CPUIOWaitRate`, `CPUStealRate`
- memory: `MemTotal`, `MemUsed`, `MemTotalMiB`, `MemUsedMiB`, `MemCache`, `MemoryAvailableBytes`, `MemoryUsagePercent`, swap fields
- disk: `DiskUsed`, `DiskTotal`, `DiskReadOpsPerSecond`, `DiskWriteOpsPerSecond`, `DiskReadBytesPerSecond`, `DiskWriteBytesPerSecond`
- network: `NetRxBytes`, `NetTxBytes`, `NetworkRecvBytesPerSecond`, `NetworkSentBytesPerSecond`, packet/error/drop rates
- tasks: `TaskCurrent`, `TaskMax`

Runtime metrics:

```go
runtimeMetrics, err := sbx.GetMetrics(ctx)
if err != nil {
	log.Fatal(err)
}
log.Printf("cpu=%.2f", runtimeMetrics.CPUUsedPct)
log.Printf("mem=%d/%d MiB", runtimeMetrics.MemUsedMiB, runtimeMetrics.MemTotalMiB)
log.Printf("disk=%d/%d", runtimeMetrics.DiskUsed, runtimeMetrics.DiskTotal)
log.Printf("net rx=%d tx=%d", runtimeMetrics.NetRxBytes, runtimeMetrics.NetTxBytes)
```

Use `service.Metrics(ctx)` or `buildService.Metrics(ctx)` only when you need the Prometheus text output for the gateway services themselves. Those service metrics are not per-sandbox runtime metrics.

### Operator APIs

`control.Service` also includes operator-oriented methods such as `GetPoolStatus`, `StartRollingUpdate`, `GetRollingUpdateStatus`, and `CancelRollingUpdate`.

These routes are intended for platform operators, not normal application workloads. Keep them out of business-facing integrations unless you are explicitly building operational tooling.

### Template Facade

Preferred template path:

- `sandbox.NewTemplate()` for build DSL
- `sandbox.BuildTemplate(...)` for create + build + optional polling
- `sandbox.BuildTemplateInBackground(...)` for fire-and-poll-later workflows
- `sandbox.ListTemplates(...)`, `sandbox.GetTemplate(...)`, `sandbox.DeleteTemplate(...)`, `sandbox.TemplateExists(...)`, `sandbox.GetTemplateBuildStatus(...)` for lifecycle and status
- `sandbox.TemplateToJSON(...)`, `sandbox.TemplateToDockerfile(...)` for export helpers

Template builder conveniences include:

- base images and registries: `FromDockerfile` (returns `(*Template, error)`), `FromBaseImage`, `FromNodeImage`, `FromPythonImage`, `FromBunImage`, `FromUbuntuImage`, `FromDebianImage`, `FromAWSRegistry`, `FromGCPRegistry`
- file and command helpers: `Copy`, `CopyItems`, `SkipCache`, `AptInstall`, `GitClone`, `MakeDir`, `MakeSymlink`, `NpmInstall`, `PipInstall`, `BunInstall`, `Remove`, `Rename`, `RunCmd`, `RunCmds`
- execution and config helpers: `SetEnvs`, `SetWorkdir`, `SetUser`, `SetStartCmd`, `SetReadyCmd`, `FilesHash`
- supported local copy options: `ForceUpload`, `Mode`, `ResolveSymlinks`, `User`
- supported command and path options: `RunCmd(..., &sandbox.TemplateCommandOptions{User: ...})`, `GitClone(..., &sandbox.TemplateGitCloneOptions{User: ...})`, `MakeDir(..., &sandbox.TemplateMakeDirOptions{User: ...})`, `MakeSymlink(..., &sandbox.TemplateMakeSymlinkOptions{User: ...})`, `Remove(..., &sandbox.TemplateRemoveOptions{...User: ...})`, `Rename(..., &sandbox.TemplateRenameOptions{...User: ...})`
- intentionally not exposed yet: MCP server helpers and devcontainer helpers

### Build Plane Through `build.Service`

Low-level `build.Service` exposes:

- system: `Metrics`
- templates: `CreateTemplate`, `ListTemplates`, `GetTemplateByAlias`, `ResolveTemplateRef`, `GetTemplate`, `UpdateTemplate`, `DeleteTemplate`
- builds: `CreateBuild`, `GetBuildFile`, `RollbackTemplate`, `ListBuilds`, `GetBuild`, `GetBuildStatus`, `GetBuildLogs`
- tags: `AssignTemplateTags`, `DeleteTemplateTags`, `ListTemplateTags`

Build logs are served by the platform log API. `GetBuildLogs` returns structured log entries without exposing the underlying log storage.

### Observability Summary

Use `sandbox.GetObservabilitySummary(ctx)` to get one user/Project-level view of sandbox usage, template/build usage, and the public diagnostic endpoints:

```go
summary, err := sandbox.GetObservabilitySummary(ctx)
if err != nil {
	log.Fatal(err)
}
log.Printf("status=%s usage=%+v", summary.Status, summary.Usage)
```

The summary intentionally returns public product fields only. Use the endpoint hints from `summary.Endpoints` for sandbox logs, build logs, and full usage-limit checks.

The public template contract is split into three layers: E2B create fields (`Name`, `Tags`, `CPUCount`, `MemoryMB`), Atlas extension fields under `Extensions` (`BaseTemplateID`, `Visibility`, `Envs`, `VolumeMounts`, `Workdir`), E2B update field `Public`, and build-only fields on `CreateBuild` (`FromImage`, `FromTemplate`, `Steps`, `Tags`, `StartCmd`, `ReadyCmd`, registry credentials, `Steps[].FilesHash`).
Template tags are version pointers to build artifacts. Build requests without explicit tags use `default`; `sandbox.AssignTemplateTags(ctx, "template:v1", []string{"stable"})` moves `stable` to the build behind `v1`, and sandboxes can reference `template:stable` or `template:buildID`.
Each mount declares its own storage through `VolumeMounts[i].StorageType` plus the matching storage fields such as `NfsHostPath`, `StorageClass`/`StorageSizeGB`, `PersistentVolumeClaim`, or `ObjectBucket`. `Workdir` sets the sandbox default working directory and file API root; it does not create a mount by itself.
Runtime behavior defaults from the image source: templates inheriting SeaCloud base/runtime templates keep the managed runtime, while direct external images run as plain business containers. `StartCmd` and `ReadyCmd` only provide startup and readiness commands on top of that default.
Public create calls reject unsupported top-level write fields such as `Alias` and `Public`; public update calls only accept `Public`.

New custom templates default to `Type: "custom"`, `Version: "v0.1.0"`, `CPUCount: 1`, `MemoryMB: 512`, `TTLSeconds: 300`, and resource limit ratios of `1.0`. Server-generated template IDs use `tpl-{type}-{16 lowercase hex}` and server-generated initial build IDs use `build-{16 lowercase hex}`. Client-supplied build IDs passed to `CreateBuild(ctx, templateID, buildID, ...)` must be lowercase DNS labels up to 63 characters; the SDK recommends the `build-` prefix.

`CreateTemplate`, `ListTemplates`, and `GetTemplate` responses include `Type` and `Version` when the platform returns them. Treat `Type` as the stable template family and `Version` as that family's version marker.

For Go callers, the public write path and template read path now use different extension models on purpose:

- `TemplateCreateRequest` uses `PublicTemplateExtensions`; `TemplateUpdateRequest` uses `Public`
- `ListedTemplate` / `TemplateResponse` follow the E2B response shape; Atlas platform internals are not returned on public reads

This matches the current public builder API contract: request fields are intentionally narrower than response fields.

`CreateTemplate` rejects `visibility=official` on public routes, including `Extensions.Visibility == "official"`.

`CreateBuild` now follows the E2B wire contract directly: COPY contexts are passed through `Steps[].FilesHash`, and the SDK returns the raw `202 {}` trigger response without adding helper fields.

`FromImage` switches the template to an already-built image and does not start a new image build by itself. `FromTemplate` resolves a ready template image and uses it as the build base for supported E2B steps. `FromDockerfile` is a client-side convenience that parses a supported Dockerfile subset into `FromImage`, `Steps`, `StartCmd`, and `ReadyCmd`; it is not the platform admin raw-Dockerfile build route.

Build records can move through `uploaded`, `waiting`, `building`, `ready`, and `error`. `uploaded` means a referenced COPY context is still missing; upload it through the file handshake and call `CreateBuild` again with the same `buildID`.

`GetTemplateByAlias` is a pure alias lookup endpoint. It should only be used with an actual published alias value.

`ResolveTemplateRef` is the SeaCloud stable-ref lookup endpoint. It resolves a template by `templateID`, official template `type`, or visible alias.

### Runtime Helper

Low-level runtime objects returned by `createdSandbox.Runtime()`, `sandbox.RuntimeFromSandbox(...)`, `sandbox.RuntimeFromDetail(...)`, or `sandbox.NewRuntime(...)` expose:

- system: `Metrics`, `Envs`, `Configure`, `Ports`
- proxy and file transfer: `Proxy`, `Download`, `FilesContent`, `UploadBytes`, `UploadJSON`, `UploadMultipart`, `WriteBatch`, `ComposeFiles`, `ReadFile`, `WriteFile`
- filesystem RPC: `ListDir`, `Stat`, `MakeDir`, `Remove`, `Move`, `Edit`
- watchers: `WatchDir`, `CreateWatcher`, `GetWatcherEvents`, `RemoveWatcher`
- process RPC: `Start`, `Connect`, `ListProcesses`, `SendInput`, `SendSignal`, `CloseStdin`, `Update`, `StreamInput`, `GetResult`, `Run`

`cmd.RequestOptions` supports:

- `Username`: basic-auth username for envd operations that need it
- `Signature` and `SignatureExpiration`: signed file access parameters
- `Range`: partial download support
- `Headers`: custom header injection

Streaming APIs return `ProcessStream`, `FilesystemWatchStream`, and `ConnectFrame`.

## Resource Safety

- The quick starts are written for disposable resources and should be adapted before copy-pasting into production jobs.
- Prefer explicit cleanup with `defer createdSandbox.Delete(...)` and `defer service.DeleteTemplate(...)` when running probes, smoke tests, or CI.
- For long-lived workloads, move cleanup and timeout policy into your own lifecycle manager instead of relying on sample code defaults.

## Package Layout

- `github.com/SeaCloudAI/sandbox-go`: env-first high-level facade
- `github.com/SeaCloudAI/sandbox-go/control`: control-plane models and low-level APIs
- `github.com/SeaCloudAI/sandbox-go/build`: build-plane models and low-level APIs
- `github.com/SeaCloudAI/sandbox-go/cmd`: runtime models and low-level APIs
- `github.com/SeaCloudAI/sandbox-go/core`: shared transport and error primitives

## Notes

- The gateway entrypoint always needs an API key. `baseURL` can come from `SEACLOUD_BASE_URL`.
- Runtime access should be derived from sandbox response objects with `Runtime()`, `sandbox.RuntimeFromSandbox(...)`, or `sandbox.RuntimeFromDetail(...)`.
- `CreateSandbox` and `GetSandbox` responses include `EnvdURL` and `EnvdAccessToken` when the target sandbox supports CMD access.
- High-level sandbox objects expose `TrafficAccessToken` from `EnvdAccessToken` for E2B-style public traffic token access.
- Runtime file/process APIs require a template image that supports managed runtime access; if runtime APIs return `404`, verify the selected template supports CMD runtime routes.
- `waitReady=true` can take longer than the default HTTP timeout in production; pass `core.WithTimeout(...)` when creating long-wait clients.
- API errors expose `Kind` and `Retryable()` for retry logic and alert routing.
- High-level `Kill()` helpers send `SIGNAL_SIGKILL` and return `false` when the runtime reports a missing process through either `404` or `ESRCH`.
- PTY handles normalize reconnect output into `PTY` even when the runtime emits the bytes through `Stdout` / `Stderr`.
- Sandbox lifecycle timeout values are validated to `0..86400` seconds; refresh duration to `0..3600` seconds.
- Build request validation accepts E2B-style `COPY` / `ENV` / `RUN` / `WORKDIR` / `USER` steps, `force`, and structured `fromImageRegistry` credentials (`registry` / `aws` / `gcp`).
- Some gateways do not expose `/admin/*`; integration tests skip those cases on `404`.
- Some filesystem layouts reject watcher APIs entirely; the integration suite skips watcher coverage when the runtime reports that limitation.

## Security

- Do not commit `SEACLOUD_API_KEY`, `envdAccessToken`, or sandbox access tokens.
- Treat runtime tokens as sandbox-scoped secrets. Prefer `createdSandbox.Runtime()` or `sandbox.RuntimeFromSandbox(...)` so response-scoped runtime access is not copied into configuration.
- Do not log raw API keys or runtime tokens. SDK errors may include response bodies, so avoid logging full error payloads in shared systems.

## Production Smoke

Use production smoke tests only with explicitly provided credentials and disposable sandboxes:

```bash
SANDBOX_RUN_INTEGRATION=1 \
SANDBOX_TEST_BASE_URL="${SEACLOUD_BASE_URL}" \
SANDBOX_TEST_API_KEY="${SEACLOUD_API_KEY}" \
SANDBOX_TEST_TEMPLATE_ID=tpl-base-dc11799b9f9f4f9e \
go test ./tests/... -run Integration -count=1 -v
```

`tpl-base-dc11799b9f9f4f9e` is a known-good SeaCloudAI runtime template for validating CMD routes such as `ListDir`, `ReadFile`, `WriteFile`, and `Run`.
You can also run the same disposable smoke flow from GitHub Actions with `.github/workflows/integration-smoke.yml` after setting the `SANDBOX_TEST_API_KEY` repository secret.

## Integration Tests

```bash
SANDBOX_RUN_INTEGRATION=1 \
SANDBOX_TEST_BASE_URL="${SEACLOUD_BASE_URL}" \
SANDBOX_TEST_API_KEY="${SEACLOUD_API_KEY}" \
SANDBOX_TEST_TEMPLATE_ID=... \
go test ./tests/... -count=1 -v
```

Use a runtime-enabled template for CMD integration coverage. For SeaCloudAI production smoke tests, `tpl-base-dc11799b9f9f4f9e` is a known-good runtime template.
The same smoke suite is available as a manual GitHub Actions dispatch in `.github/workflows/integration-smoke.yml`.

## Release

- See `CHANGELOG.md` for release notes.
- See `RELEASE_CHECKLIST.md` before tagging or publishing a new version.
