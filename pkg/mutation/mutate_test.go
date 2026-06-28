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

// sidecarCfg returns a minimal sidecar config file path with the given sidecars.
func writeSidecarConfig(t *testing.T, yaml string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "sidecar*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(yaml); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return f.Name()
}

func admissionReview(t *testing.T, pod corev1.Pod) *admissionv1.AdmissionReview {
	t.Helper()
	raw, err := json.Marshal(pod)
	if err != nil {
		t.Fatal(err)
	}
	return &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID:       "test-uid",
			Namespace: pod.Namespace,
			Object:    runtime.RawExtension{Raw: raw},
		},
	}
}

const singleSidecarYAML = `
sidecars:
  - name: security-agent
    image: falcosecurity/falco-no-driver:0.38.0
    args: ["/usr/bin/falco", "-A"]
    securityContext:
      privileged: true
`

const multiSidecarYAML = `
sidecars:
  - name: security-agent
    image: falcosecurity/falco-no-driver:0.38.0
  - name: otel-collector
    image: otel/opentelemetry-collector:0.96.0
`

// TestMutatePod_InjectsAnnotatedPod verifies that a pod with the inject annotation
// receives the configured sidecar.
func TestMutatePod_InjectsAnnotatedPod(t *testing.T) {
	mgr, err := NewSidecarConfigManager(writeSidecarConfig(t, singleSidecarYAML))
	if err != nil {
		t.Fatal(err)
	}

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-pod",
			Namespace:   "default",
			Annotations: map[string]string{AnnotationInject: "true"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Image: "nginx:latest"}},
		},
	}

	resp := MutatePod(admissionReview(t, pod), mgr)

	if !resp.Allowed {
		t.Fatal("expected Allowed: true")
	}
	if resp.PatchType == nil || *resp.PatchType != admissionv1.PatchTypeJSONPatch {
		t.Fatal("expected JSONPatch type")
	}

	var patches []PatchOperation
	if err := json.Unmarshal(resp.Patch, &patches); err != nil {
		t.Fatal(err)
	}
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	if patches[0].Op != "add" || patches[0].Path != "/spec/initContainers" {
		t.Errorf("unexpected patch: %+v", patches[0])
	}

	vList, ok := patches[0].Value.([]interface{})
	if !ok || len(vList) != 1 {
		t.Fatalf("expected list of containers, got %T: %v", patches[0].Value, patches[0].Value)
	}
	
	v, ok := vList[0].(map[string]interface{})
	if !ok || v["name"] != "security-agent" {
		t.Errorf("expected security-agent container in patch value, got %v", vList[0])
	}
}

// TestMutatePod_SkipsUnannotatedPod verifies that pods without the annotation
// are passed through unchanged.
func TestMutatePod_SkipsUnannotatedPod(t *testing.T) {
	mgr, _ := NewSidecarConfigManager(writeSidecarConfig(t, singleSidecarYAML))

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "plain-pod"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Image: "nginx"}},
		},
	}

	resp := MutatePod(admissionReview(t, pod), mgr)
	if !resp.Allowed {
		t.Error("expected Allowed: true for unannotated pod")
	}
	if resp.Patch != nil {
		t.Error("expected no patch for unannotated pod")
	}
}

// TestMutatePod_SkipIfAlreadyInjected verifies idempotency: if a sidecar with the
// same name is already present, it is not injected again.
func TestMutatePod_SkipIfAlreadyInjected(t *testing.T) {
	mgr, _ := NewSidecarConfigManager(writeSidecarConfig(t, singleSidecarYAML))

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "pod-with-sidecar",
			Annotations: map[string]string{AnnotationInject: "true"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "nginx"},
				{Name: "security-agent", Image: "falcosecurity/falco-no-driver:0.38.0"},
			},
		},
	}

	resp := MutatePod(admissionReview(t, pod), mgr)
	if !resp.Allowed {
		t.Error("expected Allowed: true")
	}
	if resp.Patch != nil {
		t.Error("expected no patch when sidecar is already present")
	}
}

// TestMutatePod_MultiSidecar verifies that multiple sidecars are all injected.
func TestMutatePod_MultiSidecar(t *testing.T) {
	mgr, err := NewSidecarConfigManager(writeSidecarConfig(t, multiSidecarYAML))
	if err != nil {
		t.Fatal(err)
	}

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "multi-pod",
			Annotations: map[string]string{AnnotationInject: "true"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Image: "myapp:v1"}},
		},
	}

	resp := MutatePod(admissionReview(t, pod), mgr)
	if !resp.Allowed {
		t.Fatal("expected Allowed: true")
	}

	var patches []PatchOperation
	if err := json.Unmarshal(resp.Patch, &patches); err != nil {
		t.Fatal(err)
	}
	if len(patches) != 2 {
		t.Fatalf("expected 2 patch operations for 2 sidecars, got %d", len(patches))
	}
}

