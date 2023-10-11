package scenarios

import (
	"bytes"
	"context"

	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/kustomize"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type reloader struct{}

var _ Scenario = (*reloader)(nil)

func (r reloader) Execute(ctx context.Context, env *environment.Env) error {
	base, err := AbsolutePathToBase()
	if err != nil {
		return err
	}

	// add base helm repositories.
	k := kustomize.New(base, nil)
	if err = k.Build(); err != nil {
		return err
	}
	out, err := k.Output()
	if err != nil {
		return err
	}

	// apply helm repositories to the cluster
	buf := bytes.NewBuffer(out)
	// default buffer size is 1MB
	dec := yaml.NewYAMLOrJSONDecoder(buf, 1<<20)
	obj := unstructured.Unstructured{}
	if err = dec.Decode(&obj); err != nil { // EOF?
		return err
	}

	genericClient, err := client.New(env.K8sClient.Config(), client.Options{})
	if err != nil {
		return err
	}

	err = genericClient.Patch(ctx, &obj, client.Apply, client.ForceOwnership)
	if err != nil {
		return err
	}

	return nil
}
