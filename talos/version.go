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
			log.Warn().Err(err).Str("node", nodeName).Msg("Failed to get node endpoint")
			endpoint = nodeName // Fallback to node name
		}

		// Get the actual Talos version for this specific node
		nodeVersion, err := c.getNodeTalosVersion(ctx, endpoint)
		if err != nil {
			log.Warn().Err(err).Str("node", nodeName).Msg("Failed to get node Talos version, using cluster version")
			nodeVersion = talosVersion // Fallback to cluster version
		}

		nodes = append(nodes, NodeInfo{
			Name:           nodeName,
			TalosVersion:   nodeVersion, // Use the actual version for this node
			Ready:          true,        // Assume ready if Kubernetes API can see it
			IsControlPlane: isControlPlane,
			Endpoint:       endpoint,
		})
	}

	// Get Kubernetes version by querying the Kubernetes API through Talos
	var k8sVersion string
	k8sVersionDetailed, err := k8s.GetDetailedVersion(ctx)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get Kubernetes version, using placeholder")
		k8sVersion = "unknown"
	} else {
		k8sVersion = k8sVersionDetailed.GitVersion
	}

	clusterInfo := &ClusterInfo{
		TalosVersion: talosVersion,
		K8sVersion:   k8sVersion,
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
// First tries to get the correct Talos endpoint, falls back to Kubernetes node address
func (c *Client) getNodeEndpoint(ctx context.Context, nodeName string) (string, error) {
	// First try to get the endpoint from Talos configuration
	endpoint, err := c.GetNodeEndpointFromTalos(ctx, nodeName)
	if err == nil {
		log.Debug().
			Str("node", nodeName).
			Str("endpoint", endpoint).
			Msg("Using Talos endpoint for node")
		return endpoint, nil
	}

	log.Debug().
		Str("node", nodeName).
		Err(err).
		Msg("Failed to get Talos endpoint, falling back to Kubernetes node address")

	// Fall back to Kubernetes node address (which may be incorrect)
	return k8s.GetNodeEndpoint(ctx, nodeName)
}

// getNodeTalosVersion gets the Talos version for a specific node
func (c *Client) getNodeTalosVersion(ctx context.Context, nodeEndpoint string) (string, error) {
	// Create a client for this specific node
	nodeClient, err := c.CreateNodeClient(nodeEndpoint)
	if err != nil {
		return "", fmt.Errorf("failed to create client for node %s: %w", nodeEndpoint, err)
	}
	defer nodeClient.Close()

	// Get version from this specific node
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	versionResp, err := nodeClient.Version(timeoutCtx)
	if err != nil {
		return "", fmt.Errorf("failed to get version from node %s: %w", nodeEndpoint, err)
	}

	// Extract version from the first message
	for _, message := range versionResp.Messages {
		if message.Version != nil && message.Version.Tag != "" {
			log.Debug().
				Str("endpoint", nodeEndpoint).
				Str("version", message.Version.Tag).
				Msg("Retrieved Talos version from node")
			return message.Version.Tag, nil
		}
	}

	return "", fmt.Errorf("no version found in response from node %s", nodeEndpoint)
}
