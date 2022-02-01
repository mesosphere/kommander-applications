package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"k8s.io/kube-openapi/pkg/validation/strfmt"
	"k8s.io/kube-openapi/pkg/validation/validate"
)

func addCRDv1(ctx *Context, crd *apiextensionsv1.CustomResourceDefinition) {
	for _, version := range crd.Spec.Versions {
		addCRDSchema(ctx, fmt.Sprintf("%s/%s/%s", crd.Spec.Group, version.Name, crd.Spec.Names.Kind), version.Schema.OpenAPIV3Schema)
	}
}

func addCRDv1beta1(ctx *Context, crd *apiextensionsv1beta1.CustomResourceDefinition) {
	for _, version := range crd.Spec.Versions {
		schema := &apiextensionsv1beta1.JSONSchemaProps{}
		if version.Schema != nil {
			schema = version.Schema.OpenAPIV3Schema
		} else if crd.Spec.Validation != nil {
			schema = crd.Spec.Validation.OpenAPIV3Schema
		}
		addCRDSchema(ctx, fmt.Sprintf("%s/%s/%s", crd.Spec.Group, version.Name, crd.Spec.Names.Kind), schema)
	}
}

func addCRDSchema(ctx *Context, name string, schema interface{}) {
	ctx.V(1).Infof("Adding CRD %q", name)
	schemaJSON, err := json.Marshal(schema)
	if err != nil {
		ctx.Error(err, "")
		return
	}
	specSchema := new(spec.Schema)
	err = json.Unmarshal(schemaJSON, schema)
	if err != nil {
		ctx.Error(err, "")
		return
	}
	ctx.CRDSchemas[name] = specSchema
}

func validateResourceAgainstCRDs(ctx *Context, yamlData []byte) {
	resource := map[string]interface{}{}
	err := yaml.Unmarshal(yamlData, &resource)
	if err != nil {
		ctx.Error(err, "")
		return
	}

	apiVersion := resource["apiVersion"]
	kind := resource["kind"]
	gvk := fmt.Sprintf("%s/%s", apiVersion, kind)
	if ctx.Config.SkipTypes[fmt.Sprint(apiVersion)] || ctx.Config.SkipTypes[gvk] {
		ctx.V(2).Infof("Validation of type %q skipped", gvk)
		return
	}
	ctx.V(2).Infof("Validating resource of type %q against CRD schema", gvk)
	crdSchema, ok := ctx.CRDSchemas[gvk]
	if !ok {
		ctx.Errorf(nil, "Resource type %q not found", gvk)
		return
	}

	err = validate.AgainstSchema(crdSchema, resource, strfmt.Default)
	if err != nil {
		ctx.Errorf(nil, "Custom resource %q does not match the CRD schema: %v\n%s", gvk, err, yamlData)
		return
	}
}

func loadAdditionalCRDs(ctx *Context) {
	kustomization := `
---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
`
	for _, crd := range ctx.Config.AdditionalCRDs {
		kustomization += fmt.Sprintf("  - %s\n", crd)
	}

	dir := filepath.Join(ctx.TempDir, "additional-crds")
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		ctx.Error(err, "")
		return
	}
	err = os.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte(kustomization), 0644)
	if err != nil {
		ctx.Error(err, "")
		return
	}
	checkKustomization(ctx, dir)
}
