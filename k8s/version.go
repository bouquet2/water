package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bouquet2/water/version"
	"github.com/rs/zerolog/log"
)

// VersionInfo holds Kubernetes version information
type VersionInfo struct {
	GitVersion   string
	Major        string
	Minor        string
	GitCommit    string
	GitTreeState string
	BuildDate    string
	GoVersion    string
	Compiler     string
	Platform     string
}

// retryKubernetesAPICall retries a Kubernetes API call with exponential backoff
func retryKubernetesAPICall[T any](ctx context.Context, operation func() (T, error), operationName string) (T, error) {
	const maxRetries = 3
	const baseDelay = 2 // seconds

	var result T
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Debug().
			Int("attempt", attempt).
			Int("max_retries", maxRetries).
			Str("operation", operationName).
			Msg("Attempting Kubernetes API call")

		result, err := operation()
		if err == nil {
			if attempt > 1 {
				log.Info().
					Int("attempt", attempt).
					Str("operation", operationName).
					Msg("Kubernetes API call succeeded after retry")
			}
			return result, nil
		}

		lastErr = err
		log.Warn().
			Err(err).
			Int("attempt", attempt).
			Int("max_retries", maxRetries).
			Str("operation", operationName).
			Msg("Kubernetes API call failed")

		if attempt < maxRetries {
			// Exponential backoff: 2s, 4s, 8s
			delay := baseDelay * (1 << (attempt - 1))
			log.Info().
				Int("delay_seconds", delay).
				Int("next_attempt", attempt+1).
				Str("operation", operationName).
				Msg("Retrying Kubernetes API call after delay")

			select {
			case <-ctx.Done():
				return result, ctx.Err()
			case <-time.After(time.Duration(delay) * time.Second):
				// Continue to next attempt
			}
		}
	}

	return result, fmt.Errorf("failed %s after %d attempts: %w", operationName, maxRetries, lastErr)
}

// GetDetailedVersion retrieves detailed Kubernetes version information with retry logic
func GetDetailedVersion(ctx context.Context) (*VersionInfo, error) {
	return retryKubernetesAPICall(ctx, func() (*VersionInfo, error) {
		client, err := GetSharedClient()
		if err != nil {
			return nil, fmt.Errorf("failed to get Kubernetes client: %w", err)
		}

		version, err := client.clientset.Discovery().ServerVersion()
		if err != nil {
			return nil, fmt.Errorf("failed to get server version: %w", err)
		}

		return &VersionInfo{
			GitVersion:   version.GitVersion,
			Major:        version.Major,
			Minor:        version.Minor,
			GitCommit:    version.GitCommit,
			GitTreeState: version.GitTreeState,
			BuildDate:    version.BuildDate,
			GoVersion:    version.GoVersion,
			Compiler:     version.Compiler,
			Platform:     version.Platform,
		}, nil
	}, "get detailed version")
}

// CompareVersions compares two Kubernetes versions
// Returns:
//
//	-1 if version1 < version2
//	 0 if version1 == version2
//	 1 if version1 > version2
func CompareVersions(version1, version2 string) (int, error) {
	// Clean versions (remove 'v' prefix if present)
	v1 := strings.TrimPrefix(version1, "v")
	v2 := strings.TrimPrefix(version2, "v")

	// Parse versions
	v1Parts, err := parseVersion(v1)
	if err != nil {
		return 0, fmt.Errorf("failed to parse version1 %s: %w", version1, err)
	}

	v2Parts, err := parseVersion(v2)
	if err != nil {
		return 0, fmt.Errorf("failed to parse version2 %s: %w", version2, err)
	}

	// Compare major version
	if v1Parts[0] < v2Parts[0] {
		return -1, nil
	}
	if v1Parts[0] > v2Parts[0] {
		return 1, nil
	}

	// Compare minor version
	if v1Parts[1] < v2Parts[1] {
		return -1, nil
	}
	if v1Parts[1] > v2Parts[1] {
		return 1, nil
	}

	// Compare patch version
	if v1Parts[2] < v2Parts[2] {
		return -1, nil
	}
	if v1Parts[2] > v2Parts[2] {
		return 1, nil
	}

	return 0, nil
}

// parseVersion parses a semantic version string into major, minor, patch integers
func parseVersion(version string) ([3]int, error) {
	var major, minor, patch int
	var parts [3]int

	// Handle versions with additional suffixes (e.g., "1.28.0-alpha.1")
	mainVersion := strings.Split(version, "-")[0]

	n, err := fmt.Sscanf(mainVersion, "%d.%d.%d", &major, &minor, &patch)
	if err != nil || n != 3 {
		return parts, fmt.Errorf("invalid version format: %s", version)
	}

	parts[0] = major
	parts[1] = minor
	parts[2] = patch

	return parts, nil
}

// IsVersionUpgradeNeeded checks if an upgrade is needed from current to target version
func IsVersionUpgradeNeeded(currentVersion, targetVersion string) (bool, error) {
	log.Debug().
		Str("current", currentVersion).
		Str("target", targetVersion).
		Msg("Checking if Kubernetes version upgrade is needed")

	if currentVersion == "" || targetVersion == "" {
		return false, fmt.Errorf("current or target version is empty")
	}

	comparison, err := CompareVersions(currentVersion, targetVersion)
	if err != nil {
		return false, fmt.Errorf("failed to compare versions: %w", err)
	}

	// Upgrade needed if current version is less than target version
	upgradeNeeded := comparison < 0

	log.Debug().
		Str("current", currentVersion).
		Str("target", targetVersion).
		Bool("upgrade_needed", upgradeNeeded).
		Msg("Version upgrade check completed")

	return upgradeNeeded, nil
}

// ValidateVersion validates that a version string is in the correct format
func ValidateVersion(version string) error {
	if version == "" {
		return fmt.Errorf("version cannot be empty")
	}

	// Remove 'v' prefix if present
	cleanVersion := strings.TrimPrefix(version, "v")

	_, err := parseVersion(cleanVersion)
	if err != nil {
		return fmt.Errorf("invalid version format %s: %w", version, err)
	}

	return nil
}

// GetSupportedVersions returns a list of supported Kubernetes versions
// Uses the unified version system from the version package
func GetSupportedVersions() ([]string, error) {
	return version.GetSupportedVersions(version.KubernetesRelease)
}

// GetSupportedVersionsWithContext returns supported versions with context support
func GetSupportedVersionsWithContext(ctx context.Context) ([]string, error) {
	return version.GetSupportedVersionsWithContext(ctx, version.KubernetesRelease)
}

// IsVersionSupported checks if a given version is in the supported versions list
func IsVersionSupported(targetVersion string) bool {
	supported, err := version.IsVersionSupported(targetVersion, version.KubernetesRelease)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to check if version is supported")
		return false
	}
	return supported
}

// GetLatestSupportedVersion returns the latest supported Kubernetes version
func GetLatestSupportedVersion() string {
	supportedVersions, err := GetSupportedVersions()
	if err != nil || len(supportedVersions) == 0 {
		return ""
	}

	// Assuming the list is ordered with latest first
	return supportedVersions[0]
}

// FormatVersionInfo formats version information for display
func FormatVersionInfo(versionInfo *VersionInfo) string {
	return fmt.Sprintf("Kubernetes %s (built %s, go %s)",
		versionInfo.GitVersion,
		versionInfo.BuildDate,
		versionInfo.GoVersion)
}
