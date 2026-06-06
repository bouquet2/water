package talos

import (
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/rs/zerolog/log"
)

var lifecycleAPIRange = semver.MustParseRange(">=1.14.0 <2.0.0")

func SupportsLifecycleUpgrade(version string) (bool, error) {
	log.Debug().Str("version", version).Msg("Checking LifecycleService API support")

	v := strings.TrimPrefix(version, "v")

	parsed, err := semver.Parse(v)
	if err != nil {
		return false, fmt.Errorf("failed to parse version '%s': %w", version, err)
	}

	supported := lifecycleAPIRange(parsed)

	log.Debug().
		Str("version", version).
		Bool("supports_lifecycle_api", supported).
		Msg("LifecycleService API support check completed")

	return supported, nil
}