package config

import (
	"fmt"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// UpgradeOrder represents the order in which nodes should be upgraded
type UpgradeOrder string

const (
	// ControlPlaneFirst upgrades control plane nodes first, then workers (default)
	ControlPlaneFirst UpgradeOrder = "control-plane-first"
	// WorkersFirst upgrades worker nodes first, then control plane
	WorkersFirst UpgradeOrder = "workers-first"
)

// Config represents the main configuration structure
type Config struct {
	Talos TalosConfig `mapstructure:"talos"`
	K8s   K8sConfig   `mapstructure:"k8s"`
}

// TalosConfig represents Talos-specific configuration
type TalosConfig struct {
	ImageID      string       `mapstructure:"imageId"`
	Version      string       `mapstructure:"version"`
	UpgradeOrder UpgradeOrder `mapstructure:"upgradeOrder"`
}

// K8sConfig represents Kubernetes-specific configuration
type K8sConfig struct {
	Version      string       `mapstructure:"version"`
	UpgradeOrder UpgradeOrder `mapstructure:"upgradeOrder"`
}

// LoadConfig loads configuration from a YAML file using Viper
func LoadConfig(configPath string) (*Config, error) {
	// Set up Viper
	v := viper.New()
	v.SetConfigType("yaml")

	// If no config path provided, look for default locations
	if configPath == "" {
		log.Info().Msg("Loading configuration file")
		v.SetConfigName("water")
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.water")
		v.AddConfigPath("/etc/water")
	} else {
		log.Info().Str("path", configPath).Msg("Loading configuration file")
		// Use the provided config path
		dir := filepath.Dir(configPath)
		filename := filepath.Base(configPath)
		ext := filepath.Ext(filename)
		name := filename[:len(filename)-len(ext)]

		v.SetConfigName(name)
		v.AddConfigPath(dir)
	}

	// Read the config file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	log.Info().Str("file", v.ConfigFileUsed()).Msg("Configuration file loaded successfully")

	// Validate mandatory fields using Viper
	mandatoryFields := []string{
		"talos.version",
		"talos.imageId",
		"k8s.version",
	}

	for _, field := range mandatoryFields {
		if !v.IsSet(field) || v.GetString(field) == "" {
			return nil, fmt.Errorf("required field '%s' is missing or empty", field)
		}
	}

	// Validate version format (should start with 'v')
	if talosVersion := v.GetString("talos.version"); talosVersion[0] != 'v' {
		return nil, fmt.Errorf("talos.version should start with 'v' (e.g., v1.10.5)")
	}

	if k8sVersion := v.GetString("k8s.version"); k8sVersion[0] != 'v' {
		return nil, fmt.Errorf("k8s.version should start with 'v' (e.g., v1.33.3)")
	}

	// Unmarshal into our config struct
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Set default upgrade orders if not specified
	if config.Talos.UpgradeOrder == "" {
		config.Talos.UpgradeOrder = ControlPlaneFirst
	}
	if config.K8s.UpgradeOrder == "" {
		config.K8s.UpgradeOrder = ControlPlaneFirst
	}

	// Validate upgrade orders
	if config.Talos.UpgradeOrder != ControlPlaneFirst && config.Talos.UpgradeOrder != WorkersFirst {
		return nil, fmt.Errorf("invalid talos.upgradeOrder '%s': must be '%s' or '%s'",
			config.Talos.UpgradeOrder, ControlPlaneFirst, WorkersFirst)
	}
	if config.K8s.UpgradeOrder != ControlPlaneFirst && config.K8s.UpgradeOrder != WorkersFirst {
		return nil, fmt.Errorf("invalid k8s.upgradeOrder '%s': must be '%s' or '%s'",
			config.K8s.UpgradeOrder, ControlPlaneFirst, WorkersFirst)
	}

	log.Info().
		Str("talos_version", config.Talos.Version).
		Str("talos_image_id", config.Talos.ImageID).
		Str("talos_upgrade_order", string(config.Talos.UpgradeOrder)).
		Str("k8s_version", config.K8s.Version).
		Str("k8s_upgrade_order", string(config.K8s.UpgradeOrder)).
		Msg("Configuration loaded and validated")

	return &config, nil
}
