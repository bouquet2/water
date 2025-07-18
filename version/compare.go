package version

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

// ComparisonResult represents the result of comparing two versions
type ComparisonResult int

const (
	// Equal indicates versions are equal
	Equal ComparisonResult = 0
	// Newer indicates the first version is newer than the second
	Newer ComparisonResult = 1
	// Older indicates the first version is older than the second
	Older ComparisonResult = -1
)

// String returns a string representation of the comparison result
func (r ComparisonResult) String() string {
	switch r {
	case Equal:
		return "equal"
	case Newer:
		return "newer"
	case Older:
		return "older"
	default:
		return "unknown"
	}
}

// Compare compares two semantic versions
func Compare(version1, version2 string) (ComparisonResult, error) {
	log.Debug().
		Str("version1", version1).
		Str("version2", version2).
		Msg("Comparing versions")

	// Normalize versions (remove 'v' prefix if present)
	v1 := strings.TrimPrefix(version1, "v")
	v2 := strings.TrimPrefix(version2, "v")

	// Parse versions
	parts1, err := parseVersion(v1)
	if err != nil {
		return Equal, fmt.Errorf("failed to parse version1 '%s': %w", version1, err)
	}

	parts2, err := parseVersion(v2)
	if err != nil {
		return Equal, fmt.Errorf("failed to parse version2 '%s': %w", version2, err)
	}

	// Compare major, minor, patch
	for i := 0; i < 3; i++ {
		if parts1[i] > parts2[i] {
			log.Debug().
				Str("version1", version1).
				Str("version2", version2).
				Str("result", "newer").
				Msg("Version comparison completed")
			return Newer, nil
		}
		if parts1[i] < parts2[i] {
			log.Debug().
				Str("version1", version1).
				Str("version2", version2).
				Str("result", "older").
				Msg("Version comparison completed")
			return Older, nil
		}
	}

	log.Debug().
		Str("version1", version1).
		Str("version2", version2).
		Str("result", "equal").
		Msg("Version comparison completed")
	return Equal, nil
}

// parseVersion parses a semantic version string into major, minor, patch integers
func parseVersion(version string) ([3]int, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return [3]int{}, fmt.Errorf("invalid version format: expected major.minor.patch, got '%s'", version)
	}

	var result [3]int
	for i, part := range parts {
		// Handle pre-release versions (e.g., "1.10.5-alpha.1")
		if strings.Contains(part, "-") {
			part = strings.Split(part, "-")[0]
		}
		
		num, err := strconv.Atoi(part)
		if err != nil {
			return [3]int{}, fmt.Errorf("invalid version part '%s': %w", part, err)
		}
		result[i] = num
	}

	return result, nil
}

// NeedsUpgrade determines if an upgrade is needed from current to target version
func NeedsUpgrade(currentVersion, targetVersion string) (bool, error) {
	log.Info().
		Str("current", currentVersion).
		Str("target", targetVersion).
		Msg("Checking if upgrade is needed")

	result, err := Compare(currentVersion, targetVersion)
	if err != nil {
		return false, fmt.Errorf("failed to compare versions: %w", err)
	}

	needsUpgrade := result == Older
	
	log.Info().
		Str("current", currentVersion).
		Str("target", targetVersion).
		Bool("needs_upgrade", needsUpgrade).
		Str("comparison", result.String()).
		Msg("Upgrade check completed")

	return needsUpgrade, nil
}

// ValidateVersion validates that a version string is in the correct format
func ValidateVersion(version string) error {
	log.Debug().Str("version", version).Msg("Validating version format")

	if version == "" {
		return fmt.Errorf("version cannot be empty")
	}

	// Remove 'v' prefix if present
	v := strings.TrimPrefix(version, "v")

	// Try to parse it
	_, err := parseVersion(v)
	if err != nil {
		return fmt.Errorf("invalid version format '%s': %w", version, err)
	}

	log.Debug().Str("version", version).Msg("Version format is valid")
	return nil
}

// GetMajorMinor extracts the major.minor version from a full version string
func GetMajorMinor(version string) (string, error) {
	v := strings.TrimPrefix(version, "v")
	parts, err := parseVersion(v)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("v%d.%d", parts[0], parts[1]), nil
}

// IsCompatible checks if two versions are compatible (same major.minor)
func IsCompatible(version1, version2 string) (bool, error) {
	majorMinor1, err := GetMajorMinor(version1)
	if err != nil {
		return false, err
	}

	majorMinor2, err := GetMajorMinor(version2)
	if err != nil {
		return false, err
	}

	return majorMinor1 == majorMinor2, nil
}
