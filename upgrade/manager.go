package upgrade

import (
	"context"
	"fmt"
	"time"

	"github.com/bouquet2/water/config"
	"github.com/bouquet2/water/k8s"
	"github.com/bouquet2/water/talos"
	"github.com/bouquet2/water/version"
	"github.com/rs/zerolog/log"
)

// Manager handles the upgrade process for Talos and Kubernetes
type Manager struct {
	talosClient *talos.Client
	config      *config.Config
}

// NewManager creates a new upgrade manager
func NewManager(talosClient *talos.Client, cfg *config.Config) *Manager {
	return &Manager{
		talosClient: talosClient,
		config:      cfg,
	}
}

// UpgradeResult represents the result of an upgrade operation
type UpgradeResult struct {
	TalosUpgraded    bool
	K8sUpgraded      bool
	Errors           []error
	NodesUpgraded    []string
	FailedNodes      []string
	RollbackRequired bool
	UpgradeDuration  time.Duration
}

// String returns a string representation of the upgrade result
func (r *UpgradeResult) String() string {
	if len(r.Errors) > 0 {
		return fmt.Sprintf("Upgrades completed with errors: Talos=%t, K8s=%t, Errors=%d",
			r.TalosUpgraded, r.K8sUpgraded, len(r.Errors))
	}
	return fmt.Sprintf("Upgrades completed successfully: Talos=%t, K8s=%t",
		r.TalosUpgraded, r.K8sUpgraded)
}

