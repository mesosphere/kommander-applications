package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"

	fluxhelmv2beta1 "github.com/fluxcd/helm-controller/api/v2beta1"
	fluxkustomizev1beta2 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	fluxsourcesv1beta1 "github.com/fluxcd/source-controller/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func validateResource(ctx *Context, resourceYaml []byte) (errors []error) {
	r := yaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(resourceYaml)))
	for {
		doc, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errors = append(errors, err)
			return
		}

		obj, _, err := ctx.Decoder.Decode(doc, nil, nil)
		if err != nil {
			if runtime.IsNotRegisteredError(err) {
				err := validateResourceAgainstCRDs(ctx, doc)
				if err != nil {
					errors = append(errors, err)
				}
				markChecked(ctx.Registry, doc)
				continue
			}
			// not a manifest, e.g. comments before yaml separator
			if runtime.IsMissingKind(err) {
				continue
			}
			errors = append(errors, fmt.Errorf("%v\n%s", err, doc))
			continue
		}

		if x, ok := obj.(metav1.Object); ok {
			ctx.V(3).Infof("Validating resource %q (%s)", x.GetName(), obj.GetObjectKind().GroupVersionKind())
		} else {
			ctx.V(3).Infof("Validating resource of type %q", obj.GetObjectKind().GroupVersionKind())
		}

		switch obj := obj.(type) {
		case *v1.ConfigMap:
			ctx.SetObject(metasToRef(obj.TypeMeta, obj.ObjectMeta), obj)
			markChecked(ctx.Registry, doc)
		case *apiextensionsv1.CustomResourceDefinition:
			ctx.V(1).Infof("Adding CRD %q", obj.Name)
			errs := addCRDv1(ctx.Registry, obj)
			if errs != nil {
				errors = append(errors, errs...)
			}
		case *apiextensionsv1beta1.CustomResourceDefinition:
			ctx.V(1).Infof("Adding CRD %q", obj.Name)
			errs := addCRDv1beta1(ctx.Registry, obj)
			if errs != nil {
				errors = append(errors, errs...)
			}
		case *fluxsourcesv1beta1.HelmRepository:
			addHelmRepo(ctx, obj)
		case *fluxhelmv2beta1.HelmRelease:
			ref := metasToRef(obj.TypeMeta, obj.ObjectMeta)
			if ctx.GetObject(ref) == nil {
				ctx.SetObject(ref, obj)
				ctx.Runner.AddCheck(&HelmReleaseValidator{
					helmRelease: obj,
				})
			}
		case *fluxkustomizev1beta2.Kustomization:
			ref := metasToRef(obj.TypeMeta, obj.ObjectMeta)
			if ctx.GetObject(ref) == nil {
				ctx.SetObject(ref, obj)
				ctx.Runner.AddCheck(&FluxKustomizationValidator{
					kustomization: obj,
				})
			}
		default:
			markChecked(ctx.Registry, doc)
		}
	}
	return
}

func markChecked(registry *Registry, yamlResource []byte) {
	ref := ObjectRef{}
	_ = yaml.Unmarshal(yamlResource, &ref)
	registry.MarkChecked(ref)
}
