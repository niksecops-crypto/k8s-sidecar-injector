package mutation

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"gopkg.in/yaml.v3"
)

// PatchOperation represents a JSON patch operation.
type PatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// SidecarConfigManager handles dynamic loading of sidecar configuration
type SidecarConfigManager struct {
	mu             sync.RWMutex
	sidecarTemplate *corev1.Container
	configPath     string
}

func NewSidecarConfigManager(path string) (*SidecarConfigManager, error) {
	mgr := &SidecarConfigManager{configPath: path}
	if err := mgr.Reload(); err != nil {
		return nil, err
	}
	return mgr, nil
}

func (m *SidecarConfigManager) Reload() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to read sidecar config: %w", err)
	}

	var container corev1.Container
	if err := yaml.Unmarshal(data, &container); err != nil {
		return fmt.Errorf("failed to unmarshal sidecar yaml: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.sidecarTemplate = &container
	slog.Info("Sidecar configuration reloaded successfully", "name", container.Name, "image", container.Image)
	return nil
}

func (m *SidecarConfigManager) GetTemplate() corev1.Container {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return *m.sidecarTemplate
}

// MutatePod handles the mutation logic for a Pod using dynamic configuration.
func MutatePod(ar *admissionv1.AdmissionReview, mgr *SidecarConfigManager) *admissionv1.AdmissionResponse {
	req := ar.Request
	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		slog.Error("Could not unmarshal pod object", "error", err, "uid", req.UID)
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	config := mgr.GetTemplate()

	slog.Info("AdmissionReview",
		"kind", req.Kind,
		"namespace", req.Namespace,
		"pod_name", pod.Name,
		"uid", req.UID,
	)

	// Check if the sidecar is already injected
	for _, container := range pod.Spec.Containers {
		if container.Name == config.Name {
			slog.Info("Sidecar already injected, skipping", "pod", pod.Name, "namespace", req.Namespace)
			return &admissionv1.AdmissionResponse{
				Allowed: true,
			}
		}
	}

	// Create JSON patch
	var patch []PatchOperation
	path := "/spec/containers/-"
	if len(pod.Spec.Containers) == 0 {
		path = "/spec/containers"
		patch = append(patch, PatchOperation{
			Op:    "add",
			Path:  path,
			Value: []corev1.Container{config},
		})
	} else {
		patch = append(patch, PatchOperation{
			Op:    "add",
			Path:  path,
			Value: config,
		})
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}
