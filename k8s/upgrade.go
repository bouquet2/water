package k8s

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/siderolabs/go-kubernetes/kubernetes/upgrade"
	"github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/cluster/kubernetes"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/config"
)

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
		PrePullImages:        false,
		DryRun:               false,
	}

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
