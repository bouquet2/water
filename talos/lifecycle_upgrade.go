package talos

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/rs/zerolog/log"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

func ParseContainerdInstance(namespace string) string {
	switch namespace {
	case "system":
		return "system"
	case "cri":
		return "cri"
	case "inmem":
		return "inmem"
	default:
		return "system"
	}
}

func endpointFromClient(nodeClient *client.Client) (string, error) {
	endpoints := nodeClient.GetEndpoints()
	if len(endpoints) == 0 {
		return "", fmt.Errorf("no endpoints available for node client")
	}
	return endpoints[0], nil
}

func (c *Client) UpgradeViaLifecycleService(ctx context.Context, nodeEndpoint, imageRef string, reporter *ProgressReporter) error {
	nodeClient, err := c.CreateNodeClient(nodeEndpoint)
	if err != nil {
		return fmt.Errorf("failed to create node client for %s: %w", nodeEndpoint, err)
	}
	defer nodeClient.Close()

	if err := c.imagePullInternal(ctx, nodeClient, imageRef, reporter); err != nil {
		return fmt.Errorf("image pull failed for %s: %w", nodeEndpoint, err)
	}

	if err := c.upgradeInternal(ctx, nodeClient, imageRef, reporter); err != nil {
		return fmt.Errorf("upgrade failed for %s: %w", nodeEndpoint, err)
	}

	return nil
}

func (c *Client) imagePullInternal(ctx context.Context, nodeClient *client.Client, imageRef string, reporter *ProgressReporter) error {
	endpoint, err := endpointFromClient(nodeClient)
	if err != nil {
		return err
	}

	log.Info().Str("node", endpoint).Str("image", imageRef).Msg("Pulling image via ImageService")

	if reporter != nil {
		reporter.UpdateWithMessage(endpoint, "Pulling Image", 10, imageRef)
	}

	if err := nodeClient.ImagePull(ctx, common.ContainerdNamespace_NS_SYSTEM, imageRef); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", imageRef, err)
	}

	if reporter != nil {
		reporter.Update(endpoint, "Image Pulled", 20)
	}

	return nil
}

func (c *Client) upgradeInternal(ctx context.Context, nodeClient *client.Client, imageRef string, reporter *ProgressReporter) error {
	endpoint, err := endpointFromClient(nodeClient)
	if err != nil {
		return err
	}

	log.Info().Str("node", endpoint).Str("image", imageRef).Msg("Starting lifecycle upgrade")

	if reporter != nil {
		reporter.Update(endpoint, "Starting Upgrade", 30)
	}

	req := &machine.LifecycleServiceUpgradeRequest{
		Containerd: &common.ContainerdInstance{
			Driver:    common.ContainerDriver_CONTAINERD,
			Namespace: common.ContainerdNamespace_NS_SYSTEM,
		},
		Source: &machine.InstallArtifactsSource{
			ImageName: imageRef,
		},
	}

	stream, err := nodeClient.LifecycleClient.Upgrade(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to start upgrade stream: %w", err)
	}

	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("error receiving upgrade progress: %w", err)
		}

		if resp.Progress != nil {
			switch respType := resp.Progress.Response.(type) {
			case *machine.LifecycleServiceInstallProgress_Message:
				if reporter != nil {
					reporter.UpdateWithMessage(endpoint, "Installing", 50, respType.Message)
				}
			case *machine.LifecycleServiceInstallProgress_ExitCode:
				if respType.ExitCode != 0 {
					return fmt.Errorf("upgrade failed with exit code %d", respType.ExitCode)
				}
				if reporter != nil {
					reporter.Update(endpoint, "Upgrade Complete", 100)
				}
			}
		}
	}

	log.Info().Str("node", endpoint).Msg("Lifecycle upgrade completed")
	return nil
}
