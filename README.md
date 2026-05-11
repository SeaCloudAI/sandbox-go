# Sandbox Go SDK

Go SDK for Sandbox control-plane, build-plane, and nano-executor CMD APIs.

## Install

```bash
go get github.com/SeaCloudAI/sandbox-go
```

## Entrypoints

Preferred public API:

- preferred sandbox entrypoint: package-level helpers such as `sandbox.Create(...)`, `sandbox.Connect(...)`, and `sandbox.List(...)`, which read gateway config from env by default
- optional advanced client: `sandbox.NewClientFromEnv(...)` for custom control/build workflows
- sandbox runtime modules from the returned object: `created.Commands()`, `created.Files()`, `created.Git()`, `created.Pty()`
- preferred template entrypoint: package-level helpers such as `sandbox.BuildTemplate(...)`, `sandbox.BuildTemplateInBackground(...)`, `sandbox.ListTemplates(...)`, and `sandbox.GetTemplate(...)`
- low-level build plane via `client.Build`
- raw runtime helper: `createdSandbox.Runtime()` or `client.RuntimeFromSandbox(createdSandbox)`

`control` and `build` both talk to the same gateway. Runtime access is derived from sandbox create/detail/connect responses; callers should not hardcode runtime endpoints or tokens. Project-scoped deployments can inject gateway routing context with `core.WithProjectID(...)`.

## E2B Alignment

- Supported alignment target: sandbox lifecycle, files, commands, git, PTY, and the high-level template DSL are designed to follow the same public workflow as `e2b-docs/sdk`.
- Code interpreter alignment: `sandbox.RunCode(...)` is available for `python`, `javascript`, `typescript`, `bash`, `r`, and `java`. Python results support `display(...)`, last-expression capture, tables, Matplotlib PNG/chart payloads, a persistent default execution context, and stateful `CreateCodeContext/ListCodeContexts/RestartCodeContext/RemoveCodeContext` helpers. Non-Python contexts use the same API surface but currently behave as stateless execution profiles.
- Known unsupported area: snapshot APIs are not exposed because the underlying platform does not support them yet.
- Known partial area: only Python contexts are stateful. Non-Python contexts still execute in isolated one-shot processes.
- Go-specific note: the SDK aims for semantic equivalence rather than identical hand feel. Method names, `context.Context`, and `(..., error)` returns stay Go-native on purpose.
- Runtime compatibility note: the SDK normalizes a few runtime-specific quirks so the high-level behavior stays E2B-like, such as missing-process `Kill()` results and PTY reconnect output framing.

## Environment

Use environment variables for gateway configuration in all examples and quick starts:

- `E2B_DOMAIN`: preferred gateway entrypoint
- `E2B_API_KEY`: preferred API key
- `SEACLOUD_PROJECT_ID`: optional project routing key for project-scoped gateways

Set them once in your shell:

```bash
export E2B_DOMAIN="https://sandbox-gateway.cloud.seaart.ai"
export E2B_API_KEY="..."
export SEACLOUD_PROJECT_ID="project-..."
```

Default production gateway:

```text
https://sandbox-gateway.cloud.seaart.ai
```

High-level create helpers default to the official `base` template when you do not pass a template explicitly. For production integrations, prefer passing a concrete template ID such as `tpl-...` or a stable official template type such as `base`, `code-interpreter`, `claude`, or `codex` when your environment publishes those official templates.

## Production Readiness

- Package-level helpers are fine for simple env-first flows. For repeated low-level or mixed control/build workflows, initialize exactly one root client and reuse it.
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
- Timeout semantics: public sandbox, command, PTY, git, and code-execution `Timeout` values are in seconds. `core.WithTimeout(...)` controls the SDK HTTP client timeout.

## Quick Start

### Control Plane

