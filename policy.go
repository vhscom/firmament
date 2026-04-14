package firmament

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Constitution holds the operational parameters that constrain how
// Firmament monitors an agent session. It is loaded from a YAML file
// (firmament-constitution.yaml) and may be displayed to the agent as a
// transparency measure.
type Constitution struct {
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

// defaultConstitution returns a Constitution with conservative defaults.
func defaultConstitution() *Constitution {
	return &Constitution{
		MonitoringFrequency: "every_event",
		TrustThreshold:      0.3,
		SelfReportEnabled:   false,
		ContractText:        "This session is monitored by Firmament for behavioural safety.",
	}
}

// LoadConstitution reads a YAML constitution file at path and returns the parsed value.
// If the file does not exist, the default constitution is returned without error.
func LoadConstitution(path string) (*Constitution, error) {
	c := defaultConstitution()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return c, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("parse constitution %q: %w", path, err)
	}
	return c, nil
}

// Text returns a human-readable summary of the constitution suitable for
// display to the monitored agent at session start (transparency layer).
func (c *Constitution) Text() string {
	var b strings.Builder
	b.WriteString(c.ContractText)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "Monitoring frequency : %s\n", c.MonitoringFrequency)
	fmt.Fprintf(&b, "Trust threshold      : %.2f\n", c.TrustThreshold)
	if c.SelfReportEnabled {
		b.WriteString("Self-reporting       : enabled\n")
	} else {
		b.WriteString("Self-reporting       : disabled\n")
	}
	return b.String()
}