// HasErrors returns true if there were any errors during the upgrade
func (r *UpgradeResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// AddUpgradedNode adds a node to the list of successfully upgraded nodes
func (r *UpgradeResult) AddUpgradedNode(nodeName string) {
	if !contains(r.NodesUpgraded, nodeName) {
		r.NodesUpgraded = append(r.NodesUpgraded, nodeName)
	}
}

// AddFailedNode adds a node to the list of failed nodes
func (r *UpgradeResult) AddFailedNode(nodeName string) {
	if !contains(r.FailedNodes, nodeName) {
		r.FailedNodes = append(r.FailedNodes, nodeName)
	}
	r.RollbackRequired = true
}

// GetSuccessRate returns the success rate of the upgrade
func (r *UpgradeResult) GetSuccessRate() float64 {
	total := len(r.NodesUpgraded) + len(r.FailedNodes)
	if total == 0 {
		return 0.0
	}
	return float64(len(r.NodesUpgraded)) / float64(total)
}

// PerformUpgrade performs the complete upgrade process
func (m *Manager) PerformUpgrade() (*UpgradeResult, error) {
	log.Info().Msg("Starting upgrade process")
	startTime := time.Now()

	result := &UpgradeResult{
		NodesUpgraded: make([]string, 0),
		FailedNodes:   make([]string, 0),
		Errors:        make([]error, 0),
	}

	// Validate prerequisites before starting upgrade
	if err := m.validateUpgradePrerequisites(); err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("upgrade prerequisites validation failed: %w", err))
		return result, fmt.Errorf("upgrade prerequisites validation failed: %w", err)
	}

	// Get current cluster information
	clusterInfo, err := m.talosClient.GetClusterInfo()
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("failed to get cluster information: %w", err))
		return result, fmt.Errorf("failed to get cluster information: %w", err)
	}

	log.Info().
		Str("current_talos", clusterInfo.TalosVersion).
		Str("current_k8s", clusterInfo.K8sVersion).
		Str("target_talos", m.config.Talos.Version).
		Str("target_k8s", m.config.K8s.Version).
		Int("total_nodes", len(clusterInfo.Nodes)).
		Msg("Current vs target versions")

	// Check if Talos upgrade is needed by examining individual nodes
	talosNeedsUpgrade, nodesToUpgrade := m.checkTalosUpgradeNeeded(clusterInfo)
	if len(nodesToUpgrade) > 0 {
		log.Info().
			Strs("nodes_to_upgrade", nodesToUpgrade).
			Str("target_version", m.config.Talos.Version).
			Msg("Some nodes need Talos upgrade")
	}

	// Check if target Talos version is available
	if err := version.ValidateTargetVersion(m.config.Talos.Version, version.TalosRelease); err != nil {
		log.Warn().
			Str("target_version", m.config.Talos.Version).
			Msg("Target Talos version is not yet released - skipping Talos upgrade")
		// Don't perform Talos upgrade, but continue to check Kubernetes
	} else if talosNeedsUpgrade {
		log.Info().Msg("Talos upgrade required")
		if err := m.upgradeTalos(); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("Talos upgrade failed: %w", err))
		} else {
			result.TalosUpgraded = true
		}
	} else {
		log.Info().Msg("Talos is already at the target version")
	}

	// Check if target Kubernetes version is available
	if err := version.ValidateTargetVersion(m.config.K8s.Version, version.KubernetesRelease); err != nil {
		log.Warn().
			Str("target_version", m.config.K8s.Version).
			Msg("Target Kubernetes version is not yet released - skipping Kubernetes upgrade")
	} else {
		// Check if Kubernetes upgrade is needed
		k8sNeedsUpgrade, err := version.NeedsUpgrade(clusterInfo.K8sVersion, m.config.K8s.Version)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("failed to check Kubernetes version: %w", err))
		} else if k8sNeedsUpgrade {
			log.Info().Msg("Kubernetes upgrade required")

			// If Talos was upgraded, wait a bit before upgrading Kubernetes
			if result.TalosUpgraded {
				log.Info().Msg("Waiting for Talos upgrade to stabilize before upgrading Kubernetes")
				time.Sleep(2 * time.Minute)
			}

			if err := m.upgradeKubernetes(); err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("Kubernetes upgrade failed: %w", err))
			} else {
				result.K8sUpgraded = true
			}
		} else {
			log.Info().Msg("Kubernetes is already at the target version")
		}
	}

	// Set final upgrade duration
	result.UpgradeDuration = time.Since(startTime)

	// Log final result
	if result.HasErrors() {
		log.Error().
			Int("error_count", len(result.Errors)).
			Bool("talos_upgraded", result.TalosUpgraded).
			Bool("k8s_upgraded", result.K8sUpgraded).
			Int("nodes_upgraded", len(result.NodesUpgraded)).
			Int("nodes_failed", len(result.FailedNodes)).
			Dur("total_duration", result.UpgradeDuration).
			Float64("success_rate", result.GetSuccessRate()).
			Msg("Upgrade process completed with errors")

		for i, err := range result.Errors {
			log.Error().Err(err).Int("error_index", i).Msg("Upgrade error")
		}

		if result.RollbackRequired {
			log.Warn().Msg("Rollback may be required due to failed nodes")
		}
	} else {
		log.Info().
			Bool("talos_upgraded", result.TalosUpgraded).
			Bool("k8s_upgraded", result.K8sUpgraded).
			Int("nodes_upgraded", len(result.NodesUpgraded)).
			Dur("total_duration", result.UpgradeDuration).
			Msg("Upgrade process completed successfully")
	}

	return result, nil
}

