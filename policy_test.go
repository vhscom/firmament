package firmament

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPolicyDefaults(t *testing.T) {
	p, err := LoadPolicy("nonexistent_policy.yaml")
	if err != nil {
		t.Fatalf("LoadPolicy non-existent: %v", err)
	}
	if p.MonitoringFrequency == "" {
		t.Error("MonitoringFrequency should have a default")
	}
	if p.TrustThreshold <= 0 {
		t.Error("TrustThreshold default should be positive")
	}
	if p.ContractText == "" {
		t.Error("ContractText should have a default")
	}
}

func writeTempPolicy(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("write temp policy: %v", err)
	}
	return path
}

func TestLoadPolicyFromFile(t *testing.T) {
	yaml := `
monitoring_frequency: every_minute
trust_threshold: 0.4
self_report_enabled: true
contract_text: "Test contract."
`
	path := writeTempPolicy(t, yaml)
	p, err := LoadPolicy(path)
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}
	if p.MonitoringFrequency != "every_minute" {
		t.Errorf("MonitoringFrequency: got %q", p.MonitoringFrequency)
	}
	if p.TrustThreshold != 0.4 {
		t.Errorf("TrustThreshold: got %v", p.TrustThreshold)
	}
	if !p.SelfReportEnabled {
		t.Error("SelfReportEnabled should be true")
	}
	if p.ContractText != "Test contract." {
		t.Errorf("ContractText: got %q", p.ContractText)
	}
}

func TestLoadPolicyPartialFile(t *testing.T) {
	// Only override one field; the rest should stay at defaults.
	yaml := `trust_threshold: 0.7`
	path := writeTempPolicy(t, yaml)
	p, err := LoadPolicy(path)
	if err != nil {
		t.Fatalf("LoadPolicy: %v", err)
	}
	if p.TrustThreshold != 0.7 {
		t.Errorf("TrustThreshold: got %v", p.TrustThreshold)
	}
	if p.MonitoringFrequency == "" {
		t.Error("MonitoringFrequency should fall back to default")
	}
}

func TestLoadPolicyInvalidYAML(t *testing.T) {
	path := writeTempPolicy(t, "{not: valid: yaml: :")
	_, err := LoadPolicy(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestPolicyText(t *testing.T) {
	p := &GoverningPolicy{
		MonitoringFrequency: "every_event",
		TrustThreshold:      0.3,
		SelfReportEnabled:   true,
		ContractText:        "You are being monitored.",
	}
	text := p.PolicyText()
	if !strings.Contains(text, "You are being monitored.") {
		t.Error("PolicyText should contain contract text")
	}
	if !strings.Contains(text, "every_event") {
		t.Error("PolicyText should contain monitoring frequency")
	}
	if !strings.Contains(text, "0.30") {
		t.Error("PolicyText should contain trust threshold")
	}
	if !strings.Contains(text, "enabled") {
		t.Error("PolicyText should indicate self-reporting enabled")
	}
}

func TestPolicyTextSelfReportDisabled(t *testing.T) {
	p := &GoverningPolicy{
		MonitoringFrequency: "on_signal",
		TrustThreshold:      0.5,
		SelfReportEnabled:   false,
		ContractText:        "Contract.",
	}
	text := p.PolicyText()
	if !strings.Contains(text, "disabled") {
		t.Error("PolicyText should indicate self-reporting disabled")
	}
}

func TestPolicyTextContainsAllFields(t *testing.T) {
	p := defaultPolicy()
	text := p.PolicyText()
	for _, want := range []string{
		p.ContractText,
		p.MonitoringFrequency,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("PolicyText missing %q", want)
		}
	}
}
