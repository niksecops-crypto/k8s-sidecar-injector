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

func TestMutatePod(t *testing.T) {
	// Create a temporary sidecar config file
	tmpfile, err := os.CreateTemp("", "sidecar*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	content := `
name: security-agent
image: falcosecurity/falco-no-driver:latest
args: ["/usr/bin/falco", "-A"]
securityContext:
  privileged: true
`
	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	mgr, err := NewSidecarConfigManager(tmpfile.Name())
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "app-container",
					Image: "nginx:latest",
				},
			},
		},
	}

	podBytes, _ := json.Marshal(pod)

	ar := &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID: "test-uid",
			Kind: metav1.GroupVersionKind{
				Group:   "",
				Version: "v1",
				Kind:    "Pod",
			},
			Object: runtime.RawExtension{
				Raw: podBytes,
			},
		},
	}

	response := MutatePod(ar, mgr)

	if !response.Allowed {
		t.Errorf("Expected allowed true, got false")
	}

	if response.PatchType == nil || *response.PatchType != admissionv1.PatchTypeJSONPatch {
		t.Errorf("Expected patch type JSONPatch")
	}

	var patches []PatchOperation
	if err := json.Unmarshal(response.Patch, &patches); err != nil {
		t.Fatalf("Could not unmarshal patch: %v", err)
	}

	if len(patches) != 1 {
		t.Errorf("Expected 1 patch operation, got %d", len(patches))
	}

	patch := patches[0]
	if patch.Op != "add" {
		t.Errorf("Expected op add, got %s", patch.Op)
	}

	if patch.Path != "/spec/containers/-" {
		t.Errorf("Expected path /spec/containers/-, got %s", patch.Path)
	}

	value, ok := patch.Value.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected value to be a map")
	}

	if value["name"] != "security-agent" {
		t.Errorf("Expected name security-agent, got %v", value["name"])
	}
}

func TestMutatePod_SkipIfAlreadyInjected(t *testing.T) {
	tmpfile, _ := os.CreateTemp("", "sidecar*.yaml")
	defer os.Remove(tmpfile.Name())
	if err := os.WriteFile(tmpfile.Name(), []byte("name: security-agent\nimage: some-image"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr, _ := NewSidecarConfigManager(tmpfile.Name())

	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "security-agent",
					Image: "some-image",
				},
			},
		},
	}

	podBytes, _ := json.Marshal(pod)

	ar := &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID: "test-uid",
			Object: runtime.RawExtension{
				Raw: podBytes,
			},
		},
	}

	response := MutatePod(ar, mgr)

	if !response.Allowed {
		t.Errorf("Expected allowed true")
	}

	if response.Patch != nil {
		t.Errorf("Expected no patch if already injected")
	}
}

func TestMutatePod_EmptyContainers(t *testing.T) {
	tmpfile, _ := os.CreateTemp("", "sidecar*.yaml")
	defer os.Remove(tmpfile.Name())
	if err := os.WriteFile(tmpfile.Name(), []byte("name: sidecar\nimage: nginx"), 0644); err != nil {
		t.Fatal(err)
	}
	mgr, _ := NewSidecarConfigManager(tmpfile.Name())

	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{},
		},
	}
	podBytes, _ := json.Marshal(pod)
	ar := &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{Raw: podBytes},
		},
	}

	response := MutatePod(ar, mgr)
	if !response.Allowed {
		t.Errorf("Expected allowed true")
	}
	
	var patches []PatchOperation
	if err := json.Unmarshal(response.Patch, &patches); err != nil {
		t.Fatalf("Failed to unmarshal patch: %v", err)
	}
	if patches[0].Path != "/spec/containers" {
		t.Errorf("Expected path /spec/containers for empty pod, got %s", patches[0].Path)
	}
}

func TestSidecarConfigManager_ErrorPaths(t *testing.T) {
	// Test non-existent file
	_, err := NewSidecarConfigManager("/non/existent/path")
	if err == nil {
		t.Errorf("Expected error for non-existent file")
	}

	// Test invalid YAML
	tmpfile, _ := os.CreateTemp("", "invalid*.yaml")
	defer os.Remove(tmpfile.Name())
	if err := os.WriteFile(tmpfile.Name(), []byte("invalid: yaml: :"), 0644); err != nil {
		t.Fatal(err)
	}
	
	_, err = NewSidecarConfigManager(tmpfile.Name())
	if err == nil {
		t.Errorf("Expected error for invalid YAML")
	}
}

func TestMutatePod_InvalidJson(t *testing.T) {
	tmpfile, _ := os.CreateTemp("", "sidecar*.yaml")
	defer os.Remove(tmpfile.Name())
	os.WriteFile(tmpfile.Name(), []byte("name: sidecar\nimage: nginx"), 0644)
	mgr, _ := NewSidecarConfigManager(tmpfile.Name())
	
	ar := &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{Raw: []byte("invalid-json")},
		},
	}
	response := MutatePod(ar, mgr)
	if response.Allowed {
		t.Errorf("Expected allowed false for invalid JSON")
	}
}

func TestSidecarConfigManager_Reload_Success(t *testing.T) {
	tmpfile, _ := os.CreateTemp("", "reload*.yaml")
	defer os.Remove(tmpfile.Name())
	os.WriteFile(tmpfile.Name(), []byte("name: initial\nimage: nginx"), 0644)

	mgr, err := NewSidecarConfigManager(tmpfile.Name())
	if err != nil {
		t.Fatal(err)
	}

	if mgr.GetTemplate().Name != "initial" {
		t.Errorf("Expected initial name")
	}

	// Update file and reload
	os.WriteFile(tmpfile.Name(), []byte("name: updated\nimage: alpine"), 0644)
	if err := mgr.Reload(); err != nil {
		t.Errorf("Failed to reload: %v", err)
	}

	if mgr.GetTemplate().Name != "updated" {
		t.Errorf("Expected updated name")
	}
}

func TestMutatePod_MultipleContainers(t *testing.T) {
	tmpfile, _ := os.CreateTemp("", "multi*.yaml")
	defer os.Remove(tmpfile.Name())
	os.WriteFile(tmpfile.Name(), []byte("name: sidecar\nimage: nginx"), 0644)
	mgr, _ := NewSidecarConfigManager(tmpfile.Name())

	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app1", Image: "img1"},
				{Name: "app2", Image: "img2"},
			},
		},
	}
	podBytes, _ := json.Marshal(pod)
	ar := &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{Raw: podBytes},
		},
	}

	response := MutatePod(ar, mgr)
	if !response.Allowed {
		t.Errorf("Expected allowed true")
	}

	var patches []PatchOperation
	json.Unmarshal(response.Patch, &patches)
	if patches[0].Path != "/spec/containers/-" {
		t.Errorf("Expected path /spec/containers/- for non-empty pod, got %s", patches[0].Path)
	}
}
