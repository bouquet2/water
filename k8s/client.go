package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps the Kubernetes client with additional functionality
type Client struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
}

var (
	// Global client instance
	globalClient *Client
	clientOnce   sync.Once
	clientMutex  sync.RWMutex
)

// NewClient creates a new Kubernetes client
func NewClient() (*Client, error) {
	return NewClientWithKubeconfig("")
}

// NewClientWithKubeconfig creates a new Kubernetes client with a specific kubeconfig path
func NewClientWithKubeconfig(kubeconfigPath string) (*Client, error) {
	// Try to get in-cluster config first
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Debug().Err(err).Msg("Not running in cluster, trying kubeconfig")

		// Fall back to kubeconfig
		kubeconfig := kubeconfigPath
		if kubeconfig == "" {
			kubeconfig = getKubeconfigPath()
		}
		log.Info().Str("kubeconfig_path", kubeconfig).Msg("Creating Kubernetes client")

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
		}
	} else {
		log.Info().Msg("Creating Kubernetes client using in-cluster config")
	}

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset: %w", err)
	}

	log.Info().Msg("Kubernetes client created successfully")

	return &Client{
		clientset: clientset,
		config:    config,
	}, nil
}

// getKubeconfigPath returns the path to the kubeconfig file
func getKubeconfigPath() string {
	// Check KUBECONFIG environment variable
	if kubeconfig := os.Getenv("KUBECONFIG"); kubeconfig != "" {
		return kubeconfig
	}

	// Default to ~/.kube/config
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Debug().Err(err).Msg("Failed to get home directory, using current directory")
		return "config"
	}

	return filepath.Join(homeDir, ".kube", "config")
}

// GetSharedClient returns a shared Kubernetes client instance
func GetSharedClient() (*Client, error) {
	return GetSharedClientWithKubeconfig("")
}

// GetSharedClientWithKubeconfig returns a shared Kubernetes client instance with a specific kubeconfig path
func GetSharedClientWithKubeconfig(kubeconfigPath string) (*Client, error) {
	clientMutex.RLock()
	if globalClient != nil {
		defer clientMutex.RUnlock()
		return globalClient, nil
	}
	clientMutex.RUnlock()

	clientMutex.Lock()
	defer clientMutex.Unlock()

	// Double-check after acquiring write lock
	if globalClient != nil {
		return globalClient, nil
	}

	var err error
	globalClient, err = NewClientWithKubeconfig(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	return globalClient, nil
}

// InitializeClient initializes the global client with a specific kubeconfig path
// This should be called early in the application lifecycle
func InitializeClient(kubeconfigPath string) error {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	var err error
	globalClient, err = NewClientWithKubeconfig(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to initialize Kubernetes client: %w", err)
	}

	return nil
}

// IsKubernetesAPIAvailable checks if we can connect to the Kubernetes API
func IsKubernetesAPIAvailable(ctx context.Context) bool {
	client, err := GetSharedClient()
	if err != nil {
		log.Debug().Err(err).Msg("Failed to create Kubernetes client")
		return false
	}

	// Try to get server version to test connectivity
	_, err = client.clientset.Discovery().ServerVersion()
	if err != nil {
		log.Debug().Err(err).Msg("Failed to connect to Kubernetes API")
		return false
	}

	return true
}

// GetKubernetesVersion retrieves the Kubernetes version from the cluster
func GetKubernetesVersion(ctx context.Context) (string, error) {
	client, err := GetSharedClient()
	if err != nil {
		return "", fmt.Errorf("failed to get Kubernetes client: %w", err)
	}

	version, err := client.clientset.Discovery().ServerVersion()
	if err != nil {
		return "", fmt.Errorf("failed to get server version: %w", err)
	}

	return version.GitVersion, nil
}

// GetNodeNames retrieves the names of all nodes in the cluster
func GetNodeNames(ctx context.Context) ([]string, error) {
	client, err := GetSharedClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes client: %w", err)
	}

	nodes, err := client.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	var nodeNames []string
	for _, node := range nodes.Items {
		nodeNames = append(nodeNames, node.Name)
	}

	return nodeNames, nil
}

// IsControlPlaneNode checks if a node is a control plane node
func IsControlPlaneNode(ctx context.Context, nodeName string) (bool, error) {
	client, err := GetSharedClient()
	if err != nil {
		return false, fmt.Errorf("failed to get Kubernetes client: %w", err)
	}

	node, err := client.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	// Check for control plane labels
	labels := node.Labels
	if labels == nil {
		return false, nil
	}

	// Common control plane labels
	controlPlaneLabels := []string{
		"node-role.kubernetes.io/control-plane",
		"node-role.kubernetes.io/master",
		"kubernetes.io/role=master",
	}

	for _, label := range controlPlaneLabels {
		if _, exists := labels[label]; exists {
			return true, nil
		}
	}

	return false, nil
}