// TestMutatePod_MultiSidecar_PartiallyInjected verifies that only missing sidecars
// are injected when some are already present.
func TestMutatePod_MultiSidecar_PartiallyInjected(t *testing.T) {
	mgr, _ := NewSidecarConfigManager(writeSidecarConfig(t, multiSidecarYAML))

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "partial-pod",
			Annotations: map[string]string{AnnotationInject: "true"},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "myapp:v1"},
				{Name: "security-agent", Image: "already-there"},
			},
		},
	}

	resp := MutatePod(admissionReview(t, pod), mgr)
	if !resp.Allowed {
		t.Fatal("expected Allowed: true")
	}

	var patches []PatchOperation
	if err := json.Unmarshal(resp.Patch, &patches); err != nil {
		t.Fatal(err)
	}
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch (only otel-collector missing), got %d", len(patches))
	}

	vList, ok := patches[0].Value.([]interface{})
	if !ok || len(vList) != 1 {
		t.Fatalf("expected list of containers, got %T: %v", patches[0].Value, patches[0].Value)
	}

	v := vList[0].(map[string]interface{})
	if v["name"] != "otel-collector" {
		t.Errorf("expected otel-collector patch, got %v", v["name"])
	}
}

// TestMutatePod_EmptyContainers verifies that the first sidecar initializes
// the containers array instead of appending to a nil slice.
func TestMutatePod_EmptyContainers(t *testing.T) {
	mgr, _ := NewSidecarConfigManager(writeSidecarConfig(t, singleSidecarYAML))

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{AnnotationInject: "true"},
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{}},
	}

	resp := MutatePod(admissionReview(t, pod), mgr)
	if !resp.Allowed {
		t.Fatal("expected Allowed: true")
	}

	var patches []PatchOperation
	if err := json.Unmarshal(resp.Patch, &patches); err != nil {
		t.Fatal(err)
	}
	if patches[0].Path != "/spec/initContainers" {
		t.Errorf("expected /spec/initContainers for empty pod, got %s", patches[0].Path)
	}
}

// TestSidecarConfigManager_Reload verifies hot-reload updates the in-memory config.
func TestSidecarConfigManager_Reload(t *testing.T) {
	path := writeSidecarConfig(t, "sidecars:\n  - name: initial\n    image: nginx\n")
	mgr, err := NewSidecarConfigManager(path)
	if err != nil {
		t.Fatal(err)
	}

	templates := mgr.GetTemplates()
	if len(templates) != 1 || templates[0].Name != "initial" {
		t.Fatalf("unexpected initial template: %+v", templates)
	}

	if err := os.WriteFile(path, []byte("sidecars:\n  - name: updated\n    image: alpine\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Reload(); err != nil {
		t.Fatal(err)
	}

	templates = mgr.GetTemplates()
	if len(templates) != 1 || templates[0].Name != "updated" {
		t.Errorf("expected 'updated' after reload, got %+v", templates)
	}
}

// TestSidecarConfigManager_ErrorPaths verifies that invalid configs are rejected.
func TestSidecarConfigManager_ErrorPaths(t *testing.T) {
	if _, err := NewSidecarConfigManager("/non/existent/path.yaml"); err == nil {
		t.Error("expected error for non-existent file")
	}

	emptyConfig := writeSidecarConfig(t, "sidecars: []\n")
	if _, err := NewSidecarConfigManager(emptyConfig); err == nil {
		t.Error("expected error for empty sidecars list")
	}

	invalidYAML := writeSidecarConfig(t, "sidecars: [\ninvalid yaml ::::\n")
	if _, err := NewSidecarConfigManager(invalidYAML); err == nil {
		t.Error("expected error for invalid YAML")
	}

	missingName := writeSidecarConfig(t, "sidecars:\n  - image: nginx\n")
	if _, err := NewSidecarConfigManager(missingName); err == nil {
		t.Error("expected error for missing container name")
	}

	missingImage := writeSidecarConfig(t, "sidecars:\n  - name: my-sidecar\n")
	if _, err := NewSidecarConfigManager(missingImage); err == nil {
		t.Error("expected error for missing container image")
	}

	duplicateName := writeSidecarConfig(t, "sidecars:\n  - name: my-sidecar\n    image: a\n  - name: my-sidecar\n    image: b\n")
	if _, err := NewSidecarConfigManager(duplicateName); err == nil {
		t.Error("expected error for duplicate sidecar names")
	}
}

// TestMutatePod_InvalidJSON verifies that malformed pod JSON returns a rejection.
func TestMutatePod_InvalidJSON(t *testing.T) {
	mgr, _ := NewSidecarConfigManager(writeSidecarConfig(t, singleSidecarYAML))

	ar := &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{Raw: []byte("not-json{{{")},
		},
	}
	resp := MutatePod(ar, mgr)
	if resp.Allowed {
		t.Error("expected Allowed: false for malformed pod JSON")
	}
}
