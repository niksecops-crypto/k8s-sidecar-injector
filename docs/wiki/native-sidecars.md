# Kubernetes Native Sidecars (KEP-753)

## What are Native Sidecars?
In Kubernetes 1.28+, a new feature called "SidecarContainers" was introduced (stable in 1.33+). Instead of adding sidecars to the standard `containers` array, native sidecars are added to the `initContainers` array with `restartPolicy: Always`.

### Why is this better?
1. **Startup Order**: Native sidecars start *before* your main application containers. If you are injecting a proxy (like Envoy) or a security agent (like Falco), you want it fully running before your app starts processing requests.
2. **Shutdown Order**: Native sidecars are terminated *after* all main containers have stopped. This ensures your app can still send logs, metrics, or traffic during graceful shutdown.
3. **Job Compatibility**: Previously, sidecars in `Job` pods would run forever, preventing the Job from completing. Native sidecars automatically terminate when the main containers finish.

## How k8s-sidecar-injector uses them
Unlike most older webhook implementations that inject into `/spec/containers`, this project is built for the modern Kubernetes era. 

When a pod is annotated with `sidecar-injector.io/inject: "true"`, the injector parses your `SidecarConfig` and automatically:
1. Validates the container definition.
2. Forces `restartPolicy: Always` on the container.
3. Injects the container into `/spec/initContainers`.

## Example
If you configure the injector to add an OpenTelemetry collector, the resulting Pod will look like this:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-app
spec:
  initContainers:
    - name: otel-collector
      image: otel/opentelemetry-collector:0.96.0
      restartPolicy: Always   # <--- Marks it as a Native Sidecar
  containers:
    - name: app
      image: my-app:v1
```
