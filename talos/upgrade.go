package talos

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

func (c *Client) UpgradeNode(ctx context.Context, nodeEndpoint, imageID string) error {
	log.Info().
		Str("node", nodeEndpoint).
		Str("image_id", imageID).
		Msg("Starting Talos upgrade on single node")

	nodeVersion, err := c.getNodeTalosVersion(ctx, nodeEndpoint)
	if err != nil {
		log.Warn().
			Str("node", nodeEndpoint).
			Err(err).
			Msg("Failed to get node version, assuming legacy API support")
		return c.upgradeLegacy(ctx, nodeEndpoint, imageID)
	}

	supportsLifecycle, err := SupportsLifecycleUpgrade(nodeVersion)
	if err != nil {
		log.Warn().
			Str("node", nodeEndpoint).
			Str("version", nodeVersion).
			Err(err).
			Msg("Failed to check API support, using legacy upgrade")
		return c.upgradeLegacy(ctx, nodeEndpoint, imageID)
	}

	if supportsLifecycle {
		log.Info().
			Str("node", nodeEndpoint).
			Str("version", nodeVersion).
			Msg("Using LifecycleService.Upgrade API")

		reporter := NewProgressReporter()
		return c.UpgradeViaLifecycleService(ctx, nodeEndpoint, imageID, reporter)
	}

	log.Info().
		Str("node", nodeEndpoint).
		Str("version", nodeVersion).
		Msg("Using legacy MachineService.Upgrade API")

	return c.upgradeLegacy(ctx, nodeEndpoint, imageID)
}

func (c *Client) upgradeLegacy(ctx context.Context, nodeEndpoint, imageID string) error {
	nodeClient, err := c.CreateNodeClient(nodeEndpoint)
	if err != nil {
		return fmt.Errorf("failed to create client for node %s: %w", nodeEndpoint, err)
	}
	defer nodeClient.Close()

	upgradeResp, err := nodeClient.Upgrade(ctx, imageID, false, false)
	if err != nil {
		return fmt.Errorf("failed to initiate Talos upgrade on node %s: %w", nodeEndpoint, err)
	}

	log.Debug().
		Str("node", nodeEndpoint).
		Interface("upgrade_response", upgradeResp).
		Msg("Legacy upgrade initiated")

	return nil
}

func (c *Client) CreateNodeClient(nodeEndpoint string) (*client.Client, error) {
	opts := []client.OptionFunc{
		client.WithConfig(c.clientConfig),
		client.WithEndpoints(nodeEndpoint),
	}

	nodeClient, err := client.New(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create node client: %w", err)
	}

	return nodeClient, nil
}

func (c *Client) WaitForNodeReboot(ctx context.Context, nodeEndpoint string, timeout time.Duration) error {
	log.Info().
		Str("node", nodeEndpoint).
		Dur("timeout", timeout).
		Msg("Waiting for node to reboot and come back online")

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	time.Sleep(30 * time.Second)

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for node %s to come back online", nodeEndpoint)
		case <-ticker.C:
			nodeClient, err := c.CreateNodeClient(nodeEndpoint)
			if err != nil {
				log.Debug().Str("node", nodeEndpoint).Err(err).Msg("Node not ready yet")
				continue
			}

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
