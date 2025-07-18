package talos

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

// UpgradeNode performs a Talos upgrade on a single node
func (c *Client) UpgradeNode(ctx context.Context, nodeEndpoint, imageID string) error {
	log.Info().
		Str("node", nodeEndpoint).
		Str("image_id", imageID).
		Msg("Starting Talos upgrade on single node")

	// Create a client specifically for this node
	nodeClient, err := c.CreateNodeClient(nodeEndpoint)
	if err != nil {
		return fmt.Errorf("failed to create client for node %s: %w", nodeEndpoint, err)
	}
	defer nodeClient.Close()

	// Perform the upgrade on the specific node
	upgradeResp, err := nodeClient.Upgrade(ctx, imageID, false, false)
	if err != nil {
		return fmt.Errorf("failed to initiate Talos upgrade on node %s: %w", nodeEndpoint, err)
	}

	log.Debug().
		Str("node", nodeEndpoint).
		Interface("upgrade_response", upgradeResp).
		Msg("Upgrade initiated on node")

	return nil
}

// CreateNodeClient creates a Talos client for a specific node
func (c *Client) CreateNodeClient(nodeEndpoint string) (*client.Client, error) {
	// Create client options for the specific node using stored configuration
	opts := []client.OptionFunc{
		client.WithConfig(c.clientConfig),
		client.WithEndpoints(nodeEndpoint),
	}

	// Create the node-specific client
	nodeClient, err := client.New(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create node client: %w", err)
	}

	return nodeClient, nil
}

// WaitForNodeReboot waits for a node to reboot and come back online after upgrade
func (c *Client) WaitForNodeReboot(ctx context.Context, nodeEndpoint string, timeout time.Duration) error {
	log.Info().
		Str("node", nodeEndpoint).
		Dur("timeout", timeout).
		Msg("Waiting for node to reboot and come back online")

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Wait a bit for the node to start rebooting
	time.Sleep(30 * time.Second)

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for node %s to come back online", nodeEndpoint)
		case <-ticker.C:
			// Try to connect to the node
			nodeClient, err := c.CreateNodeClient(nodeEndpoint)
			if err != nil {
				log.Debug().Str("node", nodeEndpoint).Err(err).Msg("Node not ready yet")
				continue
			}

			// Try to get version to verify the node is responsive
			versionCtx, versionCancel := context.WithTimeout(ctx, 10*time.Second)
			_, err = nodeClient.Version(versionCtx)
			versionCancel()
			nodeClient.Close()

			if err != nil {
				log.Debug().Str("node", nodeEndpoint).Err(err).Msg("Node not ready yet")
				continue
			}

			log.Info().Str("node", nodeEndpoint).Msg("Node is back online")
			return nil
		}
	}
}
