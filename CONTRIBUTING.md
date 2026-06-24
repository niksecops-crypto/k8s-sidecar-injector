# Contributing to k8s-sidecar-injector

## Getting Started

```bash
git clone https://github.com/niksecops-crypto/k8s-sidecar-injector.git
cd k8s-sidecar-injector
go mod download
go test ./...
```

## Development Workflow

1. Fork and branch: `git checkout -b feat/my-feature`
2. Make changes with tests
3. `go test -race ./...` — must pass
4. `golangci-lint run` — no errors
5. Open a PR against `main`

## Running Benchmarks

```bash
go test -bench=. -benchmem ./pkg/mutation/
```

## Local Webhook Testing

1. Generate certs: `./scripts/gen-certs.sh`
2. Run the webhook: `go run ./cmd/webhook --cert-file tls.crt --key-file tls.key`
3. Use `kubectl` with a test cluster (kind/minikube) and apply `manifests/webhook-config.yaml`

## Adding Sidecar Examples

Add new YAML templates to `examples/`. Follow the naming convention: `<tool>-sidecar.yaml`. Each file should include comments explaining use-case and any required cluster setup.

## Reporting Issues

Open a [GitHub Issue](https://github.com/niksecops-crypto/k8s-sidecar-injector/issues) with:
- Kubernetes version
- Webhook version
- AdmissionReview request/response (redact sensitive data)