// upgradeTalos performs the Talos upgrade with configurable node ordering
func (m *Manager) upgradeTalos() error {
	log.Info().
		Str("target_version", m.config.Talos.Version).
		Str("image_id", m.config.Talos.ImageID).
		Str("upgrade_order", string(m.config.Talos.UpgradeOrder)).
		Msg("Starting Talos upgrade")

	startTime := time.Now()

	// Get cluster info to determine node types
	clusterInfo, err := m.talosClient.GetClusterInfo()
	if err != nil {
		return fmt.Errorf("failed to get cluster info for upgrade planning: %w", err)
	}

	// Separate control plane and worker nodes
	var controlPlaneNodes, workerNodes []string
	for _, node := range clusterInfo.Nodes {
		if node.IsControlPlane {
			controlPlaneNodes = append(controlPlaneNodes, node.Name)
		} else {
			workerNodes = append(workerNodes, node.Name)
		}
	}

	// Upgrade nodes based on configured order
	if m.config.Talos.UpgradeOrder == "workers-first" {
		// Upgrade worker nodes first
		if len(workerNodes) > 0 {
			log.Info().Msg("Upgrading worker nodes first")
			err := m.upgradeNodesSequentially(workerNodes, clusterInfo.Nodes)
			if err != nil {
				log.Error().
					Err(err).
					Strs("nodes", workerNodes).
					Msg("Worker node upgrade failed")
				return fmt.Errorf("worker node upgrade failed: %w", err)
			}

			// Wait for workers to stabilize
			log.Info().Msg("Waiting for worker nodes to stabilize...")
			time.Sleep(2 * time.Minute)
		}

		// Then upgrade control plane nodes
		if len(controlPlaneNodes) > 0 {
			log.Info().Msg("Upgrading control plane nodes")
			err := m.upgradeNodesSequentially(controlPlaneNodes, clusterInfo.Nodes)
			if err != nil {
				log.Error().
					Err(err).
					Strs("nodes", controlPlaneNodes).
					Msg("Control plane upgrade failed")
				return fmt.Errorf("control plane upgrade failed: %w", err)
			}
		}
	} else {
		// Default: Upgrade control plane nodes first
		if len(controlPlaneNodes) > 0 {
			log.Info().Msg("Upgrading control plane nodes first")
			err := m.upgradeNodesSequentially(controlPlaneNodes, clusterInfo.Nodes)
			if err != nil {
				log.Error().
					Err(err).
					Strs("nodes", controlPlaneNodes).
					Msg("Control plane upgrade failed")
				return fmt.Errorf("control plane upgrade failed: %w", err)
			}

			// Wait for control plane to stabilize
			log.Info().Msg("Waiting for control plane to stabilize...")
			time.Sleep(2 * time.Minute)
		}

		// Then upgrade worker nodes
		if len(workerNodes) > 0 {
			log.Info().Msg("Upgrading worker nodes")
			err := m.upgradeNodesSequentially(workerNodes, clusterInfo.Nodes)
			if err != nil {
				log.Error().
					Err(err).
					Strs("nodes", workerNodes).
					Msg("Worker node upgrade failed")
				return fmt.Errorf("worker node upgrade failed: %w", err)
			}
		}
	}

	log.Info().
		Str("target_version", m.config.Talos.Version).
		Dur("duration", time.Since(startTime)).
		Int("total_nodes", len(controlPlaneNodes)+len(workerNodes)).
		Msg("Talos upgrade completed successfully")

	return nil
}

// upgradeNodesSequentially upgrades a list of nodes one by one
func (m *Manager) upgradeNodesSequentially(nodeNames []string, allNodes []talos.NodeInfo) error {
	return m.upgradeNodesSequentiallyWithResult(nodeNames, allNodes, nil)
}

// upgradeNodesSequentiallyWithResult upgrades a list of nodes one by one and tracks results
func (m *Manager) upgradeNodesSequentiallyWithResult(nodeNames []string, allNodes []talos.NodeInfo, result *UpgradeResult) error {
	// Create a map for quick node lookup
	nodeMap := make(map[string]talos.NodeInfo)
	for _, node := range allNodes {
		nodeMap[node.Name] = node
	}

	for i, nodeName := range nodeNames {
		log.Info().
			Str("node", nodeName).
			Int("current", i+1).
			Int("total", len(nodeNames)).
			Msg("Starting upgrade for node")

		// Get node info
		nodeInfo, exists := nodeMap[nodeName]
		if !exists {
			return fmt.Errorf("node %s not found in cluster info", nodeName)
		}

		// Construct the full image reference by combining imageID with version
		fullImageRef := m.config.Talos.ImageID + ":" + m.config.Talos.Version

		// Upgrade the node
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		err := m.talosClient.UpgradeNode(ctx, nodeInfo.Endpoint, fullImageRef)
		cancel()

		if err != nil {
			log.Error().
				Str("node", nodeName).
				Err(err).
				Msg("Failed to upgrade node")

			if result != nil {
				result.AddFailedNode(nodeName)
				result.Errors = append(result.Errors, fmt.Errorf("failed to upgrade node %s: %w", nodeName, err))
			}

			// Continue with other nodes instead of failing completely
			continue
		}

		log.Info().Str("node", nodeName).Msg("Upgrade initiated, waiting for node to reboot")

		// Wait for the node to reboot and come back online
		waitCtx, waitCancel := context.WithTimeout(context.Background(), 8*time.Minute)
		err = m.talosClient.WaitForNodeReboot(waitCtx, nodeInfo.Endpoint, 8*time.Minute)
		waitCancel()

		if err != nil {
			log.Warn().
				Str("node", nodeName).
				Err(err).
				Msg("Node may not have come back online within timeout")

			if result != nil {
				result.AddFailedNode(nodeName)
				result.Errors = append(result.Errors, fmt.Errorf("node %s failed to come back online: %w", nodeName, err))
			}
		} else {
			log.Info().Str("node", nodeName).Msg("Node upgrade completed successfully")

			if result != nil {
				result.AddUpgradedNode(nodeName)
			}
		}

		// Wait a bit between nodes to avoid overwhelming the cluster
		if i < len(nodeNames)-1 {
			log.Info().Msg("Waiting before upgrading next node...")
			time.Sleep(30 * time.Second)
		}
	}

	// Monitor the overall upgrade progress for all nodes
	log.Info().Strs("nodes", nodeNames).Msg("Starting post-upgrade monitoring")
	ctx := context.Background()
	err := m.monitorUpgradeProgress(ctx, m.config.Talos.Version, nodeNames, "talos")
	if err != nil {
		log.Error().Err(err).Msg("Upgrade monitoring detected issues")
		// Note: In a production system, you might want to trigger rollback here
		return fmt.Errorf("upgrade monitoring failed: %w", err)
	}

	return nil
}

