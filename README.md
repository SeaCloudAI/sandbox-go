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

## Recommended Workflow

Most applications only need the root client:

1. Initialize `sandbox.NewClient(baseURL, apiKey)`.
2. Create, list, get, or connect sandboxes through the root client.
3. Keep working from the bound sandbox object:
   `Reload()`, `Logs()`, `Pause()`, `Refresh()`, `SetTimeout()`, `Connect()`, `Delete()`.
4. When the sandbox exposes `EnvdURL`, switch into runtime operations through `sandbox.Runtime()`.
5. Use `client.Build` only for template/build workflows.

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
		"https://sandbox-gateway.cloud.seaart.ai",
		os.Getenv("SEACLOUD_API_KEY"),
		core.WithTimeout(180*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}

	ready := true
	timeout := int32(1800)
	createdSandbox, err := client.CreateSandbox(context.Background(), &control.NewSandboxRequest{
		TemplateID:  "base",
		WorkspaceID: "go-sdk-demo",
		WaitReady:   &ready,
		Timeout:     &timeout,
	})
	if err != nil {
		log.Fatal(err)
	}

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
		"https://sandbox-gateway.cloud.seaart.ai",
		os.Getenv("SEACLOUD_API_KEY"),
	)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.Build.CreateTemplate(context.Background(), &build.TemplateCreateRequest{
		Name:       "demo",
		Visibility: "personal",
		Image:      "docker.io/library/alpine:3.20",
	})
	if err != nil {
		log.Fatal(err)
	}

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
		"https://sandbox-gateway.cloud.seaart.ai",
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

## Root Client First

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
- admin: `GetPoolStatus`, `StartRollingUpdate`, `GetRollingUpdateStatus`, `CancelRollingUpdate`

Recommended root-client path:

- sandbox lifecycle: `CreateSandbox`, `ListSandboxes`, `GetSandbox`, `ConnectSandbox`
- follow-up control actions from the returned object: `Reload()`, `Logs()`, `Pause()`, `Refresh()`, `SetTimeout()`, `Connect()`, `Delete()`
- runtime actions from objects that include `EnvdURL`: `Runtime()`

Low-level direct methods like `DeleteSandbox` and `GetSandboxLogs` remain available on the root client when you want stateless calls.

### Build Plane Through `client.Build`

`client.Build` exposes:

- system: `Metrics`
- direct build: `DirectBuild`
- templates: `CreateTemplate`, `ListTemplates`, `GetTemplateByAlias`, `GetTemplate`, `UpdateTemplate`, `DeleteTemplate`
- builds: `CreateBuild`, `GetBuildFile`, `RollbackTemplate`, `ListBuilds`, `GetBuild`, `GetBuildStatus`, `GetBuildLogs`

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
SANDBOX_TEST_BASE_URL=https://sandbox-gateway.cloud.seaart.ai \
SANDBOX_TEST_API_KEY=... \
SANDBOX_TEST_TEMPLATE_ID=tpl-base-dc11799b9f9f4f9e \
go test ./tests -run Integration -v
```

`tpl-base-dc11799b9f9f4f9e` is a known-good SeaCloudAI runtime template for validating CMD routes such as `ListDir`, `ReadFile`, `WriteFile`, and `Run`.

## Integration Tests

```bash
SANDBOX_RUN_INTEGRATION=1 \
SANDBOX_TEST_BASE_URL=https://sandbox-gateway.cloud.seaart.ai \
SANDBOX_TEST_API_KEY=... \
SANDBOX_TEST_TEMPLATE_ID=... \
go test ./tests -v
```

Use a runtime-enabled template for CMD integration coverage. For SeaCloudAI production smoke tests, `tpl-base-dc11799b9f9f4f9e` is a known-good runtime template.

## Release

- See `CHANGELOG.md` for release notes.
- See `RELEASE_CHECKLIST.md` before tagging or publishing a new version.
