# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 1.x     | ✅        |
| < 1.0   | ❌        |

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Report privately via GitHub Security Advisories:
👉 [Report a vulnerability](https://github.com/niksecops-crypto/k8s-sidecar-injector/security/advisories/new)

Or email: **security@niksecops.dev**

You will receive an acknowledgement within **48 hours** and a resolution timeline within **7 days**.

## Security Considerations

- The webhook server requires a valid TLS certificate — use cert-manager or rotate certs regularly
- Sidecar templates are loaded from a ConfigMap; restrict write access to that ConfigMap via RBAC
- The webhook runs with a minimal RBAC profile — avoid granting additional cluster permissions
- Validate sidecar images with admission policies (OPA/Kyverno) before injection into production namespaces
