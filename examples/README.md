# Go Examples

Run examples from the package root.

Recommended reading order:

1. `control_sandbox`: root client -> create sandbox -> bound sandbox helpers -> cleanup
2. `cmd_smoke`: create a sandbox through the gateway, then run runtime operations
3. `build_template`: template/build workflows through `client.Build`

## Control Plane

This example shows the preferred workflow:

- initialize the root `sandbox.NewClient(...)`
- create a sandbox from the root client
- keep operating through the returned bound sandbox object
- cleanup through the same object

Required env:

- `SEACLOUD_BASE_URL`
- `SEACLOUD_API_KEY`
- `SANDBOX_EXAMPLE_TEMPLATE_ID`

Optional env:

- `SANDBOX_EXAMPLE_KEEP_RESOURCES=1`

```bash
go run ./examples/control_sandbox
```

## Build Plane

Recommended path: the example uses the root `sandbox.NewClient(...)` and `client.Build`.

Required env:

- `SEACLOUD_BASE_URL`
- `SEACLOUD_API_KEY`

Optional env:

- `SANDBOX_EXAMPLE_BUILD_IMAGE`
- `SANDBOX_EXAMPLE_KEEP_RESOURCES=1`

```bash
go run ./examples/build_template
```

## CMD Plane

Recommended path: the example uses the root `sandbox.NewClient(...)`, creates a sandbox through the gateway, then derives runtime access from the returned sandbox object.
The selected template must include nano-executor runtime support; otherwise file/process/RPC calls can return `404`.

Required env:

- `SEACLOUD_BASE_URL`
- `SEACLOUD_API_KEY`
- `SANDBOX_EXAMPLE_TEMPLATE_ID`

Optional env:

- `SANDBOX_EXAMPLE_SANDBOX_ROOT`
- `SANDBOX_EXAMPLE_KEEP_RESOURCES=1`

```bash
go run ./examples/cmd_smoke
```

For SeaCloudAI production smoke tests, `tpl-base-dc11799b9f9f4f9e` is a known-good template to use when creating the runtime-enabled sandbox.
