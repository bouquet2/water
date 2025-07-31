package talos

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/config"
	"google.golang.org/protobuf/types/known/emptypb"
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

// GetTalosEndpoints returns the configured Talos node endpoints from the client configuration
func (c *Client) GetTalosEndpoints() []string {
	if c.clientConfig == nil {
		return nil
	}

	// Get the current context
	currentContext := c.clientConfig.Contexts[c.clientConfig.Context]
	if currentContext == nil {
		return nil
	}

	// Return the nodes list if available, otherwise fall back to endpoints
	if len(currentContext.Nodes) > 0 {
		return currentContext.Nodes
	}

	return currentContext.Endpoints
}

// nodeEndpointCache caches the mapping between node names and Talos endpoints
var nodeEndpointCache map[string]string

// GetNodeEndpointFromTalos attempts to get the correct Talos endpoint for a given node name
// by building a cache of node name to endpoint mappings
func (c *Client) GetNodeEndpointFromTalos(ctx context.Context, nodeName string) (string, error) {
	// Build cache if it doesn't exist
	if nodeEndpointCache == nil {
		err := c.buildNodeEndpointCache(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to build node endpoint cache: %w", err)
		}
	}

	// Look up the endpoint for this node
	if endpoint, exists := nodeEndpointCache[nodeName]; exists {
		log.Debug().
			Str("node", nodeName).
			Str("endpoint", endpoint).
			Msg("Found cached Talos endpoint for node")
		return endpoint, nil
	}

	return "", fmt.Errorf("no Talos endpoint found for node %s", nodeName)
}

// buildNodeEndpointCache builds a cache mapping node names to Talos endpoints
// by querying all endpoints and using the version response order
func (c *Client) buildNodeEndpointCache(ctx context.Context) error {
	endpoints := c.GetTalosEndpoints()
	if len(endpoints) == 0 {
		return fmt.Errorf("no Talos endpoints configured")
	}

	log.Info().
		Strs("endpoints", endpoints).
		Msg("Building node endpoint cache from Talos configuration")

	// Initialize the cache
	nodeEndpointCache = make(map[string]string)

	// Query each endpoint individually to get hostname
	for _, endpoint := range endpoints {
		hostname, err := c.getHostnameFromEndpoint(ctx, endpoint)
		if err != nil {
			log.Warn().
				Str("endpoint", endpoint).
				Err(err).
				Msg("Failed to get hostname from endpoint")
			continue
		}

		nodeEndpointCache[hostname] = endpoint
		log.Info().
			Str("hostname", hostname).
			Str("endpoint", endpoint).
			Msg("Cached node endpoint mapping")
	}

	if len(nodeEndpointCache) == 0 {
		return fmt.Errorf("failed to build any node endpoint mappings")
	}

	log.Info().
		Int("mappings", len(nodeEndpointCache)).
		Msg("Successfully built node endpoint cache")

	return nil
}

// getHostnameFromEndpoint gets the hostname from a specific Talos endpoint
func (c *Client) getHostnameFromEndpoint(ctx context.Context, endpoint string) (string, error) {
	// Create a client for this specific endpoint
	nodeClient, err := c.CreateNodeClient(endpoint)
	if err != nil {
		return "", fmt.Errorf("failed to create client for endpoint %s: %w", endpoint, err)
	}
	defer nodeClient.Close()

	// Use the Talos API to get the hostname
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Try to get hostname using the Talos MachineClient Hostname method
	hostnameResp, err := nodeClient.MachineClient.Hostname(timeoutCtx, &emptypb.Empty{})
	if err != nil {
		return "", fmt.Errorf("failed to get hostname from endpoint %s: %w", endpoint, err)
	}

	// Extract hostname from the response
	for _, message := range hostnameResp.Messages {
		if message.Hostname != "" {
			log.Debug().
				Str("endpoint", endpoint).
				Str("hostname", message.Hostname).
				Msg("Successfully retrieved hostname from Talos endpoint")
			return message.Hostname, nil
		}
	}

	return "", fmt.Errorf("no hostname found in response from endpoint %s", endpoint)
}
