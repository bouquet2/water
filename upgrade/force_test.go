package upgrade

import (
	"strings"
	"testing"
)

func TestForceModes_Set_ValidSingleModes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ForceModes
	}{
		{
			name:  "version mode",
			input: "version",
			expected: ForceModes{modes: map[ForceMode]bool{
				ForceModeVersion: true,
			}},
		},
		{
			name:  "availability mode",
			input: "availability",
			expected: ForceModes{modes: map[ForceMode]bool{
				ForceModeAvailability: true,
			}},
		},
		{
			name:  "readiness mode",
			input: "readiness",
			expected: ForceModes{modes: map[ForceMode]bool{
				ForceModeReadiness: true,
			}},
		},
		{
			name:  "all mode",
			input: "all",
			expected: ForceModes{modes: map[ForceMode]bool{
				ForceModeAll:          true,
				ForceModeVersion:      true,
				ForceModeAvailability: true,
				ForceModeReadiness:    true,
			}},
		},
		{
			name:  "empty defaults to version",
			input: "",
			expected: ForceModes{modes: map[ForceMode]bool{
				ForceModeVersion: true,
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := NewForceModes()
			err := fm.Set(tt.input)
			if err != nil {
				t.Errorf("Set(%s) returned unexpected error: %v", tt.input, err)
			}

			for mode, expected := range tt.expected.modes {
				if fm.HasMode(mode) != expected {
					t.Errorf("HasMode(%s) = %v, expected %v", mode, fm.HasMode(mode), expected)
				}
			}
		})
	}
}

func TestForceModes_Set_ValidCombinations(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected ForceModes
	}{
		{
			name:  "version and readiness",
			input: "version,readiness",
			expected: ForceModes{modes: map[ForceMode]bool{
				ForceModeVersion:   true,
				ForceModeReadiness: true,
			}},
		},
		{
			name:  "availability and readiness",
			input: "availability,readiness",
			expected: ForceModes{modes: map[ForceMode]bool{
				ForceModeAvailability: true,
				ForceModeReadiness:    true,
			}},
		},
		{
			name:  "version and availability",
			input: "version,availability",
			expected: ForceModes{modes: map[ForceMode]bool{
				ForceModeVersion:      true,
				ForceModeAvailability: true,
			}},
		},
		{
			name:  "all three modes",
			input: "version,availability,readiness",
			expected: ForceModes{modes: map[ForceMode]bool{
				ForceModeVersion:      true,
				ForceModeAvailability: true,
				ForceModeReadiness:    true,
			}},
		},
		{
			name:  "duplicate modes are deduplicated",
			input: "version,version",
			expected: ForceModes{modes: map[ForceMode]bool{
				ForceModeVersion: true,
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := NewForceModes()
			err := fm.Set(tt.input)
			if err != nil {
				t.Errorf("Set(%s) returned unexpected error: %v", tt.input, err)
			}

			for mode, expected := range tt.expected.modes {
				if fm.HasMode(mode) != expected {
					t.Errorf("HasMode(%s) = %v, expected %v", mode, fm.HasMode(mode), expected)
				}
			}
		})
	}
}

func TestForceModes_Set_InvalidModes(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedErr string
	}{
		{
			name:        "invalid mode name",
			input:       "invalid",
			expectedErr: "invalid force mode 'invalid'",
		},
		{
			name:        "invalid mode in combination",
			input:       "version,foo",
			expectedErr: "invalid force mode 'foo'",
		},
		{
			name:        "invalid mode bar",
			input:       "bar",
			expectedErr: "invalid force mode 'bar'",
		},
		{
			name:        "empty mode in list",
			input:       ",version",
			expectedErr: "invalid force mode: empty mode",
		},
		{
			name:        "trailing comma",
			input:       "version,",
			expectedErr: "invalid force mode: empty mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := NewForceModes()
			err := fm.Set(tt.input)
			if err == nil {
				t.Errorf("Set(%s) expected error, got nil", tt.input)
			}
			if !strings.Contains(err.Error(), tt.expectedErr) {
				t.Errorf("Set(%s) error = '%v', expected to contain '%s'", tt.input, err, tt.expectedErr)
			}
		})
	}
}