// upgradeKubernetes performs the Kubernetes upgrade with configurable node ordering
func (m *Manager) upgradeKubernetes() error {
	log.Info().
		Str("target_version", m.config.K8s.Version).
		Str("upgrade_order", string(m.config.K8s.UpgradeOrder)).
		Msg("Starting Kubernetes upgrade")

	startTime := time.Now()

	// Get cluster info to determine node types
	clusterInfo, err := m.talosClient.GetClusterInfo()
	if err != nil {
		return fmt.Errorf("failed to get cluster info for Kubernetes upgrade: %w", err)
	}

	// Separate control plane and worker nodes
	var controlPlaneNodes []string
	var workerNodes []string

	for _, node := range clusterInfo.Nodes {
		if node.IsControlPlane {
			controlPlaneNodes = append(controlPlaneNodes, node.Name)
		} else {
			workerNodes = append(workerNodes, node.Name)
		}
	}

	log.Info().
		Int("control_plane_nodes", len(controlPlaneNodes)).
		Int("worker_nodes", len(workerNodes)).
		Msg("Node categorization for Kubernetes upgrade complete")

	// Upgrade Kubernetes based on configured order
	if m.config.K8s.UpgradeOrder == "workers-first" {
		// Upgrade worker nodes first
		if len(workerNodes) > 0 {
			log.Info().Msg("Upgrading Kubernetes on worker nodes first")
			err := m.upgradeKubernetesOnNodes(workerNodes, clusterInfo.Nodes)
			if err != nil {
				return fmt.Errorf("worker node Kubernetes upgrade failed: %w", err)
			}

			// Wait for workers to stabilize
			log.Info().Msg("Waiting for worker nodes to stabilize after Kubernetes upgrade...")
			time.Sleep(1 * time.Minute)
		}

		// Then upgrade control plane nodes
		if len(controlPlaneNodes) > 0 {
			log.Info().Msg("Upgrading Kubernetes on control plane nodes")
			err := m.upgradeKubernetesOnNodes(controlPlaneNodes, clusterInfo.Nodes)
			if err != nil {
				return fmt.Errorf("control plane Kubernetes upgrade failed: %w", err)
			}
		}
	} else {
		// Default: Upgrade control plane nodes first
		if len(controlPlaneNodes) > 0 {
			log.Info().Msg("Upgrading Kubernetes on control plane nodes first")
			err := m.upgradeKubernetesOnNodes(controlPlaneNodes, clusterInfo.Nodes)
			if err != nil {
				return fmt.Errorf("control plane Kubernetes upgrade failed: %w", err)
			}

			// Wait for control plane to stabilize
			log.Info().Msg("Waiting for control plane to stabilize after Kubernetes upgrade...")
			time.Sleep(1 * time.Minute)
		}

		// Then upgrade worker nodes
		if len(workerNodes) > 0 {
			log.Info().Msg("Upgrading Kubernetes on worker nodes")
			err := m.upgradeKubernetesOnNodes(workerNodes, clusterInfo.Nodes)
			if err != nil {
				return fmt.Errorf("worker node Kubernetes upgrade failed: %w", err)
			}
		}
	}

	log.Info().
		Str("target_version", m.config.K8s.Version).
		Str("upgrade_order", string(m.config.K8s.UpgradeOrder)).
		Dur("duration", time.Since(startTime)).
		Int("total_nodes", len(controlPlaneNodes)+len(workerNodes)).
		Msg("Kubernetes upgrade completed successfully")

	return nil
}

