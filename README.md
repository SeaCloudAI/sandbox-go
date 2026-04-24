# Sandbox Go SDK

Go SDK for Sandbox control-plane, build-plane, and nano-executor CMD APIs.

## Install

```bash
go get github.com/SeaCloudAI/sandbox-go
```

## Client Initialization

Recommended entrypoint:

- gateway client: `sandbox.NewClient(baseURL, apiKey)`
- build plane via root client: `client.Build`
- runtime helper: `createdSandbox.Runtime()` or `client.RuntimeFromSandbox(createdSandbox)`

`control` and `build` both talk to the same gateway and only need `apiKey`. Runtime access is derived from sandbox create/detail/connect responses; callers should not hardcode runtime endpoints or tokens.

## Environment

Use environment variables for gateway configuration in all examples and quick starts:

- `SEACLOUD_BASE_URL`: SeaCloudAI gateway entrypoint
- `SEACLOUD_API_KEY`: API key used for gateway routing and authentication
- `SEACLOUD_TEMPLATE_ID`: sandbox template identifier or official template type for your target environment

Set them once in your shell:

```bash
export SEACLOUD_BASE_URL="https://sandbox-gateway.cloud.seaart.ai"
export SEACLOUD_API_KEY="..."
export SEACLOUD_TEMPLATE_ID="tpl-..."
```

Default production gateway:

```text
https://sandbox-gateway.cloud.seaart.ai
```

Use `SEACLOUD_TEMPLATE_ID` for production integrations. It can be either a concrete template ID such as `tpl-...` or a stable official template type such as `base`, `claude`, or `codex` when your environment publishes those official templates.

## Production Readiness

- Initialize exactly one root client per process and reuse it.
- Treat every quick start as creating billable or quota-bound resources unless it explicitly cleans them up.
- Prefer explicit template references from configuration over hardcoded example values.
- In SeaCloudAI environments, prefer official template types such as `base`, `claude`, or `codex` when you want a stable platform-managed entrypoint.
- Use longer client timeouts for `waitReady` flows and image builds.
- Derive runtime access from sandbox responses instead of storing runtime endpoints or tokens in config.

## Compatibility

- Go: see `go.mod` for the supported toolchain version used by this SDK.
- API model: this SDK targets the unified SeaCloudAI sandbox gateway and keeps public template APIs limited to user-facing fields.
- Stability: operator/admin routes may exist on the gateway, but they are not part of the public SDK workflow described in this README.
- Retry model: treat create/delete/build operations as remote control-plane actions; add idempotency and retry policy in your application layer according to your workload.

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
	"github.com/SeaCloudAI/sandbox-go/control"
	"github.com/SeaCloudAI/sandbox-go/core"
)

