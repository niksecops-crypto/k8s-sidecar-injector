# Kubernetes Mutating Webhook: Sidecar Injector (Enterprise-Ready)

[![CI](https://github.com/niksecops-crypto/k8s-sidecar-injector/actions/workflows/ci.yml/badge.svg)](https://github.com/niksecops-crypto/k8s-sidecar-injector/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

[English](#english) | [Русский](README.ru.md)

---

## English

### Project Overview
This project implements a **Production-Ready Mutating Admission Webhook** for Kubernetes in Go. It enables a **Zero-trust architecture** by automatically injecting security sidecars, log collectors, or proxy services into Pods without requiring developers to modify their Dockerfiles or manifests (**Non-intrusive**).

### Architecture
```text
User/CI-CD
    |
    v
+-----------------------+
|  K8s API Server       |
+-----------+-----------+
            |
            | (1) Admission Review Request
            v
+-----------------------+
| Mutating Webhook      | (2) Read Sidecar Template from ConfigMap
| (This Go Service)     | (3) Generate JSON Patch
+-----------+-----------+
            |
            | (4) Admission Review Response (Patch)
            v
+-----------------------+
|  K8s API Server       |
+-----------+-----------+
            |
            | (5) Create Pod with Injected Sidecar
            v
+-----------+-----------+
| Pod                  |
|  +----------------+  |
|  | Main Container |  |
|  +----------------+  |
|  | Sidecar        |  |
|  +----------------+  |
+----------------------+
```

### Enterprise Features
- **Dynamic Configuration**: Sidecar templates are defined in a **ConfigMap**. Update the template and reload the webhook via SIGHUP without recompilation.
- **Zero-Trust & Security**: Integrated with **cert-manager** for automated TLS certificate management in production.
- **Dagger Zero-Trust CI/CD**: 🚀 Powered by **Dagger Go SDK**. Our pipeline isn't just a script; it's a portable, containerized Go application. **No YAML-hell**, **100% reproducible** on your laptop or any CI provider. It includes:
  - **SAST (gosec)**: Deep code analysis for security vulnerabilities.
  - **SCA (trivy)**: Real-time dependency & container image scanning.
  - **K8s Linting**: Validating manifests against industry-standard security policies.
- **Production-Ready**: Includes **Graceful Shutdown**, **Prometheus Metrics**, and **Health Probes**.
- **High Observability**: Structured JSON logging using Go 1.21 `slog`.
- **CI/CD Integration**: Automated linting and testing (60%+ coverage) via GitHub Actions.

### Technical Stack
- **Go 1.21+**: High performance and efficiency.
- **Helm**: Standard package manager for K8s deployment.
- **cert-manager**: Industry standard for X.509 certificate management.
- **Prometheus**: Real-time monitoring and metrics.

---

### Dagger CI/CD (Zero-Trust Pipeline)
Forget about fragile YAML scripts. Our project uses a **Dagger-powered Go Module** to handle CI/CD. It’s a 100% portable, type-safe pipeline that runs anywhere—on your laptop, GitHub Actions, or any cloud. 

**Why Dagger?**
- **No YAML-Hell**: CI logic is written in pure Go.
- **Portability**: Run the exact same pipeline locally: `dagger call run-all --source .`.
- **Zero-Trust**: Every build stage runs in a secure, isolated container.
- **Speed**: Intelligent caching across all stages.

---

### Installation & Deployment

#### 1. Helm Deployment (Recommended for Production)
The Helm chart supports automated TLS via cert-manager.

```bash
cd deploy/helm/k8s-sidecar-injector
helm install sidecar-injector . -n sidecar-injector --create-namespace
```

#### 2. Local Development (Self-signed)
```bash
# Generate certs locally
chmod +x scripts/gen-certs.sh
./scripts/gen-certs.sh

# Apply manifests
kubectl apply -f manifests/
```

### Dynamic Configuration (ConfigMap)
The sidecar template is stored in a ConfigMap. You can modify it at runtime:
```yaml
# Example ConfigMap entry
sidecar.yaml: |
  name: "security-agent"
  image: "falcosecurity/falco-no-driver:latest"
  args: ["/usr/bin/falco", "-A"]
```
After updating the ConfigMap, the webhook will reload the template automatically if the pod is restarted or if you send a SIGHUP signal to the process.

### Validation
```bash
kubectl run nginx --image=nginx
kubectl get pod nginx -o jsonpath='{.spec.containers[*].name}'
# Output: nginx security-agent
```

---
**Developed by niksecops-crypto**
