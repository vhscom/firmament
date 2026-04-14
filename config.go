package firmament

import (
	"errors"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds Firmament runtime configuration.
// Values may be set via a YAML file or overridden by environment variables.
type Config struct {
	// LogPath is the file path for signal output.
	// Empty string writes to stdout. Env: FIRMAMENT_LOG_PATH.
	LogPath string `yaml:"log_path"`

	// Patterns lists the behavioral pattern names to enable.
	// Empty or absent enables all built-in patterns.
	Patterns []string `yaml:"patterns"`
}

// DefaultConfig returns a Config populated with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Patterns: []string{"action_concealment"},
	}
}

// LoadConfig reads a YAML config file at path.
// If the file does not exist, DefaultConfig is returned without error.
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ApplyEnv overrides config fields with values from environment variables.
// Environment variables take precedence over the file-based config.
func (c *Config) ApplyEnv() {
	if v := os.Getenv("FIRMAMENT_LOG_PATH"); v != "" {
		c.LogPath = v
	}
}

// EnabledPatterns returns the list of pattern names to activate.
// Falls back to the default set if Patterns is empty.
func (c *Config) EnabledPatterns() []string {
	if len(c.Patterns) == 0 {
		return DefaultConfig().Patterns
	}
	return c.Patterns
}
