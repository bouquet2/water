package upgrade

import (
	"fmt"
	"strings"
)

type ForceMode string

const (
	ForceModeVersion      ForceMode = "version"
	ForceModeAvailability ForceMode = "availability"
	ForceModeReadiness    ForceMode = "readiness"
	ForceModeAll          ForceMode = "all"
)

type ForceModes struct {
	modes map[ForceMode]bool
}

func NewForceModes() *ForceModes {
	return &ForceModes{
		modes: make(map[ForceMode]bool),
	}
}

func (fm *ForceModes) Set(value string) error {
	if value == "" {
		fm.modes[ForceModeVersion] = true
		return nil
	}

	modes := strings.Split(value, ",")
	for _, modeStr := range modes {
		modeStr = strings.TrimSpace(strings.ToLower(modeStr))
		if modeStr == "" {
			return fmt.Errorf("invalid force mode: empty mode in '%s'", value)
		}

		mode := ForceMode(modeStr)
		switch mode {
		case ForceModeVersion, ForceModeAvailability, ForceModeReadiness, ForceModeAll:
			fm.modes[mode] = true
		default:
			return fmt.Errorf("invalid force mode '%s', available modes: version, availability, readiness, all", modeStr)
		}
	}

	if fm.modes[ForceModeAll] {
		fm.modes[ForceModeVersion] = true
		fm.modes[ForceModeAvailability] = true
		fm.modes[ForceModeReadiness] = true
	}

	return nil
}

func (fm *ForceModes) String() string {
	if len(fm.modes) == 0 {
		return ""
	}

	if fm.modes[ForceModeAll] {
		return "all"
	}

	// Iterate in defined order for consistent output
	active := []string{}
	for _, mode := range []ForceMode{ForceModeVersion, ForceModeAvailability, ForceModeReadiness} {
		if fm.modes[mode] {
			active = append(active, string(mode))
		}
	}

	return strings.Join(active, ",")
}

func (fm *ForceModes) HasMode(mode ForceMode) bool {
	return fm.modes[mode]
}

func (fm *ForceModes) IsForcingVersion() bool {
	return fm.HasMode(ForceModeVersion)
}

func (fm *ForceModes) IsForcingAvailability() bool {
	return fm.HasMode(ForceModeAvailability)
}

func (fm *ForceModes) IsForcingReadiness() bool {
	return fm.HasMode(ForceModeReadiness)
}

func (fm *ForceModes) IsForcingAll() bool {
	return fm.HasMode(ForceModeAll)
}

func (fm *ForceModes) IsActive() bool {
	return len(fm.modes) > 0
}

func (fm *ForceModes) ModesString() string {
	return fm.String()
}