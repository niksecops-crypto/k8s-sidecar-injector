# Kubernetes Mutating Webhook: Sidecar Injector (Enterprise-Ready)

[Русский](README.ru.md) | [English](README.md)

---

## English Version

### Project Overview
This project implements a **Mutating Admission Webhook** for Kubernetes in Go. It automatically injects sidecar containers into newly created pods based on annotations.

This project goes beyond standard injectors by leveraging **Kubernetes Native Sidecars (initContainers + RestartPolicy: Always)** introduced in Kubernetes 1.28+. This guarantees perfect startup and shutdown ordering for your sidecars!

Read more about why this matters in our [Native Sidecars Wiki](docs/wiki/native-sidecars.md).

### Key Enterprise Features
- **Native Sidecars**: Injects into `initContainers` instead of `containers` to ensure sidecars start first and die last.
- **Hot-Reloadable Config**: Change the sidecars injected via ConfigMap without restarting the webhook (Zero downtime).
- **Strict Configuration Validation**: Checks for duplicate sidecar names and missing images before accepting config changes.
- **Prometheus Metrics**: Built-in metrics (`sidecar_injector_mutations_total` and `sidecar_injector_mutation_duration_seconds`) at `:8443/metrics`.
- **Restricted Pod Security**: Runs completely non-root, read-only filesystem, dropping all capabilities.
- **Auto-TLS Fallback**: Works natively with `cert-manager` for production, but automatically generates self-signed certificates for quick local development.

### Technical Stack
- **Go 1.21+**: High performance, zero dependency core.
- **Helm 3**: Robust packaging and deployment.
- **JSON Patch (RFC 6902)**: Standardized, exact mutations without altering the original pod source code.

---

### Getting Started

#### 1. Clone the repository
```bash
git clone https://github.com/niksecops-crypto/k8s-sidecar-injector.git
cd k8s-sidecar-injector
```

#### 2. Deploy using Helm
This is the recommended way to install the injector.

```bash
helm upgrade --install k8s-sidecar-injector ./deploy/helm/k8s-sidecar-injector \
    --namespace sidecar-injector --create-namespace
```

If you don't use `cert-manager`, the Helm chart will automatically pass `AUTO_GENERATE_CERT=true` to the deployment and manage a temporary self-signed certificate for you in an `emptyDir` volume!

### Validation

To verify the injection is working, run a test pod with the `sidecar-injector.io/inject: "true"` annotation:

```bash
kubectl run test-pod --image=nginx --restart=Never --labels="sidecar-injector.io/inject=true" --annotations="sidecar-injector.io/inject=true"
```

Check the `initContainers` in the pod to see the injected Native Sidecar:
```bash
kubectl get pod test-pod -o jsonpath='{.spec.initContainers[*].name}'
```
**Expected output:** `security-agent` (or whichever sidecars you defined in `values.yaml`).

---

### Customization
Edit `deploy/helm/k8s-sidecar-injector/values.yaml` to configure which sidecars to inject.

```yaml
sidecars:
  - name: "my-sidecar"
    image: "my-image:latest"
    # ... any standard corev1.Container specs
```

---
**Developed specifically for Nik577.**