func TestForceModes_String(t *testing.T) {
	tests := []struct {
		name     string
		setup    string
		expected string
	}{
		{
			name:     "empty modes",
			setup:    "",
			expected: "version",
		},
		{
			name:     "single mode version",
			setup:    "version",
			expected: "version",
		},
		{
			name:     "all mode",
			setup:    "all",
			expected: "all",
		},
		{
			name:     "combination",
			setup:    "version,readiness",
			expected: "version,readiness",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := NewForceModes()
			if tt.name == "empty modes" {
				_ = fm.Set("")
			} else {
				_ = fm.Set(tt.setup)
			}
			result := fm.String()
			if result != tt.expected {
				t.Errorf("String() = '%s', expected '%s'", result, tt.expected)
			}
		})
	}
}

func TestForceModes_Set_WhitespaceHandling(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[ForceMode]bool
	}{
		{
			name:     "leading whitespace",
			input:    " version",
			expected: map[ForceMode]bool{ForceModeVersion: true},
		},
		{
			name:     "trailing whitespace",
			input:    "version ",
			expected: map[ForceMode]bool{ForceModeVersion: true},
		},
		{
			name:     "whitespace in combination",
			input:    "version, readiness",
			expected: map[ForceMode]bool{ForceModeVersion: true, ForceModeReadiness: true},
		},
		{
			name:     "whitespace around all",
			input:    " all ",
			expected: map[ForceMode]bool{ForceModeAll: true, ForceModeVersion: true, ForceModeAvailability: true, ForceModeReadiness: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := NewForceModes()
			err := fm.Set(tt.input)
			if err != nil {
				t.Errorf("Set(%s) returned unexpected error: %v", tt.input, err)
			}

			for mode, expected := range tt.expected {
				if fm.HasMode(mode) != expected {
					t.Errorf("HasMode(%s) = %v, expected %v", mode, fm.HasMode(mode), expected)
				}
			}
		})
	}
}

func TestForceModes_Set_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[ForceMode]bool
	}{
		{
			name:     "uppercase mode",
			input:    "VERSION",
			expected: map[ForceMode]bool{ForceModeVersion: true},
		},
		{
			name:     "mixed case combination",
			input:    "Version,READINESS",
			expected: map[ForceMode]bool{ForceModeVersion: true, ForceModeReadiness: true},
		},
		{
			name:     "uppercase all",
			input:    "ALL",
			expected: map[ForceMode]bool{ForceModeAll: true, ForceModeVersion: true, ForceModeAvailability: true, ForceModeReadiness: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := NewForceModes()
			err := fm.Set(tt.input)
			if err != nil {
				t.Errorf("Set(%s) returned unexpected error: %v", tt.input, err)
			}

			for mode, expected := range tt.expected {
				if fm.HasMode(mode) != expected {
					t.Errorf("HasMode(%s) = %v, expected %v", mode, fm.HasMode(mode), expected)
				}
			}
		})
	}
}

func TestForceModes_Set_AllSupersedesOtherModes(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "all with version",
			input: "all,version",
		},
		{
			name:  "all with multiple modes",
			input: "all,version,readiness",
		},
		{
			name:  "version with all",
			input: "version,all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := NewForceModes()
			err := fm.Set(tt.input)
			if err != nil {
				t.Errorf("Set(%s) returned unexpected error: %v", tt.input, err)
			}

			// All should imply all three individual modes
			if !fm.HasMode(ForceModeAll) {
				t.Errorf("HasMode(all) = false, expected true")
			}
			if !fm.HasMode(ForceModeVersion) {
				t.Errorf("HasMode(version) = false, expected true (implied by all)")
			}
			if !fm.HasMode(ForceModeAvailability) {
				t.Errorf("HasMode(availability) = false, expected true (implied by all)")
			}
			if !fm.HasMode(ForceModeReadiness) {
				t.Errorf("HasMode(readiness) = false, expected true (implied by all)")
			}

			// String() should just return "all" (all supersedes)
			if fm.String() != "all" {
				t.Errorf("String() = '%s', expected 'all' (all mode supersedes)", fm.String())
			}
		})
	}
}