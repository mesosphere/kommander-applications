package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"k8s.io/kube-openapi/pkg/validation/strfmt"
	"k8s.io/kube-openapi/pkg/validation/validate"
)

func addCRDv1(registry *Registry, crd *apiextensionsv1.CustomResourceDefinition) (errors []error) {
	for _, version := range crd.Spec.Versions {
		err := addCRDSchema(registry, metav1.TypeMeta{
			APIVersion: crd.Spec.Group + "/" + version.Name,
			Kind:       crd.Spec.Names.Kind,
		}, version.Schema.OpenAPIV3Schema)
		if err != nil {
			errors = append(errors, err)
		}
	}
	return
}

func addCRDv1beta1(registry *Registry, crd *apiextensionsv1beta1.CustomResourceDefinition) (errors []error) {
	for _, version := range crd.Spec.Versions {
		schema := &apiextensionsv1beta1.JSONSchemaProps{}
		if version.Schema != nil {
			schema = version.Schema.OpenAPIV3Schema
		} else if crd.Spec.Validation != nil {
			schema = crd.Spec.Validation.OpenAPIV3Schema
		}
		err := addCRDSchema(registry, metav1.TypeMeta{
			APIVersion: crd.Spec.Group + "/" + version.Name,
			Kind:       crd.Spec.Names.Kind,
		}, schema)
		if err != nil {
			errors = append(errors, err)
		}
	}
	return
}

func addCRDSchema(registry *Registry, typeMeta metav1.TypeMeta, schema interface{}) error {
	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		return err
	}
	specSchema := new(spec.Schema)
	err = json.Unmarshal(schemaJSON, schema)
	if err != nil {
		return err
	}
	registry.SetCRDSchema(typeMeta, specSchema)
	return nil
}

func validateResourceAgainstCRDs(ctx *Context, yamlData []byte) error {
	resource := map[string]interface{}{}
	err := yaml.Unmarshal(yamlData, &resource)
	if err != nil {
		return err
	}

	apiVersion := resource["apiVersion"].(string)
	kind := resource["kind"].(string)
	gvk := fmt.Sprintf("%s/%s", apiVersion, kind)
	if ctx.Config.SkipTypes[apiVersion] || ctx.Config.SkipTypes[gvk] {
		ctx.V(2).Infof("Validation of type %q skipped", gvk)
		return nil
	}
	ctx.V(2).Infof("Validating resource of type %q against CRD schema", gvk)
	crdSchema := ctx.GetCRDSchema(metav1.TypeMeta{APIVersion: apiVersion, Kind: kind})
	if crdSchema == nil {
		return fmt.Errorf("Resource type %q not found", gvk)
	}

	err = validate.AgainstSchema(crdSchema, resource, strfmt.Default)
	if err != nil {
		return fmt.Errorf("Custom resource %q does not match the CRD schema: %v\n%s", gvk, err, yamlData)
	}
	return nil
}

func loadAdditionalCRDs(ctx *Context) []error {
	kustomization := `
---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
`
	for _, crd := range ctx.Config.AdditionalCRDs {
		kustomization += fmt.Sprintf("  - %s\n", crd)
	}

	dir := filepath.Join(ctx.RootDir, "additional-crds")
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return []error{err}
	}
	err = os.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte(kustomization), 0644)
	if err != nil {
		return []error{err}
	}
	return checkKustomization(ctx, dir)
}
