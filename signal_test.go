package firmament

import (
	"testing"
	"time"
)

func TestSignalValid(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name  string
		sig   Signal
		valid bool
	}{
		{
			name:  "valid concealment",
			sig:   Signal{Type: SignalConcealment, SessionID: "s", Severity: 3, Timestamp: now},
			valid: true,
		},
		{
			name:  "valid all types",
			sig:   Signal{Type: SignalCoherence, SessionID: "s", Severity: 1, Timestamp: now},
			valid: true,
		},
		{
			name:  "valid max severity",
			sig:   Signal{Type: SignalEscalation, SessionID: "s", Severity: 5, Timestamp: now},
			valid: true,
		},
		{
			name:  "unknown type",
			sig:   Signal{Type: "unknown", SessionID: "s", Severity: 3, Timestamp: now},
			valid: false,
		},
		{
			name:  "severity too low",
			sig:   Signal{Type: SignalConcealment, SessionID: "s", Severity: 0, Timestamp: now},
			valid: false,
		},
		{
			name:  "severity too high",
			sig:   Signal{Type: SignalConcealment, SessionID: "s", Severity: 6, Timestamp: now},
			valid: false,
		},
		{
			name:  "empty session",
			sig:   Signal{Type: SignalConcealment, SessionID: "", Severity: 3, Timestamp: now},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sig.Valid(); got != tt.valid {
				t.Errorf("Valid() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestValidSignalTypesAllConstants(t *testing.T) {
	constants := []SignalType{
		SignalCoherence,
		SignalConcealment,
		SignalEscalation,
		SignalEvaluationAware,
	}
	for _, c := range constants {
		if _, ok := ValidSignalTypes[c]; !ok {
			t.Errorf("constant %q missing from ValidSignalTypes", c)
		}
	}
}