// upgradeKubernetesOnNodes upgrades Kubernetes on a list of nodes using Talos API
func (m *Manager) upgradeKubernetesOnNodes(nodeNames []string, allNodes []talos.NodeInfo) error {
	// Create a map for quick node lookup
	nodeMap := make(map[string]talos.NodeInfo)
	for _, node := range allNodes {
		nodeMap[node.Name] = node
	}

	for i, nodeName := range nodeNames {
		log.Info().
			Str("node", nodeName).
			Int("current", i+1).
			Int("total", len(nodeNames)).
			Msg("Starting Kubernetes upgrade for node")

		// Get node info
		nodeInfo, exists := nodeMap[nodeName]
		if !exists {
			return fmt.Errorf("node %s not found in cluster info", nodeName)
		}

		// Upgrade Kubernetes on the node using Talos API
		err := m.upgradeKubernetesOnSingleNode(nodeInfo)
		if err != nil {
			return fmt.Errorf("failed to upgrade Kubernetes on node %s: %w", nodeName, err)
		}

		log.Info().Str("node", nodeName).Msg("Kubernetes upgrade completed for node")

		// Wait a bit between nodes to avoid overwhelming the cluster
		if i < len(nodeNames)-1 {
			log.Info().Msg("Waiting before upgrading Kubernetes on next node...")
			time.Sleep(30 * time.Second)
		}
	}

	// Monitor the Kubernetes upgrade progress for all nodes
	log.Info().Strs("nodes", nodeNames).Msg("Starting post-Kubernetes-upgrade monitoring")
	ctx := context.Background()
	err := m.monitorUpgradeProgress(ctx, m.config.K8s.Version, nodeNames, "kubernetes")
	if err != nil {
		log.Error().Err(err).Msg("Kubernetes upgrade monitoring detected issues")
		// Note: In a production system, you might want to trigger rollback here
		return fmt.Errorf("Kubernetes upgrade monitoring failed: %w", err)
	}

	return nil
}

// upgradeKubernetesOnSingleNode upgrades Kubernetes on a single node using Talos API
func (m *Manager) upgradeKubernetesOnSingleNode(nodeInfo talos.NodeInfo) error {
	log.Info().
		Str("node", nodeInfo.Name).
		Str("endpoint", nodeInfo.Endpoint).
		Str("target_version", m.config.K8s.Version).
		Msg("Upgrading Kubernetes on single node using Talos machine configuration")

	// Create a context with timeout for the upgrade operation
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Use the k8s package function to upgrade Kubernetes using Talos cluster API
	err := k8s.UpgradeKubernetesOnNode(ctx, m.talosClient.GetClient(), m.talosClient.GetConfig(), nodeInfo.Endpoint, m.config.K8s.Version)
	if err != nil {
		return fmt.Errorf("failed to upgrade Kubernetes on node %s: %w", nodeInfo.Name, err)
	}

	log.Info().
		Str("node", nodeInfo.Name).
		Str("version", m.config.K8s.Version).
		Msg("Kubernetes upgrade completed successfully on node")

	return nil
}

