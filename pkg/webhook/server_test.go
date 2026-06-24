package webhook

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"k8s-sidecar-injector/pkg/mutation"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestHandleMutate(t *testing.T) {
	// Setup temporary sidecar config
	tmpfile, _ := os.CreateTemp("", "sidecar*.yaml")
	defer os.Remove(tmpfile.Name())
	if err := os.WriteFile(tmpfile.Name(), []byte("name: sidecar\nimage: nginx"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr, _ := mutation.NewSidecarConfigManager(tmpfile.Name())
	server := &Server{ConfigManager: mgr}

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "main", Image: "alpine"}},
		},
	}
	podBytes, _ := json.Marshal(pod)

	ar := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
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
	arBytes, _ := json.Marshal(ar)

	req := httptest.NewRequest("POST", "/mutate", bytes.NewBuffer(arBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.HandleMutate(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var finalReview admissionv1.AdmissionReview
	if err := json.Unmarshal(body, &finalReview); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if finalReview.Response == nil || !finalReview.Response.Allowed {
		t.Errorf("Expected response allowed true")
	}

	if finalReview.Response.UID != "test-uid" {
		t.Errorf("Expected UID test-uid, got %v", finalReview.Response.UID)
	}
}

func TestHandleMutate_ErrorPaths(t *testing.T) {
	server := &Server{}

	// Test Empty Body
	req := httptest.NewRequest("POST", "/mutate", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.HandleMutate(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for empty body")
	}

	// Test Wrong Content-Type
	req = httptest.NewRequest("POST", "/mutate", bytes.NewBuffer([]byte("{}")))
	req.Header.Set("Content-Type", "text/plain")
	w = httptest.NewRecorder()
	server.HandleMutate(w, req)
	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("Expected 415 for wrong content type")
	}

	// Test Invalid JSON
	req = httptest.NewRequest("POST", "/mutate", bytes.NewBuffer([]byte("invalid-json")))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	server.HandleMutate(w, req)
	if w.Code != http.StatusOK { // Admission webhook returns 200 with error in response
		t.Errorf("Expected 200 for decode error")
	}
}

func TestHandleHealthz(t *testing.T) {
	server := &Server{}
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	server.HandleHealthz(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %v", w.Code)
	}
}

func TestHandleReadyz(t *testing.T) {
	server := &Server{}
	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	server.HandleReadyz(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %v", w.Code)
	}
}

func TestHandleMetrics(t *testing.T) {
	server := &Server{}
	handler := server.HandleMetrics()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status OK, got %v", w.Code)
	}
	
	body, _ := io.ReadAll(w.Body)
	if len(body) == 0 {
		t.Errorf("Expected non-empty metrics body")
	}
}
