# Go Examples

Run examples from the package root.

Shared env:

- `SEACLOUD_BASE_URL`
- `SEACLOUD_API_KEY`

Before running any example, export these variables once in your shell. Use the gateway entrypoint documented in the package `README.md`.

Example-specific inputs intentionally use the `SANDBOX_EXAMPLE_*` prefix so they do not collide with the production-oriented variables shown in the package `README.md`.
Examples focus on the stable lifecycle, template, command, and PTY flows. Watcher APIs are covered in tests instead, because some sandbox filesystem layouts reject them entirely.

Recommended reading order:

1. `zero_to_one`: env setup -> official templates -> lifecycle -> command/files -> frontend URL -> local-code template build
2. `code_interpreter`: default Python context -> explicit Python context -> non-Python stateless `context`
3. `full_workflow`: create a template -> trigger a build -> wait for build -> start sandbox -> connect runtime -> run -> logs/metrics -> cleanup
4. `template_features`: `FromDockerfile` -> local `Copy(..., Mode/ResolveSymlinks/User)` -> `sandbox.BuildTemplateInBackground(...)` -> `sandbox.GetTemplateBuildStatus(...)` -> existence/detail
5. `control_sandbox`: `sandbox.Create(...)` -> reload -> cleanup
6. `cmd_smoke`: `sandbox.Create(...)` -> `Files()` / `Commands()`
7. `build_template`: minimal `sandbox.BuildTemplate(...)`

## Zero To One

This is the tutorial-style example for first-time users:

- create a `base` sandbox and run basic file/command operations
- pause and resume the sandbox to show lifecycle management
- create a `code-interpreter` sandbox and run Python code
- deploy a tiny static frontend inside a sandbox and print the public proxy URL from `GetHost(3000)`
- build a new template by uploading local frontend files with `Template.Copy(...)`

Optional env:

- `SANDBOX_EXAMPLE_BASE_TEMPLATE=base`
- `SANDBOX_EXAMPLE_CODE_TEMPLATE=code-interpreter`
- `SANDBOX_EXAMPLE_FRONTEND_TEMPLATE=code-interpreter`
- `SANDBOX_EXAMPLE_KEEP_RESOURCES=1`

```bash
go run ./examples/zero_to_one
```

## Code Interpreter

This example focuses on the E2B-style code interpreter facade:

- repeated `RunCode(...)` calls sharing the default Python context
- explicit stateful Python contexts with `CreateCodeContext(...)`
- non-Python contexts acting as reusable execution profiles for `Language`, `CWD`, and `TimeoutMS`
- requires a template that actually bundles the code-interpreter environment; `base` is not enough

Required env:

- `SANDBOX_EXAMPLE_TEMPLATE_ID`

Optional env:

- `SANDBOX_EXAMPLE_KEEP_RESOURCES=1`

```bash
go run ./examples/code_interpreter
```

For SeaCloudAI environments, prefer an official `code-interpreter` template or a concrete `tpl-code-interpreter-...` template ID for this example.

## Full Workflow

This is the primary example when evaluating the SDK end to end:

- create a template
- trigger a build from a runtime-enabled image
- wait for the build to finish
- inspect build status, build logs, and template detail
- start a sandbox from that template
- reload, fetch sandbox logs, connect, inspect runtime metrics, and run a command
- delete the sandbox and template unless `SANDBOX_EXAMPLE_KEEP_RESOURCES=1`

Required env:

- `SANDBOX_EXAMPLE_RUNTIME_BASE_IMAGE`

Optional env:

- `SANDBOX_EXAMPLE_KEEP_RESOURCES=1`

The source image must already be runtime-enabled for CMD APIs.

```bash
go run ./examples/full_workflow
```

## Control Plane

This example shows the preferred workflow:

- create a sandbox through `sandbox.Create(...)`
- keep operating through the returned bound sandbox object
- reload once to show the bound-object workflow
- cleanup through the same object

Required env:

- `SANDBOX_EXAMPLE_TEMPLATE_ID`

Optional env:

- `SANDBOX_EXAMPLE_KEEP_RESOURCES=1`

```bash
go run ./examples/control_sandbox
```

## Build Plane

Recommended path: the example uses `sandbox.BuildTemplate(...)`.
The flow shows the env-first high-level template workflow directly: template DSL -> build polling -> template detail -> cleanup.

Required env: none

Optional env:

- `SANDBOX_EXAMPLE_BUILD_IMAGE`
- `SANDBOX_EXAMPLE_KEEP_RESOURCES=1`

```bash
go run ./examples/build_template
```

## Template Features

This example covers the supported template helpers that are not obvious from the minimal build flow:

- parse a Dockerfile from disk with `FromDockerfile`
- inspect the generated request with `sandbox.TemplateToJSON(...)` and `sandbox.TemplateToDockerfile(...)`
- add extra steps with `SkipCache()` and `RunCmd(..., &sandbox.TemplateCommandOptions{User: ...})`
- upload a local symlink target with `Copy(..., &sandbox.TemplateCopyOptions{Mode, ResolveSymlinks, User})`
- trigger `sandbox.BuildTemplateInBackground(...)` and poll with `sandbox.GetTemplateBuildStatus(...)`
- verify template existence with `sandbox.TemplateExists(...)` and inspect template detail with `sandbox.GetTemplate(...)`

Required env: none

Optional env:

- `SANDBOX_EXAMPLE_BUILD_IMAGE`
- `SANDBOX_EXAMPLE_KEEP_RESOURCES=1`

```bash
go run ./examples/template_features
```

## CMD Plane

Recommended path: the example uses `sandbox.Create(...)` and then stays on `Files()` / `Commands()`.
The selected template must include nano-executor runtime support; otherwise file/process/RPC calls can return `404`.
The flow stays minimal: write file -> read file -> list directory -> run command.
The example writes under `/root/workspace`, which is the writable sandbox workspace in the current SeaCloud runtime.

Required env:

- `SANDBOX_EXAMPLE_TEMPLATE_ID`

Optional env:

- `SANDBOX_EXAMPLE_KEEP_RESOURCES=1`

```bash
go run ./examples/cmd_smoke
```

For SeaCloudAI production smoke tests, `tpl-base-dc11799b9f9f4f9e` is a known-good template for CMD/runtime examples such as this one. Use a `code-interpreter` template instead when you want to run `RunCode(...)`.
