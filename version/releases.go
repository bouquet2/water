package version

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// ReleaseType represents the type of release to check
type ReleaseType string

const (
	TalosRelease      ReleaseType = "talos"
	KubernetesRelease ReleaseType = "kubernetes"
)

// GitHubRelease represents a GitHub release from the API
type GitHubRelease struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Prerelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
}

// ReleaseConfig holds configuration for fetching releases
type ReleaseConfig struct {
	RepoURL     string
	MaxVersions int
	UserAgent   string
}

// getReleaseConfig returns the configuration for a specific release type
func getReleaseConfig(releaseType ReleaseType) ReleaseConfig {
	switch releaseType {
	case TalosRelease:
		return ReleaseConfig{
			RepoURL:     "https://api.github.com/repos/siderolabs/talos/releases",
			MaxVersions: 5,
			UserAgent:   "water-talos-version-checker/1.0",
		}
	case KubernetesRelease:
		return ReleaseConfig{
			RepoURL:     "https://api.github.com/repos/siderolabs/kubelet/releases",
			MaxVersions: 5,
			UserAgent:   "water-k8s-version-checker/1.0",
		}
	default:
		return ReleaseConfig{}
	}
}

// fetchVersionsFromGitHub fetches the latest stable versions from GitHub API
func fetchVersionsFromGitHub(ctx context.Context, releaseType ReleaseType) ([]string, error) {
	config := getReleaseConfig(releaseType)
	if config.RepoURL == "" {
		return nil, fmt.Errorf("unsupported release type: %s", releaseType)
	}

	log.Debug().
		Str("url", config.RepoURL).
		Str("type", string(releaseType)).
		Msg("Fetching versions from GitHub API")

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", config.RepoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set User-Agent header to be a good API citizen
	req.Header.Set("User-Agent", config.UserAgent)

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	// Parse JSON response
	var releases []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to decode JSON response: %w", err)
	}

	log.Debug().
		Int("total_releases", len(releases)).
		Str("type", string(releaseType)).
		Msg("Fetched releases from GitHub")

	// Filter stable releases and extract versions
	var stableVersions []string
	versionMap := make(map[string]bool) // To avoid duplicates

	for _, release := range releases {
		// Skip prerelease, draft, or invalid versions
		if release.Prerelease || release.Draft || release.TagName == "" {
			continue
		}

		// Ensure version starts with 'v'
		version := release.TagName
		if !strings.HasPrefix(version, "v") {
			continue
		}

		// Validate version format (basic check)
		if err := ValidateVersion(version); err != nil {
			log.Debug().
				Str("version", version).
				Str("type", string(releaseType)).
				Err(err).
				Msg("Skipping invalid version")
			continue
		}

		// Add to map to avoid duplicates
		if !versionMap[version] {
			versionMap[version] = true
			stableVersions = append(stableVersions, version)
		}
	}

	// Sort versions in descending order (newest first)
	sort.Slice(stableVersions, func(i, j int) bool {
		// Compare versions - return true if i > j (for descending order)
		comparison, err := Compare(stableVersions[i], stableVersions[j])
		if err != nil {
			log.Debug().Err(err).Msg("Error comparing versions, using string comparison")
			return stableVersions[i] > stableVersions[j]
		}
		return comparison == Newer
	})

	// Limit to the configured maximum versions
	if len(stableVersions) > config.MaxVersions {
		stableVersions = stableVersions[:config.MaxVersions]
	}

	log.Debug().
		Strs("versions", stableVersions).
		Str("type", string(releaseType)).
		Msg("Filtered and sorted stable versions")

	return stableVersions, nil
}

// GetSupportedVersions returns a list of supported versions for the given release type
func GetSupportedVersions(releaseType ReleaseType) ([]string, error) {
	return GetSupportedVersionsWithContext(context.Background(), releaseType)
}

// GetSupportedVersionsWithContext returns supported versions with context support
func GetSupportedVersionsWithContext(ctx context.Context, releaseType ReleaseType) ([]string, error) {
	const maxRetries = 3
	const retryDelay = 2 * time.Second

	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Debug().
			Int("attempt", attempt).
			Int("max_retries", maxRetries).
			Str("type", string(releaseType)).
			Msg("Attempting to fetch versions from GitHub API")

		versions, err := fetchVersionsFromGitHub(ctx, releaseType)
		if err != nil {
			lastErr = err
			log.Warn().
				Err(err).
				Int("attempt", attempt).
				Int("max_retries", maxRetries).
				Str("type", string(releaseType)).
				Msg("Failed to fetch versions from GitHub API")

			if attempt < maxRetries {
				log.Info().
					Dur("delay", retryDelay).
					Int("next_attempt", attempt+1).
					Msg("Retrying after delay")
				time.Sleep(retryDelay)
				continue
			}
			break
		}

		if len(versions) == 0 {
			lastErr = fmt.Errorf("no stable versions found")
			log.Warn().
				Int("attempt", attempt).
				Str("type", string(releaseType)).
				Msg("No stable versions found from GitHub API")

			if attempt < maxRetries {
				log.Info().
					Dur("delay", retryDelay).
					Int("next_attempt", attempt+1).
					Msg("Retrying after delay")
				time.Sleep(retryDelay)
				continue
			}
			break
		}

		log.Info().
			Strs("versions", versions).
			Str("type", string(releaseType)).
			Int("attempt", attempt).
			Msg("Successfully fetched supported versions")
		return versions, nil
	}

	return nil, fmt.Errorf("failed to fetch %s versions after %d attempts: %w",
		releaseType, maxRetries, lastErr)
}

// IsVersionSupported checks if a given version is in the supported versions list
func IsVersionSupported(version string, releaseType ReleaseType) (bool, error) {
	supportedVersions, err := GetSupportedVersions(releaseType)
	if err != nil {
		return false, fmt.Errorf("failed to get supported versions: %w", err)
	}

	for _, supported := range supportedVersions {
		if version == supported {
			return true, nil
		}
	}

	return false, nil
}

// ValidateTargetVersion checks if the target version is available in releases
// Returns an error if the version is not yet released
func ValidateTargetVersion(targetVersion string, releaseType ReleaseType) error {
	log.Debug().
		Str("target_version", targetVersion).
		Str("type", string(releaseType)).
		Msg("Validating target version")

	// First validate the version format
	if err := ValidateVersion(targetVersion); err != nil {
		return fmt.Errorf("invalid target version format: %w", err)
	}

	// Get supported versions once and reuse
	supportedVersions, err := GetSupportedVersions(releaseType)
	if err != nil {
		return fmt.Errorf("failed to get supported versions: %w", err)
	}

	// Check if the version is supported (available in releases)
	isSupported := false
	for _, supported := range supportedVersions {
		if targetVersion == supported {
			isSupported = true
			break
		}
	}

	if !isSupported {
		log.Info().
			Str("target_version", targetVersion).
			Str("type", string(releaseType)).
			Strs("available_versions", supportedVersions).
			Msg("Target version is not yet released")

		return fmt.Errorf("%s version %s is not yet released. Available versions: %v",
			cases.Title(language.Und).String(string(releaseType)), targetVersion, supportedVersions)
	}

	log.Debug().
		Str("target_version", targetVersion).
		Str("type", string(releaseType)).
		Msg("Target version is valid and available")
	return nil
}
