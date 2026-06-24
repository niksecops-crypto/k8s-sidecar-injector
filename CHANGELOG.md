# Changelog

## [1.2.0] - 2024-12-10
### Added
- Benchmark tests for `MutatePod`, `GetTemplate`, and `Reload` (`pkg/mutation/mutate_bench_test.go`)
- Example sidecar configs: Falco agent, OpenTelemetry Collector, Envoy proxy (`examples/`)
- CHANGELOG

### Changed
- `SidecarConfigManager.Reload` is now safe to call concurrently (RWMutex)

## [1.1.0] - 2024-11-02
### Added
- Prometheus metrics endpoint (`/metrics`)
- Readiness probe (`/readyz`) separate from liveness (`/healthz`)
- Dagger CI pipeline: SAST (gosec), SCA (trivy), K8s manifest linting (kubeconform)
- Helm chart with cert-manager TLS integration

### Changed
- Sidecar template moved from static file to dynamic ConfigMap reload
- Structured JSON logging via `log/slog`

## [1.0.0] - 2024-10-01
### Added
- Initial release: Kubernetes mutating admission webhook
- Automatic sidecar injection based on `sidecar-injector/inject: "true"` annotation
- RBAC manifests and TLS cert generation script
- GitHub Actions CI
