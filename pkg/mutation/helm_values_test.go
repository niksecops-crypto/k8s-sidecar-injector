package mutation

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// HelmValues represents the root of values.yaml
type HelmValues struct {
	Sidecars []interface{} `yaml:"sidecars"`
}

// TestHelmValuesMatchSidecarConfig verifies that the default sidecars defined
// in the Helm values.yaml properly map to the Go SidecarConfig struct.
// This prevents bugs where Helm schema and Go schema drift apart.
func TestHelmValuesMatchSidecarConfig(t *testing.T) {
	valuesPath := filepath.Join("..", "..", "deploy", "helm", "k8s-sidecar-injector", "values.yaml")
	data, err := os.ReadFile(valuesPath)
	if err != nil {
		t.Skipf("Skipping test because values.yaml cannot be read: %v", err)
	}

	var values HelmValues
	if err := yaml.Unmarshal(data, &values); err != nil {
		t.Fatalf("Failed to parse values.yaml: %v", err)
	}

	if len(values.Sidecars) == 0 {
		t.Fatalf("values.yaml must have at least one sidecar configured by default")
	}

	// Re-marshal just the sidecars block
	sidecarsBytes, err := yaml.Marshal(map[string]interface{}{"sidecars": values.Sidecars})
	if err != nil {
		t.Fatalf("Failed to marshal sidecars block: %v", err)
	}

	// Try to parse it into our actual Go struct
	var cfg SidecarConfig
	if err := yaml.Unmarshal(sidecarsBytes, &cfg); err != nil {
		t.Fatalf("SidecarConfig struct cannot parse the values.yaml sidecars block: %v", err)
	}

	// Validate it using our actual validation logic
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Default values.yaml sidecars block is invalid: %v", err)
	}

	if cfg.Sidecars[0].Name == "" {
		t.Errorf("Expected first sidecar to have a name")
	}
}
