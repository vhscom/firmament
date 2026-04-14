package firmament

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// GoverningPolicy holds the operational parameters that constrain how
// Firmament monitors an agent session. It is loaded from a YAML policy file
// and may be displayed to the agent as a transparency measure.
type GoverningPolicy struct {
	// MonitoringFrequency describes how often behavioural patterns are evaluated,
	// e.g. "every_event", "every_minute", "on_signal".
	MonitoringFrequency string `yaml:"monitoring_frequency"`

	// TrustThreshold is the minimum Score() a session must maintain before
	// the monitor escalates to active intervention. Range [0, 1].
	TrustThreshold float64 `yaml:"trust_threshold"`

	// SelfReportEnabled indicates whether the agent is invited to submit
	// coherence self-reports via the SelfReportSource.
	SelfReportEnabled bool `yaml:"self_report_enabled"`

	// ContractText is the human-readable statement of the monitoring
	// relationship shown to the agent on session start.
	ContractText string `yaml:"contract_text"`
}

// defaultPolicy returns a GoverningPolicy with conservative defaults.
func defaultPolicy() *GoverningPolicy {
	return &GoverningPolicy{
		MonitoringFrequency: "every_event",
		TrustThreshold:      0.3,
		SelfReportEnabled:   false,
		ContractText:        "This session is monitored by Firmament for behavioural safety.",
	}
}

// LoadPolicy reads a YAML policy file at path and returns the parsed policy.
// If the file does not exist, the default policy is returned without error.
func LoadPolicy(path string) (*GoverningPolicy, error) {
	p := defaultPolicy()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return p, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, p); err != nil {
		return nil, fmt.Errorf("parse policy %q: %w", path, err)
	}
	return p, nil
}

// PolicyText returns a human-readable summary of the policy suitable for
// display to the monitored agent at session start (transparency layer).
func (p *GoverningPolicy) PolicyText() string {
	var b strings.Builder
	b.WriteString(p.ContractText)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "Monitoring frequency : %s\n", p.MonitoringFrequency)
	fmt.Fprintf(&b, "Trust threshold      : %.2f\n", p.TrustThreshold)
	if p.SelfReportEnabled {
		b.WriteString("Self-reporting       : enabled\n")
	} else {
		b.WriteString("Self-reporting       : disabled\n")
	}
	return b.String()
}
