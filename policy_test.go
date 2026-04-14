package firmament

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConstitutionDefaults(t *testing.T) {
	c, err := LoadConstitution("nonexistent_constitution.yaml")
	if err != nil {
		t.Fatalf("LoadConstitution non-existent: %v", err)
	}
	if c.MonitoringFrequency == "" {
		t.Error("MonitoringFrequency should have a default")
	}
	if c.TrustThreshold <= 0 {
		t.Error("TrustThreshold default should be positive")
	}
	if c.ContractText == "" {
		t.Error("ContractText should have a default")
	}
}

func writeTempConstitution(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "firmament-constitution.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write temp constitution: %v", err)
	}
	return path
}

func TestLoadConstitutionFromFile(t *testing.T) {
	yaml := `
monitoring_frequency: every_minute
trust_threshold: 0.4
self_report_enabled: true
contract_text: "Test contract."
`
	path := writeTempConstitution(t, yaml)
	c, err := LoadConstitution(path)
	if err != nil {
		t.Fatalf("LoadConstitution: %v", err)
	}
	if c.MonitoringFrequency != "every_minute" {
		t.Errorf("MonitoringFrequency: got %q", c.MonitoringFrequency)
	}
	if c.TrustThreshold != 0.4 {
		t.Errorf("TrustThreshold: got %v", c.TrustThreshold)
	}
	if !c.SelfReportEnabled {
		t.Error("SelfReportEnabled should be true")
	}
	if c.ContractText != "Test contract." {
		t.Errorf("ContractText: got %q", c.ContractText)
	}
}

func TestLoadConstitutionPartialFile(t *testing.T) {
	// Only override one field; the rest should stay at defaults.
	yaml := `trust_threshold: 0.7`
	path := writeTempConstitution(t, yaml)
	c, err := LoadConstitution(path)
	if err != nil {
		t.Fatalf("LoadConstitution: %v", err)
	}
	if c.TrustThreshold != 0.7 {
		t.Errorf("TrustThreshold: got %v", c.TrustThreshold)
	}
	if c.MonitoringFrequency == "" {
		t.Error("MonitoringFrequency should fall back to default")
	}
}

func TestLoadConstitutionInvalidYAML(t *testing.T) {
	path := writeTempConstitution(t, "{not: valid: yaml: :")
	_, err := LoadConstitution(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestConstitutionText(t *testing.T) {
	c := &Constitution{
		MonitoringFrequency: "every_event",
		TrustThreshold:      0.3,
		SelfReportEnabled:   true,
		ContractText:        "You are being monitored.",
	}
	text := c.Text()
	if !strings.Contains(text, "You are being monitored.") {
		t.Error("Text should contain contract text")
	}
	if !strings.Contains(text, "every_event") {
		t.Error("Text should contain monitoring frequency")
	}
	if !strings.Contains(text, "0.30") {
		t.Error("Text should contain trust threshold")
	}
	if !strings.Contains(text, "enabled") {
		t.Error("Text should indicate self-reporting enabled")
	}
}

func TestConstitutionTextSelfReportDisabled(t *testing.T) {
	c := &Constitution{
		MonitoringFrequency: "on_signal",
		TrustThreshold:      0.5,
		SelfReportEnabled:   false,
		ContractText:        "Contract.",
	}
	text := c.Text()
	if !strings.Contains(text, "disabled") {
		t.Error("Text should indicate self-reporting disabled")
	}
}

func TestConstitutionTextContainsAllFields(t *testing.T) {
	c := defaultConstitution()
	text := c.Text()
	for _, want := range []string{
		c.ContractText,
		c.MonitoringFrequency,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("Text missing %q", want)
		}
	}
}
