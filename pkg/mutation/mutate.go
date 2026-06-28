package mutation

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
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

// Validate ensures the parsed configuration is semantically valid.
func (cfg *SidecarConfig) Validate() error {
	if len(cfg.Sidecars) == 0 {
		return errors.New("no sidecars configured")
	}
	seen := make(map[string]bool)
	for _, s := range cfg.Sidecars {
		if s.Name == "" {
			return errors.New("container name cannot be empty")
		}
		if s.Image == "" {
			return fmt.Errorf("container image cannot be empty for sidecar %q", s.Name)
		}
		if seen[s.Name] {
			return fmt.Errorf("duplicate sidecar name %q found in configuration", s.Name)
		}
		seen[s.Name] = true
	}
	return nil
}

// SidecarConfigManager manages the sidecar configuration with support
// for hot-reload via SIGHUP without downtime.
type SidecarConfigManager struct {
	mu         sync.RWMutex
	config     SidecarConfig
	configPath string
}

var (
	mutationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "sidecar_injector_mutations_total",
		Help: "Total number of pod mutations processed",
	}, []string{"status", "namespace"})

	mutationDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "sidecar_injector_mutation_duration_seconds",
		Help:    "Duration of the mutation process",
		Buckets: prometheus.DefBuckets,
	})
)

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

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid sidecar config: %w", err)
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
// Sidecars are injected as Native Sidecars (initContainers with restartPolicy: Always).
func MutatePod(ar *admissionv1.AdmissionReview, mgr *SidecarConfigManager) *admissionv1.AdmissionResponse {
	start := time.Now()
	req := ar.Request

	recordMetrics := func(status string) {
		mutationsTotal.WithLabelValues(status, req.Namespace).Inc()
		mutationDuration.Observe(time.Since(start).Seconds())
	}

	var pod corev1.Pod
	if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
		slog.Error("could not unmarshal pod object", "uid", req.UID, "error", err)
		recordMetrics("error")
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
		recordMetrics("skipped")
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	templates := mgr.GetTemplates()

	existing := make(map[string]bool)
	for _, c := range pod.Spec.Containers {
		existing[c.Name] = true
	}
	for _, c := range pod.Spec.InitContainers {
		existing[c.Name] = true
	}

	var patch []PatchOperation
	for _, tmpl := range templates {
		if existing[tmpl.Name] {
			slog.Info("sidecar already present, skipping", "sidecar", tmpl.Name, "pod", pod.Name)
			continue
		}

		// Enforce Native Sidecar properties (Kubernetes 1.28+)
		if tmpl.RestartPolicy == nil || *tmpl.RestartPolicy != corev1.ContainerRestartPolicyAlways {
			always := corev1.ContainerRestartPolicyAlways
			tmpl.RestartPolicy = &always
		}

		if len(pod.Spec.InitContainers) == 0 && len(patch) == 0 {
			patch = append(patch, PatchOperation{
				Op:    "add",
				Path:  "/spec/initContainers",
				Value: []corev1.Container{tmpl},
			})
		} else {
			patch = append(patch, PatchOperation{
				Op:    "add",
				Path:  "/spec/initContainers/-",
				Value: tmpl,
			})
		}
	}

	if len(patch) == 0 {
		slog.Info("all sidecars already present, no mutations needed", "pod", pod.Name)
		recordMetrics("skipped")
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		recordMetrics("error")
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{Message: err.Error()},
		}
	}

	pt := admissionv1.PatchTypeJSONPatch
	slog.Info("injecting sidecars", "pod", pod.Name, "namespace", req.Namespace, "patches", len(patch))
	recordMetrics("mutated")
	return &admissionv1.AdmissionResponse{
		Allowed:   true,
		Patch:     patchBytes,
		PatchType: &pt,
	}
}