func main() {
	client, err := sandbox.NewClient(
		os.Getenv("SEACLOUD_BASE_URL"),
		os.Getenv("SEACLOUD_API_KEY"),
		core.WithTimeout(180*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	ready := true
	timeout := int32(1800)
	createdSandbox, err := client.CreateSandbox(context.Background(), &control.NewSandboxRequest{
		TemplateID: os.Getenv("SEACLOUD_TEMPLATE_ID"),
		WaitReady:  &ready,
		Timeout:    &timeout,
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
listed, err := client.ListSandboxes(context.Background(), nil)
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

### Build Plane Through Root Client

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
	client, err := sandbox.NewClient(
		os.Getenv("SEACLOUD_BASE_URL"),
		os.Getenv("SEACLOUD_API_KEY"),
	)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.Build.CreateTemplate(context.Background(), &build.TemplateCreateRequest{
		Name:  "demo",
		Image: "docker.io/library/alpine:3.20",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Build.DeleteTemplate(context.Background(), resp.TemplateID)

	log.Printf("template=%s build=%s", resp.TemplateID, resp.BuildID)
}
```

### Runtime Helper

```go
package main

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/SeaCloudAI/sandbox-go"
	"github.com/SeaCloudAI/sandbox-go/cmd"
	"github.com/SeaCloudAI/sandbox-go/control"
)

func main() {
	client, err := sandbox.NewClient(
		os.Getenv("SEACLOUD_BASE_URL"),
		os.Getenv("SEACLOUD_API_KEY"),
	)
	if err != nil {
		log.Fatal(err)
	}

	ready := true
	createdSandbox, err := client.CreateSandbox(context.Background(), &control.NewSandboxRequest{
		TemplateID: os.Getenv("SANDBOX_EXAMPLE_TEMPLATE_ID"),
		WaitReady:  &ready,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer createdSandbox.Delete(context.Background())

	runtime, err := createdSandbox.Runtime()
	if err != nil {
		log.Fatal(err)
	}

	err = runtime.WriteFile(context.Background(), &cmd.UploadBytesRequest{
		Path: "/root/workspace/hello.txt",
		Data: []byte("hello from go"),
	}, nil)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := runtime.ReadFile(context.Background(), &cmd.FileRequest{
		Path: "/root/workspace/hello.txt",
	}, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%s", body)
}
```

## Recommended Usage

For most integrations, stay on the root client as long as possible:

- initialize once with `sandbox.NewClient(baseURL, apiKey)`
- use `CreateSandbox`, `ListSandboxes`, `GetSandbox`, `ConnectSandbox`
- continue from the returned sandbox object with `Reload()`, `Logs()`, `Pause()`, `Refresh()`, `SetTimeout()`, `Connect()`, `Delete()`
- only switch to runtime with `Runtime()` when you need file/process/stream operations
- use `client.Build` only for template/build workflows

Low-level domain packages remain available when you want direct stateless calls or need request/response models explicitly.

## API Surface

### Control Plane APIs

`sandbox.Client` exposes control-plane methods directly:

- system: `Metrics`, `Shutdown`
- sandboxes: `CreateSandbox`, `ListSandboxes`, `GetSandbox`, `DeleteSandbox`
- sandbox operations: `GetSandboxLogs`, `PauseSandbox`, `ConnectSandbox`, `SetSandboxTimeout`, `RefreshSandbox`, `SendHeartbeat`

Recommended root-client path:

- sandbox lifecycle: `CreateSandbox`, `ListSandboxes`, `GetSandbox`, `ConnectSandbox`
- follow-up control actions from the returned object: `Reload()`, `Logs()`, `Pause()`, `Refresh()`, `SetTimeout()`, `Connect()`, `Delete()`
- runtime actions from objects that include `EnvdURL`: `Runtime()`

Low-level direct methods like `DeleteSandbox` and `GetSandboxLogs` remain available on the root client when you want stateless calls.

### Operator APIs

The root client also includes operator-oriented methods such as `GetPoolStatus`, `StartRollingUpdate`, `GetRollingUpdateStatus`, and `CancelRollingUpdate`.

These routes are intended for platform operators, not normal application workloads. Keep them out of business-facing integrations unless you are explicitly building operational tooling.

### Build Plane Through `client.Build`

`client.Build` exposes:

- system: `Metrics`
- direct build: `DirectBuild`
- templates: `CreateTemplate`, `ListTemplates`, `GetTemplateByAlias`, `GetTemplate`, `UpdateTemplate`, `DeleteTemplate`
- builds: `CreateBuild`, `GetBuildFile`, `RollbackTemplate`, `ListBuilds`, `GetBuild`, `GetBuildStatus`, `GetBuildLogs`

The public template request surface intentionally stays small: `name`, `image` or `dockerfile`, and a few optional runtime settings such as `visibility`, `baseTemplateID`, `envs`, `cpuCount`, `memoryMB`, `diskSizeMB`, `ttlSeconds`, `port`, `startCmd`, `readyCmd`.

`CreateTemplate` and `UpdateTemplate` reject `visibility=official` in the public SDK.

`GetTemplateByAlias` is a stable-ref lookup endpoint. It resolves a template by `templateID` or by an official template `type`; it should not be treated as a personal/team display-name search API.

### Runtime Helper

The object returned by `createdSandbox.Runtime()` or `client.RuntimeFromSandbox(...)` exposes:

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

- The gateway entrypoint only needs `baseURL + apiKey` to initialize.
- Gateway routing context is derived from the API key; SDK callers should not construct control/build headers manually.
- Runtime access should be derived from sandbox response objects with `Runtime()` or `RuntimeFromSandbox(...)`.
- `CreateSandbox` and `GetSandbox` responses include `EnvdURL` and `EnvdAccessToken` when the target sandbox supports CMD access.
- Runtime file/process APIs require a template image that starts nano-executor and returns runtime access fields; if runtime APIs return `404`, verify the selected template supports CMD runtime routes.
- `waitReady=true` can take longer than the default HTTP timeout in production; pass `core.WithTimeout(...)` when creating long-wait clients.
- API errors expose `Kind` and `Retryable()` for retry logic and alert routing.
- Sandbox timeout values are validated to `0..86400`; refresh duration to `0..3600`.
- Build request validation currently rejects unsupported `fromImageRegistry`, `force`, and per-step `args`/`force`.
- Some gateways do not expose `/admin/*` or `/build`; integration tests skip those cases on `404`.

## Security

- Do not commit `SEACLOUD_API_KEY`, `envdAccessToken`, or sandbox access tokens.
- Treat runtime tokens as sandbox-scoped secrets. Prefer `createdSandbox.Runtime()` or `client.RuntimeFromSandbox(...)` so response-scoped runtime access is not copied into configuration.
- Do not log raw API keys or runtime tokens. SDK errors may include response bodies, so avoid logging full error payloads in multi-tenant systems.
- The SDK does not construct tenant routing headers. Gateway routing context is derived from the API key.

## Production Smoke

Use production smoke tests only with explicitly provided credentials and disposable sandboxes:

```bash
SANDBOX_RUN_INTEGRATION=1 \
SANDBOX_TEST_BASE_URL="${SEACLOUD_BASE_URL}" \
SANDBOX_TEST_API_KEY="${SEACLOUD_API_KEY}" \
SANDBOX_TEST_TEMPLATE_ID=tpl-base-dc11799b9f9f4f9e \
go test ./tests -run Integration -v
```

`tpl-base-dc11799b9f9f4f9e` is a known-good SeaCloudAI runtime template for validating CMD routes such as `ListDir`, `ReadFile`, `WriteFile`, and `Run`.

## Integration Tests

```bash
SANDBOX_RUN_INTEGRATION=1 \
SANDBOX_TEST_BASE_URL="${SEACLOUD_BASE_URL}" \
SANDBOX_TEST_API_KEY="${SEACLOUD_API_KEY}" \
SANDBOX_TEST_TEMPLATE_ID=... \
go test ./tests -v
```

Use a runtime-enabled template for CMD integration coverage. For SeaCloudAI production smoke tests, `tpl-base-dc11799b9f9f4f9e` is a known-good runtime template.

## Release

- See `CHANGELOG.md` for release notes.
- See `RELEASE_CHECKLIST.md` before tagging or publishing a new version.
