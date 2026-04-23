# Changelog

All notable changes to this project will be documented in this file.

This project follows Semantic Versioning for public SDK APIs.

## [0.1.0] - 2026-04-23

### Added

- Initial Go SDK for SeaCloudAI sandbox control-plane, build-plane, and runtime CMD APIs.
- Unified root client initialization with `sandbox.NewClient(baseURL, apiKey)`.
- Build namespace through `client.Build`.
- Runtime helpers through `client.Runtime(...)`, `client.RuntimeFromSandbox(...)`, and bound sandbox objects.
- Typed API errors with retry classification.
- Configurable HTTP timeout through `core.WithTimeout(...)`.
- Examples, unit tests, and integration-test scaffolding.