```go
package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/SeaCloudAI/sandbox-go"
	"github.com/SeaCloudAI/sandbox-go/core"
)

func main() {
	ready := true
	timeout := int32(1800)
	createdSandbox, err := sandbox.Create(context.Background(), "base", &sandbox.CreateOptions{
		WaitReady: &ready,
		Timeout:   &timeout,
	}, core.WithProjectID(os.Getenv("SEACLOUD_PROJECT_ID")), core.WithTimeout(180*time.Second))
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
- serialization: `sandbox.TemplateToJSON`, `sandbox.TemplateToDockerfile`
- base images and registries: `FromDockerfile`, `FromBaseImage`, `FromNodeImage`, `FromPythonImage`, `FromBunImage`, `FromUbuntuImage`, `FromDebianImage`, `FromAWSRegistry`, `FromGCPRegistry`
- build-step helpers: `Copy`, `CopyItems`, `SkipCache`, `AptInstall`, `GitClone`, `MakeDir`, `MakeSymlink`, `NpmInstall`, `PipInstall`, `BunInstall`, `Remove`, `Rename`, `RunCmd`, `RunCmds`
- execution and config helpers: `SetEnvs`, `SetWorkdir`, `SetUser`, `SetStartCmd`, `SetReadyCmd`, `FilesHash`
- supported local copy options: `ForceUpload`, `Mode`, `ResolveSymlinks`, `User`
- supported command and path options: `RunCmd(..., &sandbox.TemplateCommandOptions{User: ...})`, `GitClone(..., &sandbox.TemplateGitCloneOptions{User: ...})`, `MakeDir(..., &sandbox.TemplateMakeDirOptions{User: ...})`, `MakeSymlink(..., &sandbox.TemplateMakeSymlinkOptions{User: ...})`, `Remove(..., &sandbox.TemplateRemoveOptions{...User: ...})`, `Rename(..., &sandbox.TemplateRenameOptions{...User: ...})`
- intentionally not exposed yet: MCP server helpers and devcontainer helpers

### Raw Build Plane Through Root Client

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/SeaCloudAI/sandbox-go"
	"github.com/SeaCloudAI/sandbox-go/build"
)

func main() {
	client, err := sandbox.NewClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.Build.CreateTemplate(context.Background(), &build.TemplateCreateRequest{
		Name: "demo",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Build.DeleteTemplate(context.Background(), resp.TemplateID)

	buildID := "build-demo"
	_, err = client.Build.CreateBuild(
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
	"os"

	"github.com/SeaCloudAI/sandbox-go"
)

func main() {
	client, err := sandbox.NewClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	ready := true
	createdSandbox, err := client.Create(context.Background(), "base", &sandbox.CreateOptions{
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

	body, err := files.Read(context.Background(), "/root/workspace/hello.txt")
	if err != nil {
		log.Fatal(err)
	}
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

For Python, repeated `RunCode(...)` calls reuse the sandbox's default code context. You can create additional Python contexts with `CreateCodeContext(...)` when you need isolated state. For other languages, `CreateCodeContext(...)` returns a reusable execution profile that supplies default `Language`, `CWD`, and `Timeout` values, but each run still executes in a fresh one-shot process.

Bound sandbox helpers currently include:

- lifecycle: `Reload`, `Connect`, `Resume`, `GetInfo`, `Logs`, `Pause`, `Kill`, `Delete`, `Refresh`, `SetTimeout`, `IsRunning`
- runtime conveniences: `GetMetrics`, `GetHost`, `Proxy`
- code interpreter: `RunCode`, `CreateCodeContext`, `ListCodeContexts`, `RestartCodeContext`, `RemoveCodeContext`
- commands module: `Run`, `Exec`, `Start`, `Wait`, `List`, `Connect`, `Kill`, `SendStdin`
- filesystem module: `Exists`, `GetInfo`, `List`, `MakeDir`, `Read`, `Write`, `WriteFiles`, `Remove`, `Rename`, `WatchDir`
- git module: `Clone`, `Pull`, `Checkout`, `Status`
- pty module: `Create`, `Connect`, `Kill`, `SendStdin`, `Resize`

## Recommended Usage

For most integrations, prefer the env-first high-level flow:

- set `E2B_DOMAIN`, `E2B_API_KEY`, and optionally `SEACLOUD_PROJECT_ID`
- create sandboxes with `sandbox.Create(...)`
- continue through `Commands()/Files()/Git()/Pty()`
- build templates with `sandbox.BuildTemplate(...)` and `sandbox.BuildTemplateInBackground(...)`
- use `sandbox.NewClientFromEnv(...)` only when you need low-level metrics, raw build-plane access, or explicit control/build orchestration beyond the high-level facade

Low-level methods remain available when you need tighter request control:

- use `CreateSandbox`, `ListSandboxes`, `GetSandbox`, `ConnectSandbox`
- continue from the returned sandbox object with `Reload()`, `Connect()`, `Resume()`, `GetInfo()`, `GetMetrics()`, `GetHost()`, `Logs()`, `Pause()`, `Refresh()`, `SetTimeout()`, `Kill()`, `Delete()`, and `IsRunning()`
- only switch to runtime with `Runtime()` when you need file/process/stream operations
- use `sandbox.BuildTemplate(...)`, `sandbox.BuildTemplateInBackground(...)`, `sandbox.TemplateExists(...)`, `sandbox.GetTemplateBuildStatus(...)`, `sandbox.ListTemplates(...)`, `sandbox.GetTemplate(...)`, and `sandbox.DeleteTemplate(...)` for the preferred template workflow
- use `client.Build` only for raw template/build workflows
- use `build.NewTemplateBuildBuilder()` when you want a small fluent helper that expands into `BuildRequest`

Low-level domain packages remain available when you need direct request/response models or tighter transport control.

## API Surface

### Control Plane APIs

`sandbox.Client` exposes control-plane methods directly:

- system: `Metrics`, `Shutdown`
- sandboxes: `CreateSandbox`, `ListSandboxes`, `GetSandbox`, `DeleteSandbox`
- sandbox operations: `GetSandboxLogs`, `PauseSandbox`, `ConnectSandbox`, `SetSandboxTimeout`, `RefreshSandbox`, `SendHeartbeat`

Recommended root-client path:

- high-level lifecycle: `Create`, `Connect`, `List`, `Get`
- template helpers: `BuildTemplate`, `BuildTemplateInBackground`, `TemplateExists`, `GetTemplateBuildStatus`, `ListTemplates`, `GetTemplate`, `DeleteTemplate`
- low-level lifecycle: `CreateSandbox`, `ListSandboxes`, `GetSandbox`, `ConnectSandbox`
- follow-up control actions from the returned object: `Reload()`, `Connect()`, `Resume()`, `GetInfo()`, `GetMetrics()`, `GetHost()`, `Logs()`, `Pause()`, `Refresh()`, `SetTimeout()`, `Kill()`, `Delete()`, `IsRunning()`
- runtime actions from objects that include `EnvdURL`: `Runtime()`

Low-level direct methods like `DeleteSandbox` and `GetSandboxLogs` remain available on the root client when you want explicit control-plane requests.

### Operator APIs

The root client also includes operator-oriented methods such as `GetPoolStatus`, `StartRollingUpdate`, `GetRollingUpdateStatus`, and `CancelRollingUpdate`.

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

### Build Plane Through `client.Build`

Low-level `client.Build` exposes:

- system: `Metrics`
- direct build: `DirectBuild`
- templates: `CreateTemplate`, `ListTemplates`, `GetTemplateByAlias`, `ResolveTemplateRef`, `GetTemplate`, `UpdateTemplate`, `DeleteTemplate`
- builds: `CreateBuild`, `GetBuildFile`, `RollbackTemplate`, `ListBuilds`, `GetBuild`, `GetBuildStatus`, `GetBuildLogs`

The public template contract is split into three layers: top-level create fields (`Name`, `Tags`, `CPUCount`, `MemoryMB`), template extensions under `Extensions` (`BaseTemplateID`, `Visibility`, `Envs`, `StorageType`, `StorageSizeGB`, `VolumeMounts`), and build-only fields on `CreateBuild` (`FromImage`, `FromTemplate`, `Steps`, `StartCmd`, `ReadyCmd`, registry credentials, `FilesHash`).
When `Extensions.StorageType="nfs"`, the public API still does not expose `nfsHostPath`; each `VolumeMounts[i].Name` is treated as the per-sandbox NFS subdirectory name under the inherited base template's NFS root, and `VolumeMounts[i].Path` is the container mount path. A mount named `workspace` becomes the primary workspace path.
Runtime behavior defaults from the image source: templates inheriting SeaCloud base/runtime templates keep the managed runtime, while direct external images run as plain business containers. `StartCmd` and `ReadyCmd` only provide startup and readiness commands on top of that default.
Public create/update calls reject legacy top-level write fields such as `Alias`, `TeamID`, and `Public`.

For Go callers, the public write path and template read path now use different extension models on purpose:

- `TemplateCreateRequest` / `TemplateUpdateRequest` use `PublicTemplateExtensions`
- `ListedTemplate` / `TemplateResponse` keep the fuller `TemplateExtensions` shape returned by the service

This matches the current public builder API contract: request fields are intentionally narrower than response fields.

`CreateTemplate` and `UpdateTemplate` reject `visibility=official` on public routes, including `Extensions.Visibility == "official"`.

`CreateBuild` now follows the wire contract directly: callers pass top-level `FilesHash` when needed, and the SDK returns the raw `202 {}` trigger response without adding helper fields.

`GetTemplateByAlias` is a pure alias lookup endpoint. It should only be used with an actual published alias value.

`ResolveTemplateRef` is the SeaCloud stable-ref lookup endpoint. It resolves a template by `templateID`, official template `type`, or visible alias.

### Runtime Helper

Low-level runtime objects returned by `createdSandbox.Runtime()` or `client.RuntimeFromSandbox(...)` expose:

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
- Prefer explicit cleanup with `defer createdSandbox.Delete(...)` and `defer client.Build.DeleteTemplate(...)` when running probes, smoke tests, or CI.
- For long-lived workloads, move cleanup and timeout policy into your own lifecycle manager instead of relying on sample code defaults.

## Package Layout

- `github.com/SeaCloudAI/sandbox-go`: root client and recommended entrypoint
- `github.com/SeaCloudAI/sandbox-go/control`: control-plane models and low-level APIs
- `github.com/SeaCloudAI/sandbox-go/build`: build-plane models and low-level APIs
- `github.com/SeaCloudAI/sandbox-go/cmd`: runtime models and low-level APIs
- `github.com/SeaCloudAI/sandbox-go/core`: shared transport and error primitives

## Notes

- The gateway entrypoint always needs an API key. `baseURL` can come from `E2B_DOMAIN`.
- Project-scoped deployments can set `core.WithProjectID(...)`; the SDK sends `X-Project-ID` on control/build requests.
- Runtime access should be derived from sandbox response objects with `Runtime()` or `RuntimeFromSandbox(...)`.
- `CreateSandbox` and `GetSandbox` responses include `EnvdURL` and `EnvdAccessToken` when the target sandbox supports CMD access.
- Runtime file/process APIs require a template image that starts nano-executor and returns runtime access fields; if runtime APIs return `404`, verify the selected template supports CMD runtime routes.
- `waitReady=true` can take longer than the default HTTP timeout in production; pass `core.WithTimeout(...)` when creating long-wait clients.
- API errors expose `Kind` and `Retryable()` for retry logic and alert routing.
- High-level `Kill()` helpers send `SIGNAL_SIGKILL` and return `false` when the runtime reports a missing process through either `404` or `ESRCH`.
- PTY handles normalize reconnect output into `PTY` even when the runtime emits the bytes through `Stdout` / `Stderr`.
- Sandbox timeout values are validated to `0..86400`; refresh duration to `0..3600`.
- Build request validation accepts E2B-style `COPY` / `ENV` / `RUN` / `WORKDIR` / `USER` steps, `force`, and structured `fromImageRegistry` credentials (`registry` / `aws` / `gcp`).
- Some gateways do not expose `/admin/*` or `/build`; integration tests skip those cases on `404`.
- Some filesystem layouts reject watcher APIs entirely; the integration suite skips watcher coverage when the runtime reports that limitation.

## Security

- Do not commit `E2B_API_KEY`, `envdAccessToken`, or sandbox access tokens.
- Treat runtime tokens as sandbox-scoped secrets. Prefer `createdSandbox.Runtime()` or `client.RuntimeFromSandbox(...)` so response-scoped runtime access is not copied into configuration.
- Do not log raw API keys or runtime tokens. SDK errors may include response bodies, so avoid logging full error payloads in shared systems.
- Set `core.WithProjectID(...)` when your gateway requires explicit project routing. The SDK sends it as `X-Project-ID`.

## Production Smoke

Use production smoke tests only with explicitly provided credentials and disposable sandboxes:

```bash
SANDBOX_RUN_INTEGRATION=1 \
SANDBOX_TEST_BASE_URL="${E2B_DOMAIN}" \
SANDBOX_TEST_API_KEY="${E2B_API_KEY}" \
SANDBOX_TEST_TEMPLATE_ID=tpl-base-dc11799b9f9f4f9e \
go test ./tests/... -run Integration -count=1 -v
```

`tpl-base-dc11799b9f9f4f9e` is a known-good SeaCloudAI runtime template for validating CMD routes such as `ListDir`, `ReadFile`, `WriteFile`, and `Run`.
You can also run the same disposable smoke flow from GitHub Actions with `.github/workflows/integration-smoke.yml` after setting the `SANDBOX_TEST_API_KEY` repository secret.

## Integration Tests

```bash
SANDBOX_RUN_INTEGRATION=1 \
SANDBOX_TEST_BASE_URL="${E2B_DOMAIN}" \
SANDBOX_TEST_API_KEY="${E2B_API_KEY}" \
SANDBOX_TEST_TEMPLATE_ID=... \
go test ./tests/... -count=1 -v
```

Use a runtime-enabled template for CMD integration coverage. For SeaCloudAI production smoke tests, `tpl-base-dc11799b9f9f4f9e` is a known-good runtime template.
The same smoke suite is available as a manual GitHub Actions dispatch in `.github/workflows/integration-smoke.yml`.

## Release

- See `CHANGELOG.md` for release notes.
- See `RELEASE_CHECKLIST.md` before tagging or publishing a new version.
