// Package environment provides functions to manage, and configure environment for application specific testings.
package environment

import (
	"context"
	"errors"

	"github.com/mesosphere/kommander-applications/apptests/client"
	"github.com/mesosphere/kommander-applications/apptests/flux"
	"github.com/mesosphere/kommander-applications/apptests/kind"

	runclient "github.com/fluxcd/pkg/runtime/client"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

const (
	kommanderFluxNamespace = "kommander-flux"
	kommanderNamespace     = "kommander"
)

// Env holds the configuration and state for application specific testings.
// It contains the applications to test, the Kubernetes client, and the kind cluster.
type Env struct {
	// K8sClient is a reference to the Kubernetes client
	// This client is used to interact with the cluster built during the execution of the application specific testing.
	K8sClient *client.Client

	// Cluster is a dedicated instance of a kind cluster created for running an application specific test.
	Cluster *kind.Cluster
}

// Provision creates and configures the environment for application specific testings.
// It calls the provisionEnv function and assigns the returned references to the Environment fields.
// It returns an error if any of the steps fails.
func (e *Env) Provision(ctx context.Context) error {
	cluster, k8sClient, err := provisionEnv(ctx)
	if err != nil {
		// If the provisioning fails, it tries to destroy the cluster
		// and returns a joined error.
		err2 := e.Destroy(ctx)
		if err2 != nil {
			return errors.Join(err, err2)
		}
	}

	e.K8sClient = k8sClient
	e.Cluster = cluster

	return nil
}

// Destroy delete the provisioned cluster if existed.
func (e *Env) Destroy(ctx context.Context) error {
	if e.Cluster != nil {
		return e.Cluster.Delete(ctx)
	}

	return nil
}

// provisionEnv creates a kind cluster, a Kubernetes client, and installs flux components on the cluster.
// It returns the created cluster and client references, or an error if any of the steps fails.
func provisionEnv(ctx context.Context) (*kind.Cluster, *client.Client, error) {
	cluster, err := kind.CreateCluster(ctx, "")
	if err != nil {
		return nil, nil, err
	}

	c, err := client.NewClient(cluster.KubeconfigFilePath())
	if err != nil {
		return nil, nil, err
	}

	// creating the necessary namespaces
	namespaces := []corev1.Namespace{
		{ObjectMeta: metav1.ObjectMeta{Name: kommanderNamespace}},
		{ObjectMeta: metav1.ObjectMeta{Name: kommanderFluxNamespace}},
	}
	for _, ns := range namespaces {
		if _, err = c.Clientset().CoreV1().
			Namespaces().
			Create(ctx, &ns, metav1.CreateOptions{}); err != nil {
			return nil, nil, err
		}
	}

	err = flux.Install(ctx, flux.Options{
		KubeconfigArgs:    genericclioptions.NewConfigFlags(true),
		KubeclientOptions: new(runclient.Options),
		Namespace:         kommanderFluxNamespace,
		Components:        []string{"source-controller", "kustomize-controller", "helm-controller"},
	})

	return cluster, c, err
}
