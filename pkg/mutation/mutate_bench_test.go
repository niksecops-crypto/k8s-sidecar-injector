package mutation

import (
	"encoding/json"
	"os"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func setupBenchManager(b *testing.B) *SidecarConfigManager {
	b.Helper()
	f, err := os.CreateTemp(b.TempDir(), "sidecar-bench-*.yaml")
	if err != nil {
		b.Fatal(err)
	}
	const cfg = `
sidecars:
  - name: security-agent
    image: falcosecurity/falco-no-driver:0.38.0
    args: ["/usr/bin/falco", "-A", "-K", "/var/run/secrets/kubernetes.io/serviceaccount/token"]
    securityContext:
      privileged: true
    resources:
      requests:
        cpu: 100m
        memory: 128Mi
      limits:
        cpu: 500m
        memory: 512Mi
`
	if _, err := f.WriteString(cfg); err != nil {
		b.Fatal(err)
	}
	f.Close()

	mgr, err := NewSidecarConfigManager(f.Name())
	if err != nil {
		b.Fatal(err)
	}
	return mgr
}

func buildAdmissionReview(containers int) *admissionv1.AdmissionReview {
	ctrs := make([]corev1.Container, containers)
	for i := range ctrs {
		ctrs[i] = corev1.Container{
			Name:  "app",
			Image: "nginx:1.25",
		}
	}
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "bench-pod",
			Namespace:   "default",
			Annotations: map[string]string{AnnotationInject: "true"},
		},
		Spec: corev1.PodSpec{Containers: ctrs},
	}
	raw, _ := json.Marshal(pod)
	return &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID:    "bench-uid",
			Object: runtime.RawExtension{Raw: raw},
		},
	}
}

func BenchmarkMutatePod_SingleContainer(b *testing.B) {
	mgr := setupBenchManager(b)
	ar := buildAdmissionReview(1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MutatePod(ar, mgr)
	}
}

func BenchmarkMutatePod_TenContainers(b *testing.B) {
	mgr := setupBenchManager(b)
	ar := buildAdmissionReview(10)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		MutatePod(ar, mgr)
	}
}

func BenchmarkSidecarConfigManager_GetTemplates(b *testing.B) {
	mgr := setupBenchManager(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.GetTemplates()
	}
}

func BenchmarkSidecarConfigManager_Reload(b *testing.B) {
	mgr := setupBenchManager(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := mgr.Reload(); err != nil {
			b.Fatal(err)
		}
	}
}
