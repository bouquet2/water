package k8s

import (
    "context"
    "fmt"
    "io"
    "os"
    "regexp"

    "github.com/rs/zerolog/log"
    "github.com/siderolabs/go-kubernetes/kubernetes/upgrade"
    "github.com/siderolabs/talos/pkg/cluster"
    "github.com/siderolabs/talos/pkg/cluster/kubernetes"
    "github.com/siderolabs/talos/pkg/machinery/client"
    "github.com/siderolabs/talos/pkg/machinery/client/config"
    talosconstants "github.com/siderolabs/talos/pkg/machinery/constants"
    "github.com/siderolabs/talos/pkg/machinery/config/encoder"
)

// redactingWriter masks sensitive data like IP addresses in upstream logs.
type redactingWriter struct {
    w  io.Writer
    re *regexp.Regexp
}

func newRedactingWriter(w io.Writer) *redactingWriter {
    // IPv4 regex; simple and effective for log redaction
    ipv4 := regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
    return &redactingWriter{w: w, re: ipv4}
}

func (r *redactingWriter) Write(p []byte) (int, error) {
    redacted := r.re.ReplaceAllString(string(p), "[redacted]")
    // Ensure newline termination to keep logs readable when upstream omits it
    if len(redacted) == 0 || redacted[len(redacted)-1] != '\n' {
        redacted += "\n"
    }
    return r.w.Write([]byte(redacted))
}

// UpgradeKubernetesOnNode upgrades Kubernetes on a single node using Talos cluster API
func UpgradeKubernetesOnNode(ctx context.Context, talosClient *client.Client, talosConfig *config.Config, nodeEndpoint, targetVersion string) error {
	log.Info().
		Str("node", nodeEndpoint).
		Str("target_version", targetVersion).
		Msg("Starting Kubernetes upgrade on node using Talos cluster API")

	// Get current Kubernetes version to create the upgrade path
	currentVersion, err := GetKubernetesVersion(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current Kubernetes version: %w", err)
	}

	// Create upgrade path from current version to target version
	upgradePath, err := upgrade.NewPath(currentVersion, targetVersion)
	if err != nil {
		return fmt.Errorf("failed to create upgrade path from %s to %s: %w", currentVersion, targetVersion, err)
	}

	// Create a cluster provider that implements both ClientProvider and K8sProvider
	// First create a ConfigClientProvider for the ClientProvider interface
	clientProvider := &cluster.ConfigClientProvider{
		DefaultClient: talosClient,
		TalosConfig:   talosConfig,
	}

	// Then wrap it with KubernetesClient to get the K8sProvider interface
	clusterProvider := &cluster.KubernetesClient{
		ClientProvider: clientProvider,
	}

    upgradeOptions := kubernetes.UpgradeOptions{
        Path:                 upgradePath,
        ControlPlaneEndpoint: nodeEndpoint,
        UpgradeKubelet:       true,
        PrePullImages:        true,
        DryRun:               false,
        EncoderOpt:           encoder.WithComments(encoder.CommentsAll),

        // Explicitly set default image repositories required by Talos
        // to avoid Validate() failing on empty image references.
        KubeletImage:           talosconstants.KubeletImage,
        APIServerImage:         talosconstants.KubernetesAPIServerImage,
        ControllerManagerImage: talosconstants.KubernetesControllerManagerImage,
        SchedulerImage:         talosconstants.KubernetesSchedulerImage,
        ProxyImage:             talosconstants.KubeProxyImage,
    }

    // Route Talos upgrade logs through a redactor to avoid printing raw IPs
    upgradeOptions.LogOutput = newRedactingWriter(os.Stdout)

	// Use the actual Talos cluster.kubernetes.Upgrade function
	// This is the proper way to upgrade Kubernetes in Talos environments
	log.Info().
		Str("node", nodeEndpoint).
		Str("current_version", currentVersion).
		Str("target_version", targetVersion).
		Msg("Initiating Kubernetes upgrade using Talos cluster API")

	// Call the Talos cluster.kubernetes.Upgrade function
	err = kubernetes.Upgrade(ctx, clusterProvider, upgradeOptions)
	if err != nil {
		return fmt.Errorf("failed to upgrade Kubernetes on node %s from %s to %s: %w", nodeEndpoint, currentVersion, targetVersion, err)
	}

	log.Info().
		Str("node", nodeEndpoint).
		Str("from_version", currentVersion).
		Str("to_version", targetVersion).
		Msg("Kubernetes upgrade completed successfully on node")

	return nil
}
