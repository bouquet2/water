package talos

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/config"
)

// Client wraps the Talos machinery client with additional functionality
type Client struct {
	client       *client.Client
	ctx          context.Context
	clientConfig *config.Config
}

// ClusterInfo holds information about the current cluster state
type ClusterInfo struct {
	TalosVersion string
	K8sVersion   string
	Nodes        []NodeInfo
}

// NodeInfo holds information about individual nodes
type NodeInfo struct {
	Name           string
	TalosVersion   string
	Ready          bool
	IsControlPlane bool
	Endpoint       string // IP address or hostname for Talos API
}

// NewClient creates a new Talos client
func NewClient(configPath string) (*Client, error) {
	log.Info().Str("config_path", configPath).Msg("Creating Talos client")

	// Load Talos client configuration
	clientConfig, err := config.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load Talos client config: %w", err)
	}

	// Create client options
	opts := []client.OptionFunc{
		client.WithConfig(clientConfig),
	}

	// Create the client
	c, err := client.New(context.Background(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create Talos client: %w", err)
	}

	log.Info().Msg("Talos client created successfully")

	return &Client{
		client:       c,
		ctx:          context.Background(),
		clientConfig: clientConfig,
	}, nil
}

// Close closes the Talos client connection
func (c *Client) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// GetClient returns the underlying Talos client
func (c *Client) GetClient() *client.Client {
	return c.client
}

// GetConfig returns the Talos client configuration
func (c *Client) GetConfig() *config.Config {
	return c.clientConfig
}
