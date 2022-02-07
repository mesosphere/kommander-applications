package main

import (
	fluxhelmv2beta1 "github.com/fluxcd/helm-controller/api/v2beta1"
	fluxkustomizev1beta2 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	fluxsourcesv1beta1 "github.com/fluxcd/source-controller/api/v1beta1"
	"github.com/mesosphere/dkp-cli-runtime/core/output"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
)

type Context struct {
	*Registry
	output.Output
	Runner  *Runner
	Config  Config
	Decoder runtime.Decoder
	RootDir string
}

func NewContext(out output.Output, config Config) *Context {
	scheme := runtime.NewScheme()
	//nolint:errcheck
	{
		k8sscheme.AddToScheme(scheme)
		apiextensionsv1.AddToScheme(scheme)
		apiextensionsv1beta1.AddToScheme(scheme)
		apiregistrationv1.AddToScheme(scheme)
		apiregistrationv1beta1.AddToScheme(scheme)
		fluxhelmv2beta1.AddToScheme(scheme)
		fluxkustomizev1beta2.AddToScheme(scheme)
		fluxsourcesv1beta1.AddToScheme(scheme)
	}

	mutators := []serializer.CodecFactoryOptionsMutator{}
	if config.Strict {
		mutators = append(mutators, serializer.EnableStrict)
	}
	codecs := serializer.NewCodecFactory(scheme, mutators...)
	decoder := codecs.UniversalDeserializer()

	return &Context{
		Output:   out,
		Registry: NewRegistry(),
		Runner:   &Runner{},
		Config:   config,
		Decoder:  decoder,
	}
}
