package appscenarios

import (
	"context"
	"github.com/mesosphere/kommander-applications/apptests/flux"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
	genericClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func SetupKindCluster() error {
	if ctx == nil {
		ctx = context.Background()
	}

	err := env.Provision(ctx)
	if err != nil {
		return err
	}

	k8sClient, err = genericClient.New(env.K8sClient.Config(), genericClient.Options{Scheme: flux.NewScheme()})
	if err != nil {
		return err
	}

	// Get a REST client for making http requests to pods
	gvk := schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}

	httpClient, err := rest.HTTPClientFor(env.K8sClient.Config())
	if err != nil {
		return err
	}

	restClientV1Pods, err = apiutil.RESTClientForGVK(gvk, false, env.K8sClient.Config(), serializer.NewCodecFactory(flux.NewScheme()), httpClient)
	if err != nil {
		return err
	}

	return nil
}
