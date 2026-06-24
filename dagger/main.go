package main

import (
	"context"
	"fmt"
)

// Ci is a Dagger module for k8s-sidecar-injector DevSecOps pipeline.
// It implements a 'Zero-Trust Pipeline' focusing on security and portability.
type Ci struct{}

// RunAll executes the full Zero-Trust pipeline with custom options.
func (m *Ci) RunAll(
	ctx context.Context,
	source *Directory,
	// +optional
	// +default="HIGH,CRITICAL"
	severity string,
	// +optional
	// +default=false
	skipLint bool,
) error {
	// 1. Linting (Conditional)
	if !skipLint {
		if _, err := m.Lint(ctx, source).Sync(ctx); err != nil {
			return fmt.Errorf("lint failed: %w", err)
		}
	}

	// 2. Security Scan (SAST)
	if _, err := m.Gosec(ctx, source).Sync(ctx); err != nil {
		return fmt.Errorf("gosec failed: %w", err)
	}

	// 3. Vulnerability Scan (SCA) with custom severity
	if _, err := m.TrivySca(ctx, source, severity).Sync(ctx); err != nil {
		return fmt.Errorf("trivy sca failed: %w", err)
	}

	// 4. Build & Container Scan
	container := m.Build(source)
	if _, err := m.TrivyImage(ctx, container, severity).Sync(ctx); err != nil {
		return fmt.Errorf("trivy image scan failed: %w", err)
	}

	// 5. K8s Manifest Validation
	if _, err := m.KubeLinter(ctx, source).Sync(ctx); err != nil {
		return fmt.Errorf("kubelinter failed: %w", err)
	}

	return nil
}

// Lint runs golangci-lint to ensure code quality.
func (m *Ci) Lint(ctx context.Context, source *Directory) *Container {
	return dag.Container().
		From("golangci/golangci-lint:latest-alpine").
		WithDirectory("/src", source).
		WithWorkdir("/src").
		WithExec([]string{"golangci-lint", "run", "-v"})
}

// Gosec runs SAST (Static Application Security Testing) to find security holes.
func (m *Ci) Gosec(ctx context.Context, source *Directory) *Container {
	return dag.Container().
		From("securego/gosec:latest").
		WithDirectory("/src", source).
		WithWorkdir("/src").
		WithExec([]string{"gosec", "./..."})
}

// TrivySca scans project dependencies for known vulnerabilities.
func (m *Ci) TrivySca(
	ctx context.Context,
	source *Directory,
	// +optional
	// +default="HIGH,CRITICAL"
	severity string,
) *Container {
	return dag.Container().
		From("aquasec/trivy:latest").
		WithDirectory("/src", source).
		WithWorkdir("/src").
		WithExec([]string{"fs", "--exit-code", "1", "--severity", severity, "."})
}

// Build compiles the Go application and creates a minimal Docker image.
func (m *Ci) Build(source *Directory) *Container {
	builder := dag.Container().
		From("golang:1.22-alpine").
		WithDirectory("/src", source).
		WithWorkdir("/src").
		WithExec([]string{"go", "build", "-o", "webhook", "./cmd/webhook/main.go"})

	return dag.Container().
		From("gcr.io/distroless/static-debian12").
		WithFile("/usr/local/bin/webhook", builder.File("/src/webhook")).
		WithEntrypoint([]string{"/usr/local/bin/webhook"})
}

// TrivyImage scans the built container image for vulnerabilities.
func (m *Ci) TrivyImage(
	ctx context.Context,
	container *Container,
	// +optional
	// +default="HIGH,CRITICAL"
	severity string,
) *Container {
	return dag.Container().
		From("aquasec/trivy:latest").
		WithFile("/tmp/image.tar", container.AsTarball()).
		WithExec([]string{"image", "--input", "/tmp/image.tar", "--exit-code", "1", "--severity", severity})
}

// KubeLinter validates Kubernetes manifests and Helm charts against security best practices.
func (m *Ci) KubeLinter(ctx context.Context, source *Directory) *Container {
	return dag.Container().
		From("stackrox/kube-linter:latest-alpine").
		WithDirectory("/src", source).
		WithWorkdir("/src").
		WithExec([]string{"lint", "deploy/helm/k8s-sidecar-injector"})
}
