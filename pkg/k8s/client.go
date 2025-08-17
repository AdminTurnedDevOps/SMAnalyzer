package k8s

import (
	"context"
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	Clientset  *kubernetes.Clientset
	RestConfig *rest.Config
}

func NewClient() (*Client, error) {
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &Client{
		Clientset:  clientset,
		RestConfig: config,
	}, nil
}

func (c *Client) CheckConnection(ctx context.Context) error {
	_, err := c.Clientset.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to connect to Kubernetes cluster: %w", err)
	}
	return nil
}