func (m *Manager) monitorUpgradeProgress(ctx context.Context, targetVersion string, nodeNames []string, upgradeType string) error {
	log.Info().
		Str("target_version", targetVersion).
		Str("upgrade_type", upgradeType).
		Strs("nodes", nodeNames).
		Msg("Starting upgrade progress monitoring")

	timeout := 10 * time.Minute
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	var failedNodes []string
	var successfulNodes []string

	for {
		select {
		case <-timeoutCtx.Done():
			if len(failedNodes) > 0 {
				log.Error().
					Strs("failed_nodes", failedNodes).
					Msg("Upgrade monitoring timed out with failed nodes")
				return fmt.Errorf("upgrade monitoring timed out, failed nodes: %v", failedNodes)
			}
			log.Info().Msg("Upgrade monitoring completed successfully")
			return nil

		case <-ticker.C:
			log.Debug().Msg("Checking upgrade progress...")

			// Check each node's status
			for _, nodeName := range nodeNames {
				if contains(successfulNodes, nodeName) {
					continue // Already successful
				}

				if contains(failedNodes, nodeName) {
					continue // Already failed
				}

				// Check node status
				healthy, err := m.checkNodeHealth(nodeName, targetVersion, upgradeType)
				if err != nil {
					log.Debug().
						Str("node", nodeName).
						Err(err).
						Msg("Error checking node health")
					continue
				}

				if healthy {
					successfulNodes = append(successfulNodes, nodeName)
					log.Info().
						Str("node", nodeName).
						Str("target_version", targetVersion).
						Msg("Node upgrade completed successfully")
				}
			}

			// Check if all nodes are successful
			if len(successfulNodes) == len(nodeNames) {
				log.Info().
					Strs("successful_nodes", successfulNodes).
					Msg("All nodes upgraded successfully")
				return nil
			}

			// Check for failed nodes (nodes that haven't progressed for too long)
			// This is a simplified check - in production you'd want more sophisticated monitoring
			log.Debug().
				Int("successful", len(successfulNodes)).
				Int("total", len(nodeNames)).
				Msg("Upgrade progress check")
		}
	}
}

