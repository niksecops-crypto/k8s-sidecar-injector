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

// AnnotationInject is the pod annotation that opts a pod into sidecar injection.
// Set to "true" to enable: sidecar-injector.io/inject: "true"
const AnnotationInject = "sidecar-injector.io/inject"

// PatchOperation represents a single JSON Patch operation (RFC 6902).
type PatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// SidecarConfig is the schema for the sidecar configuration file.
// Each entry under 'sidecars' is a full Kubernetes Container spec.
type SidecarConfig struct {
	Sidecars []corev1.Container `yaml:"sidecars"`
}

// SidecarConfigManager manages the sidecar configuration with support
// for hot-reload via SIGHUP without downtime.
type SidecarConfigManager struct {
	mu         sync.RWMutex
	config     SidecarConfig
	configPath string
}

// NewSidecarConfigManager creates a manager and performs an initial load.
func NewSidecarConfigManager(path string) (*SidecarConfigManager, error) {
	mgr := &SidecarConfigManager{configPath: path}
	if err := mgr.Reload(); err != nil {
		return nil, err
	}
	return mgr, nil
}

// Reload re-reads the config file and atomically replaces the in-memory config.
// Safe to call concurrently; used by SIGHUP handler.
func (m *SidecarConfigManager) Reload() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("read sidecar config %q: %w", m.configPath, err)
	}

	var cfg SidecarConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("unmarshal sidecar config: %w", err)
	}

	if len(cfg.Sidecars) == 0 {
		return fmt.Errorf("sidecar config at %q has no entries under 'sidecars'", m.configPath)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg

	names := make([]string, 0, len(cfg.Sidecars))
	for _, s := range cfg.Sidecars {
		names = append(names, s.Name)
	}
	slog.Info("sidecar config reloaded", "count", len(cfg.Sidecars), "names", names)
	return nil
}

// GetTemplates returns a snapshot of the currently configured sidecars.
// The returned slice is safe to use after the lock is released.
func (m *SidecarConfigManager) GetTemplates() []corev1.Container {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]corev1.Container, len(m.config.Sidecars))
	copy(result, m.config.Sidecars)
	return result
}

// MutatePod processes an AdmissionReview and injects configured sidecars into
// pods that carry the annotation sidecar-injector.io/inject: "true".
//
// Sidecars that are already present (matched by container name) are skipped,
// making the operation idempotent across re-schedules.
func MutatePod(ar *admissionv1.AdmissionReview, mgr *SidecarConfigManager) *admissionv1.AdmissionResponse {
	req := ar.Request

	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		slog.Error("could not unmarshal pod object", "uid", req.UID, "error", err)
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{Message: err.Error()},
		}
	}

	slog.Info("admission review",
		"namespace", req.Namespace,
		"pod", pod.Name,
		"uid", req.UID,
	)

	if pod.Annotations[AnnotationInject] != "true" {
		slog.Info("skipping: inject annotation absent or false",
			"pod", pod.Name,
			"namespace", req.Namespace,
			"annotation", AnnotationInject,
		)
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	templates := mgr.GetTemplates()

	existing := make(map[string]bool, len(pod.Spec.Containers))
	for _, c := range pod.Spec.Containers {
		existing[c.Name] = true
	}

	var patch []PatchOperation
	for _, tmpl := range templates {
		if existing[tmpl.Name] {
			slog.Info("sidecar already present, skipping", "sidecar", tmpl.Name, "pod", pod.Name)
			continue
		}

		if len(pod.Spec.Containers) == 0 && len(patch) == 0 {
			// Pod has no containers yet: initialize the array instead of appending.
			patch = append(patch, PatchOperation{
				Op:    "add",
				Path:  "/spec/containers",
				Value: []corev1.Container{tmpl},
			})
		} else {
			patch = append(patch, PatchOperation{
				Op:    "add",
				Path:  "/spec/containers/-",
				Value: tmpl,
			})
		}
	}

	if len(patch) == 0 {
		slog.Info("all sidecars already present, no mutations needed", "pod", pod.Name)
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{Message: err.Error()},
		}
	}

	pt := admissionv1.PatchTypeJSONPatch
	slog.Info("injecting sidecars", "pod", pod.Name, "namespace", req.Namespace, "patches", len(patch))
	return &admissionv1.AdmissionResponse{
		Allowed:   true,
		Patch:     patchBytes,
		PatchType: &pt,
	}
}
