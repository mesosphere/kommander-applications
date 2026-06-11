// Package client provides a client for interacting with Kubernetes clusters.
// It wraps the kubernetes client-go library and exposes a simple interface
// for creating and using a kubernetes client.
package client

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	config    *rest.Config
	clientset *kubernetes.Clientset
}

// NewClient creates a new kubernetes client.
func NewClient(kubeconfigPath string) (*Client, error) {
	// build the config from the kubeconfig path
	conf, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	// create the clientset with the config and the context
	clientset, err := kubernetes.NewForConfig(conf)
	if err != nil {
		return nil, err
	}

	return &Client{
		config:    conf,
		clientset: clientset,
	}, nil
}

// Config returns the client config.
func (c *Client) Config() *rest.Config {
	return c.config
}

// Clientset returns the kubernetes clientset.
func (c *Client) Clientset() *kubernetes.Clientset {
	return c.clientset
}