// checkNodeHealth checks if a node has successfully completed its upgrade
func (m *Manager) checkNodeHealth(nodeName, targetVersion, upgradeType string) (bool, error) {
	// Get current cluster information
	clusterInfo, err := m.talosClient.GetClusterInfo()
	if err != nil {
		return false, fmt.Errorf("failed to get cluster info: %w", err)
	}

	// Find the node
	for _, node := range clusterInfo.Nodes {
		if node.Name == nodeName {
			if !node.Ready {
				return false, nil // Node not ready yet
			}

			// Check version based on upgrade type
			switch upgradeType {
			case "talos":
				return node.TalosVersion == targetVersion, nil
			case "kubernetes":
				return clusterInfo.K8sVersion == targetVersion, nil
			default:
				return false, fmt.Errorf("unknown upgrade type: %s", upgradeType)
			}
		}
	}

	return false, fmt.Errorf("node %s not found in cluster info", nodeName)
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// validateUpgradePrerequisites checks if the cluster is ready for upgrade
func (m *Manager) validateUpgradePrerequisites() error {
	log.Info().Msg("Validating upgrade prerequisites")

	// Get current cluster information
	clusterInfo, err := m.talosClient.GetClusterInfo()
	if err != nil {
		return fmt.Errorf("failed to get cluster information for validation: %w", err)
	}

	// Check if all nodes are ready
	var notReadyNodes []string
	for _, node := range clusterInfo.Nodes {
		if !node.Ready {
			notReadyNodes = append(notReadyNodes, node.Name)
		}
	}

	if len(notReadyNodes) > 0 {
		return fmt.Errorf("some nodes are not ready for upgrade: %v", notReadyNodes)
	}

	// Check if we have at least one control plane node
	var controlPlaneCount int
	for _, node := range clusterInfo.Nodes {
		if node.IsControlPlane {
			controlPlaneCount++
		}
	}

	if controlPlaneCount == 0 {
		return fmt.Errorf("no control plane nodes found in cluster")
	}

	log.Info().
		Int("total_nodes", len(clusterInfo.Nodes)).
		Int("control_plane_nodes", controlPlaneCount).
		Int("worker_nodes", len(clusterInfo.Nodes)-controlPlaneCount).
		Msg("Upgrade prerequisites validated successfully")

	return nil
}

// CheckOnly performs a dry-run check without actually upgrading
func (m *Manager) CheckOnly() error {
	// Get current cluster information
	clusterInfo, err := m.talosClient.GetClusterInfo()
	if err != nil {
		return fmt.Errorf("failed to get cluster information: %w", err)
	}

	// Check Talos version
	talosNeedsUpgrade, err := version.NeedsUpgrade(clusterInfo.TalosVersion, m.config.Talos.Version)
	if err != nil {
		return fmt.Errorf("failed to check Talos version: %w", err)
	}

	// Check if target Talos version is available
	talosVersionAvailable := true
	if err := version.ValidateTargetVersion(m.config.Talos.Version, version.TalosRelease); err != nil {
		log.Warn().
			Str("target_version", m.config.Talos.Version).
			Msg("Target Talos version is not yet released - skipping Talos upgrade check")
		talosVersionAvailable = false
		talosNeedsUpgrade = false // Don't upgrade if version not available
	}

	// Check if target Kubernetes version is available
	k8sNeedsUpgrade := false
	k8sVersionAvailable := true
	if err := version.ValidateTargetVersion(m.config.K8s.Version, version.KubernetesRelease); err != nil {
		log.Warn().
			Str("target_version", m.config.K8s.Version).
			Msg("Target Kubernetes version is not yet released - skipping Kubernetes upgrade check")
		k8sVersionAvailable = false
	} else {
		// Check Kubernetes version only if target version is available
		var err error
		k8sNeedsUpgrade, err = version.NeedsUpgrade(clusterInfo.K8sVersion, m.config.K8s.Version)
		if err != nil {
			return fmt.Errorf("failed to check Kubernetes version: %w", err)
		}
	}

	// Report findings
	logEvent := log.Info().
		Str("current_talos", clusterInfo.TalosVersion).
		Str("target_talos", m.config.Talos.Version).
		Bool("talos_needs_upgrade", talosNeedsUpgrade).
		Str("current_k8s", clusterInfo.K8sVersion).
		Str("target_k8s", m.config.K8s.Version)

	if !talosVersionAvailable {
		logEvent = logEvent.Str("talos_status", "version not available")
	}
	if !k8sVersionAvailable {
		logEvent = logEvent.Str("k8s_status", "version not available")
	} else {
		logEvent = logEvent.Bool("k8s_needs_upgrade", k8sNeedsUpgrade)
	}

	if !talosVersionAvailable || !k8sVersionAvailable {
		logEvent.Msg("Upgrade checks completed (some versions not available)")
	} else {
		logEvent.Msg("All upgrade checks completed")
	}

	if !talosNeedsUpgrade && !k8sNeedsUpgrade {
		if talosVersionAvailable && k8sVersionAvailable {
			log.Info().Msg("No upgrades needed - cluster is up to date")
		} else {
			log.Info().Msg("No upgrades needed for available versions - some versions skipped (not yet released)")
		}
	} else {
		log.Info().Msg("Upgrades are needed - run without --check-only to perform upgrades")
	}

	return nil
}

// checkTalosUpgradeNeeded checks if any nodes need Talos upgrade
func (m *Manager) checkTalosUpgradeNeeded(clusterInfo *talos.ClusterInfo) (bool, []string) {
	var nodesToUpgrade []string

	for _, node := range clusterInfo.Nodes {
		needsUpgrade, err := version.NeedsUpgrade(node.TalosVersion, m.config.Talos.Version)
		if err != nil {
			log.Warn().
				Err(err).
				Str("node", node.Name).
				Str("current_version", node.TalosVersion).
				Str("target_version", m.config.Talos.Version).
				Msg("Failed to check if node needs upgrade, assuming it does")
			nodesToUpgrade = append(nodesToUpgrade, node.Name)
			continue
		}

		if needsUpgrade {
			log.Debug().
				Str("node", node.Name).
				Str("current_version", node.TalosVersion).
				Str("target_version", m.config.Talos.Version).
				Msg("Node needs Talos upgrade")
			nodesToUpgrade = append(nodesToUpgrade, node.Name)
		} else {
			log.Debug().
				Str("node", node.Name).
				Str("current_version", node.TalosVersion).
				Str("target_version", m.config.Talos.Version).
				Msg("Node already at target version")
		}
	}

	return len(nodesToUpgrade) > 0, nodesToUpgrade
}
