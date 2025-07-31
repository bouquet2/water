package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bouquet2/water/config"
	"github.com/bouquet2/water/k8s"
	"github.com/bouquet2/water/talos"
	"github.com/bouquet2/water/upgrade"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	appName    = "water"
	appVersion = "devel"
)

func main() {
	// Parse command line flags
	var (
		configPath        = flag.String("config", "", "Path to configuration file (default: search for water.yaml)")
		talosConfigPath   = flag.String("talosconfig", "", "Path to Talos client configuration file (default: ~/.talos/config)")
		kubeconfigPath    = flag.String("kubeconfig", "", "Path to kubeconfig file (default: ~/.kube/config or KUBECONFIG env var)")
		checkOnly         = flag.Bool("check-only", false, "Only check versions without performing upgrades")
		verbose           = flag.Bool("verbose", false, "Enable verbose logging")
		quiet             = flag.Bool("quiet", false, "Enable quiet mode (errors only)")
		version           = flag.Bool("version", false, "Show version information")
		talosUpgradeOrder = flag.String("talos-upgrade-order", "", "Override Talos upgrade order: 'control-plane-first' or 'workers-first'")
		k8sUpgradeOrder   = flag.String("k8s-upgrade-order", "", "Override Kubernetes upgrade order: 'control-plane-first' or 'workers-first'")
	)
	flag.Parse()

	// Show version and exit
	if *version {
		fmt.Printf("%s version %s\n", appName, appVersion)
		return
	}

	// Set up logging
	setupLogging(*verbose, *quiet)

	// Run the main application logic and exit with the returned code
	os.Exit(run(*configPath, *talosConfigPath, *kubeconfigPath, *checkOnly, *talosUpgradeOrder, *k8sUpgradeOrder))
}

func run(configPath, talosConfigPath, kubeconfigPath string, checkOnly bool, talosUpgradeOrder, k8sUpgradeOrder string) int {
	// Display ASCII logo
	fmt.Println(`                 __
__  _  _______ _/  |_  ___________
\ \/ \/ /\__  \\   __\/ __ \_  __ \
 \     /  / __ \|  | \  ___/|  | \/
  \/\_/  (____  /__|  \___  >__|
              \/          \/       `)

	log.Info().
		Str("app", appName).
		Str("version", appVersion).
		Msg("Starting water - Talos Linux and Kubernetes upgrade tool")

	// Set default Talos config path if not provided
	if talosConfigPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Error().Err(err).Msg("Failed to get user home directory")
			return 1
		}
		talosConfigPath = filepath.Join(homeDir, ".talos", "config")
	}

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load configuration")
		return 1
	}

	// Apply command-line overrides for upgrade orders
	if talosUpgradeOrder != "" {
		if talosUpgradeOrder != "control-plane-first" && talosUpgradeOrder != "workers-first" {
			log.Error().Str("order", talosUpgradeOrder).Msg("Invalid talos-upgrade-order: must be 'control-plane-first' or 'workers-first'")
			return 1
		}
		cfg.Talos.UpgradeOrder = config.UpgradeOrder(talosUpgradeOrder)
		log.Info().Str("order", talosUpgradeOrder).Msg("Overriding Talos upgrade order from command line")
	}

	if k8sUpgradeOrder != "" {
		if k8sUpgradeOrder != "control-plane-first" && k8sUpgradeOrder != "workers-first" {
			log.Error().Str("order", k8sUpgradeOrder).Msg("Invalid k8s-upgrade-order: must be 'control-plane-first' or 'workers-first'")
			return 1
		}
		cfg.K8s.UpgradeOrder = config.UpgradeOrder(k8sUpgradeOrder)
		log.Info().Str("order", k8sUpgradeOrder).Msg("Overriding Kubernetes upgrade order from command line")
	}

	// Initialize Kubernetes client with kubeconfig path if provided
	if kubeconfigPath != "" {
		log.Info().Str("kubeconfig", kubeconfigPath).Msg("Initializing Kubernetes client with custom kubeconfig")
		if err := k8s.InitializeClient(kubeconfigPath); err != nil {
			log.Error().Err(err).Msg("Failed to initialize Kubernetes client")
			return 1
		}
	}

	// Create Talos client
	talosClient, err := talos.NewClient(talosConfigPath)
	if err != nil {
		log.Error().Err(err).Str("talos_config", talosConfigPath).Msg("Failed to create Talos client")
		return 1
	}
	defer func() {
		if err := talosClient.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close Talos client")
		}
	}()

	// Create upgrade manager
	upgradeManager := upgrade.NewManager(talosClient, cfg)

	// Perform the operation
	if checkOnly {
		log.Info().Msg("Running in check-only mode")
		if err := upgradeManager.CheckOnly(); err != nil {
			log.Error().Err(err).Msg("Version check failed")
			return 1
		}
	} else {
		log.Info().Msg("Running upgrade process")
		result, err := upgradeManager.PerformUpgrade()
		if err != nil {
			log.Error().Err(err).Msg("Upgrade process failed")
			return 1
		}

		// Handle upgrade results
		if result.HasErrors() {
			log.Error().Msg("Upgrade completed with errors")
			for _, err := range result.Errors {
				log.Error().Err(err).Msg("Upgrade error")
			}
			return 1
		}

		if result.TalosUpgraded || result.K8sUpgraded {
			log.Info().
				Bool("talos_upgraded", result.TalosUpgraded).
				Bool("k8s_upgraded", result.K8sUpgraded).
				Msg("Upgrade process completed successfully")
		} else {
			log.Info().Msg("No upgrades were needed - cluster is already up to date")
		}
	}

	log.Info().Msg("Watered all the plants")
	return 0
}

// setupLogging configures zerolog based on the provided flags
func setupLogging(verbose, quiet bool) {
	// Set up console writer with colors
	output := zerolog.ConsoleWriter{
		Out: os.Stdout,
	}

	log.Logger = zerolog.New(output).With().Timestamp().Logger()

	// Set log level based on flags
	switch {
	case quiet:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case verbose:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	log.Debug().
		Bool("verbose", verbose).
		Bool("quiet", quiet).
		Str("level", zerolog.GlobalLevel().String()).
		Msg("Logging configured")
}