// GetNodeEndpoint retrieves the endpoint (IP address) for a node
func GetNodeEndpoint(ctx context.Context, nodeName string) (string, error) {
	client, err := GetSharedClient()
	if err != nil {
		return "", fmt.Errorf("failed to get Kubernetes client: %w", err)
	}

	node, err := client.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	// Try to get the internal IP first
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			return address.Address, nil
		}
	}

	// Fall back to external IP if internal IP is not available
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeExternalIP {
			return address.Address, nil
		}
	}

	// Fall back to hostname if no IP addresses are available
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeHostName {
			return address.Address, nil
		}
	}

	return "", fmt.Errorf("no suitable endpoint found for node %s", nodeName)
}

// GetNodeInfo retrieves detailed information about a specific node
func GetNodeInfo(ctx context.Context, nodeName string) (*NodeInfo, error) {
	client, err := GetSharedClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes client: %w", err)
	}

	node, err := client.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	// Check if node is ready
	ready := false
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
			ready = true
			break
		}
	}

	// Check if it's a control plane node
	isControlPlane, err := IsControlPlaneNode(ctx, nodeName)
	if err != nil {
		log.Debug().Err(err).Str("node", nodeName).Msg("Failed to determine if node is control plane")
		isControlPlane = false
	}

	return &NodeInfo{
		Name:           node.Name,
		Ready:          ready,
		IsControlPlane: isControlPlane,
		KubeletVersion: node.Status.NodeInfo.KubeletVersion,
		OSImage:        node.Status.NodeInfo.OSImage,
		Architecture:   node.Status.NodeInfo.Architecture,
	}, nil
}

// NodeInfo holds detailed information about a Kubernetes node
type NodeInfo struct {
	Name           string
	Ready          bool
	IsControlPlane bool
	KubeletVersion string
	OSImage        string
	Architecture   string
}

// GetClusterInfo retrieves comprehensive cluster information
func GetClusterInfo(ctx context.Context) (*ClusterInfo, error) {
	client, err := GetSharedClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get Kubernetes client: %w", err)
	}

	// Get Kubernetes version
	version, err := client.clientset.Discovery().ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("failed to get server version: %w", err)
	}

	// Get all nodes
	nodes, err := client.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	var nodeInfos []NodeInfo
	for _, node := range nodes.Items {
		// Check if node is ready
		ready := false
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				ready = true
				break
			}
		}

		// Check if it's a control plane node
		isControlPlane, err := IsControlPlaneNode(ctx, node.Name)
		if err != nil {
			log.Debug().Err(err).Str("node", node.Name).Msg("Failed to determine if node is control plane")
			isControlPlane = false
		}

		nodeInfos = append(nodeInfos, NodeInfo{
			Name:           node.Name,
			Ready:          ready,
			IsControlPlane: isControlPlane,
			KubeletVersion: node.Status.NodeInfo.KubeletVersion,
			OSImage:        node.Status.NodeInfo.OSImage,
			Architecture:   node.Status.NodeInfo.Architecture,
		})
	}

	return &ClusterInfo{
		K8sVersion: version.GitVersion,
		Nodes:      nodeInfos,
	}, nil
}

// ClusterInfo holds comprehensive information about the Kubernetes cluster
type ClusterInfo struct {
	K8sVersion string
	Nodes      []NodeInfo
}

// WaitForNodesReady waits for all nodes to be in Ready state
func WaitForNodesReady(ctx context.Context, timeout int) error {
	client, err := GetSharedClient()
	if err != nil {
		return fmt.Errorf("failed to get Kubernetes client: %w", err)
	}

	log.Info().Int("timeout_seconds", timeout).Msg("Waiting for all nodes to be ready")

	// Create a context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for nodes to be ready")
		default:
			nodes, err := client.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			if err != nil {
				log.Debug().Err(err).Msg("Failed to list nodes, retrying...")
				time.Sleep(5 * time.Second)
				continue
			}

			allReady := true
			for _, node := range nodes.Items {
				ready := false
				for _, condition := range node.Status.Conditions {
					if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
						ready = true
						break
					}
				}
				if !ready {
					allReady = false
					log.Debug().Str("node", node.Name).Msg("Node not ready yet")
					break
				}
			}

			if allReady {
				log.Info().Msg("All nodes are ready")
				return nil
			}

			time.Sleep(10 * time.Second)
		}
	}
}
