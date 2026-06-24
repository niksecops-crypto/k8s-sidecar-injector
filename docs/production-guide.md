# k8s-sidecar-injector: Production Deployment Guide

## Overview

k8s-sidecar-injector is a Kubernetes mutating admission webhook. When a pod is submitted to the API server, the webhook is called and injects pre-configured sidecar containers before the pod starts.

Key properties:
- **Opt-in**: only pods annotated with `sidecar-injector.io/inject: "true"` are mutated
- **Idempotent**: sidecars already present (matched by container name) are skipped
- **Multi-sidecar**: a single config file can define multiple sidecars injected in one call
- **Hot-reload**: send `SIGHUP` to reload config without restarting the webhook process

---

## Prerequisites

| Requirement | Version |
|-------------|---------|
| Kubernetes | 1.16+ (AdmissionWebhook enabled) |
| cert-manager | 1.x (or manual TLS cert management) |
| Go | 1.22+ (build only) |

---

## Installation

### 1. Generate TLS certificates

The webhook must be served over HTTPS. Use cert-manager (recommended) or generate certificates manually:

```bash
# Manual cert generation
./scripts/gen-certs.sh
# Creates tls.crt, tls.key, and outputs the base64-encoded CA bundle
```

### 2. Create the sidecar config ConfigMap

Choose one of the example configs from `examples/`:

```bash
# Single sidecar: Falco security agent
kubectl create configmap sidecar-config \
  --from-file=sidecar.yaml=examples/falco-sidecar.yaml \
  -n sidecar-injector

# Multi-sidecar: Falco + OTel Collector
kubectl create configmap sidecar-config \
  --from-file=sidecar.yaml=examples/full-stack-config.yaml \
  -n sidecar-injector
```

### 3. Deploy the webhook

```bash
kubectl create namespace sidecar-injector

# Apply RBAC
kubectl apply -f deploy/rbac.yaml

# Apply the Deployment and Service
kubectl apply -f deploy/deployment.yaml
kubectl apply -f deploy/service.yaml

# Apply the MutatingWebhookConfiguration (replace CA_BUNDLE first)
CA_BUNDLE=$(kubectl get secret sidecar-injector-tls -n sidecar-injector \
  -o jsonpath='{.data.ca\.crt}')
sed "s|\${CA_BUNDLE}|${CA_BUNDLE}|g" manifests/webhook-config.yaml | kubectl apply -f -
```

---

## Sidecar Configuration Format

The config file (mounted at `SIDECAR_CONFIG_PATH`, default `/etc/webhook/config/sidecar.yaml`) uses the `sidecars:` key with a list of standard Kubernetes container specs:

```yaml
sidecars:
  - name: my-sidecar
    image: myregistry.io/sidecar:v1.2.0
    args: ["--config", "/etc/sidecar/config.yaml"]
    securityContext:
      runAsNonRoot: true
      runAsUser: 1000
      readOnlyRootFilesystem: true
      allowPrivilegeEscalation: false
      capabilities:
        drop: [ALL]
    resources:
      requests:
        cpu: 50m
        memory: 64Mi
      limits:
        cpu: 200m
        memory: 256Mi
    volumeMounts:
      - mountPath: /etc/sidecar
        name: sidecar-config
        readOnly: true
```

Any field valid in a Kubernetes `Container` spec is supported.

---

## Opt-In Annotation

To inject sidecars into a pod, add the annotation to the pod template:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    metadata:
      labels:
        app: my-app
        sidecar-injector.io/inject: "true"   # ← enables injection
      annotations:
        sidecar-injector.io/inject: "true"   # ← annotation (webhook reads this)
    spec:
      containers:
        - name: app
          image: myapp:v1
```

Pods **without** this annotation are passed through the webhook unchanged.

---

## Hot-Reload

To update the sidecar config without restarting the webhook:

```bash
# Update the ConfigMap
kubectl create configmap sidecar-config \
  --from-file=sidecar.yaml=examples/full-stack-config.yaml \
  -n sidecar-injector \
  --dry-run=client -o yaml | kubectl apply -f -

# Signal the webhook pod to reload
kubectl exec -n sidecar-injector deploy/sidecar-injector -- kill -HUP 1
```

New pods created after the reload will use the updated config. Running pods are unaffected.

---

## Namespace Scoping

The `manifests/webhook-config.yaml` already excludes `kube-system` and `kube-public` via `namespaceSelector`. To further restrict to specific namespaces:

```yaml
namespaceSelector:
  matchLabels:
    sidecar-injection: enabled
```

Then label target namespaces:

```bash
kubectl label namespace production sidecar-injection=enabled
kubectl label namespace staging sidecar-injection=enabled
```

---

## Prometheus Metrics

The webhook exposes Prometheus metrics at `/metrics` (same port as the webhook, `:8443`).

| Metric | Description |
|--------|-------------|
| `webhook_requests_total` | Total admission review requests received |
| `webhook_mutations_total` | Total pods mutated (at least one sidecar injected) |
| `webhook_duration_seconds` | Histogram of admission review processing time |

---

## Commercial Patterns

### Security observability fleet (Falco everywhere)

Deploy Falco into every production pod without modifying Deployments:

```yaml
# 1. Create the config
kubectl create configmap sidecar-config \
  --from-file=sidecar.yaml=examples/falco-sidecar.yaml \
  -n sidecar-injector

# 2. Label all application namespaces
kubectl label namespace production sidecar-injection=enabled

# 3. Label pods (or add to Deployment template via GitOps):
kubectl patch deployment my-app -n production \
  --type=json \
  -p='[{"op":"add","path":"/spec/template/metadata/annotations","value":{"sidecar-injector.io/inject":"true"}}]'
```

### Distributed tracing rollout (OTel Collector)

Add the OTel Collector sidecar to services participating in distributed tracing without code changes:

```bash
kubectl create configmap sidecar-config \
  --from-file=sidecar.yaml=examples/otel-collector-sidecar.yaml \
  -n sidecar-injector
```

Services send traces to `localhost:4317` (gRPC) or `localhost:4318` (HTTP) — no service discovery needed.

---

## Troubleshooting

**Pod admission fails with "connection refused"**
Verify the webhook pod is running: `kubectl get pods -n sidecar-injector`

**Sidecar is not being injected despite annotation**
1. Check annotation spelling: `sidecar-injector.io/inject: "true"` (string, not boolean)
2. Check webhook logs: `kubectl logs -n sidecar-injector deploy/sidecar-injector`
3. Verify the namespace is not excluded by `namespaceSelector`

**"sidecar already present, skipping" in logs**
Expected behavior — the webhook detected a container with the same name and skipped injection. This prevents duplicate sidecars on pod restarts.

**Config reload failed after SIGHUP**
Inspect webhook logs for YAML parse errors. The old config remains active until a valid config is loaded.
