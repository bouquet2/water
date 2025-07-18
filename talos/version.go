package talos

import (
	"context"
	"fmt"
	"time"

	"github.com/bouquet2/water/k8s"
	"github.com/rs/zerolog/log"
)

// GetClusterInfo retrieves current cluster information
func (c *Client) GetClusterInfo() (*ClusterInfo, error) {
	log.Info().Msg("Retrieving cluster information")

	ctx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()

	// Get version information
	versionResp, err := c.client.Version(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get version information: %w", err)
	}

	log.Debug().Interface("version", versionResp).Msg("Version information retrieved")

	// Extract version information from node responses
	var talosVersion string
	var nodes []NodeInfo

	// Get actual node information from Kubernetes API
	nodeNames, err := c.getNodeNames(ctx)
	if err != nil {
		log.Debug().Err(err).Msg("Failed to get node names from Kubernetes API, using Talos response indices")
		nodeNames = make([]string, len(versionResp.Messages))
		for i := range nodeNames {
			nodeNames[i] = fmt.Sprintf("node-%d", i)
		}
	}

	// Get Talos version from the first response (all nodes should have the same version)
	for _, nodeResp := range versionResp.Messages {
		if nodeResp.Version != nil && talosVersion == "" {
			talosVersion = nodeResp.Version.Tag
			break
		}
	}

	// Create node info for all nodes we discovered via Kubernetes API
	for _, nodeName := range nodeNames {
		// Check if this is a control plane node
		isControlPlane, err := c.isControlPlaneNode(ctx, nodeName)
		if err != nil {
			log.Debug().Err(err).Str("node", nodeName).Msg("Failed to determine if node is control plane")
			isControlPlane = false
		}

		// Get node endpoint (IP address)
		endpoint, err := c.getNodeEndpoint(ctx, nodeName)
		if err != nil {
			log.Debug().Err(err).Str("node", nodeName).Msg("Failed to get node endpoint")
			endpoint = nodeName // Fallback to node name
		}

		nodes = append(nodes, NodeInfo{
			Name:           nodeName,
			TalosVersion:   talosVersion, // Use the same version for all nodes
			Ready:          true,         // Assume ready if Kubernetes API can see it
			IsControlPlane: isControlPlane,
			Endpoint:       endpoint,
		})
	}

	// Get Kubernetes version by querying the Kubernetes API through Talos
	k8sVersionDetailed, err := k8s.GetDetailedVersion(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get Kubernetes version, using placeholder")
		k8sVersionDetailed.GitVersion = "unknown"
	}

	clusterInfo := &ClusterInfo{
		TalosVersion: talosVersion,
		K8sVersion:   k8sVersionDetailed.GitVersion,
		Nodes:        nodes,
	}

	log.Info().
		Str("talos_version", clusterInfo.TalosVersion).
		Str("k8s_version", clusterInfo.K8sVersion).
		Int("node_count", len(clusterInfo.Nodes)).
		Msg("Cluster information retrieved successfully")

	return clusterInfo, nil
}

// getNodeNames retrieves the actual node names from the cluster
func (c *Client) getNodeNames(ctx context.Context) ([]string, error) {
	return k8s.GetNodeNames(ctx)
}

// isControlPlaneNode checks if a node is a control plane node
func (c *Client) isControlPlaneNode(ctx context.Context, nodeName string) (bool, error) {
	return k8s.IsControlPlaneNode(ctx, nodeName)
}

// getNodeEndpoint retrieves the endpoint (IP address) for a node
func (c *Client) getNodeEndpoint(ctx context.Context, nodeName string) (string, error) {
	return k8s.GetNodeEndpoint(ctx, nodeName)
}
