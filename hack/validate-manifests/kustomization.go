package main

import (
	"fmt"
	"path/filepath"
	"regexp"

	fluxhelmv2beta1 "github.com/fluxcd/helm-controller/api/v2beta1"
	fluxkustomizev1beta2 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	"github.com/fluxcd/kustomize-controller/controllers"
	"github.com/fluxcd/pkg/apis/meta"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resource"
	kustypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

type FluxKustomizationValidator struct {
	kustomization *fluxkustomizev1beta2.Kustomization
	checked       bool
	errors        []error
}

func (v *FluxKustomizationValidator) Name() string {
	return "Flux Kustomization " + v.kustomization.Name
}

func (v *FluxKustomizationValidator) Check(ctx *Context) (done bool, errs []error) {
	defer func() {
		if done {
			ref := metasToRef(v.kustomization.TypeMeta, v.kustomization.ObjectMeta)
			ctx.MarkChecked(ref)
		}
	}()

	for _, dep := range v.kustomization.Spec.DependsOn {
		ref := NewObjectRef(
			v.kustomization.APIVersion, v.kustomization.Kind,
			dep.Namespace, dep.Name,
		)
		if ref.Metadata.Namespace == "" {
			ref.Metadata.Namespace = v.kustomization.Namespace
		}
		if !ctx.IsChecked(ref) {
			return false, nil
		}
	}

	if !v.checked {
		v.checked = true
		if v.kustomization.Spec.SourceRef.Kind != "GitRepository" || v.kustomization.Spec.SourceRef.Name != "management" {
			return true, []error{fmt.Errorf("Can't handle Flux customizations pointing to other repos yet")}
		}
		kPath := filepath.Join(ctx.RootDir, v.kustomization.Spec.Path)
		err := controllers.NewGenerator(*v.kustomization).WriteFile(kPath)
		if err != nil {
			return true, []error{err}
		}

		ctx.V(1).Infof("Validating Flux Kustomization %q", v.kustomization.Name)
		resources, err := renderKustomization(kPath)
		if err != nil {
			return true, []error{err}
		}
		for _, resource := range resources {
			resourceYaml, err := resource.AsYAML()
			if err != nil {
				v.errors = append(v.errors, err)
				continue
			}
			resourceYaml = replaceVariables(resourceYaml, ctx.Config.ReplacementVars)
			errs := validateResource(ctx, resourceYaml)
			if errs != nil {
				v.errors = append(v.errors, errs...)
			}
		}

		if v.kustomization.Spec.Wait {
			v.kustomization.Spec.HealthChecks = nil
			for _, resource := range resources {
				switch resource.GetKind() {
				case fluxkustomizev1beta2.KustomizationKind, fluxhelmv2beta1.HelmReleaseKind:
					dep := meta.NamespacedObjectKindReference{
						APIVersion: resource.GetApiVersion(),
						Kind:       resource.GetKind(),
						Namespace:  resource.GetNamespace(),
						Name:       resource.GetName(),
					}
					v.kustomization.Spec.HealthChecks = append(v.kustomization.Spec.HealthChecks, dep)
				}
			}
		}
	}

	for _, dep := range v.kustomization.Spec.HealthChecks {
		ref := NewObjectRef(
			dep.APIVersion, dep.Kind,
			dep.Namespace, dep.Name,
		)
		if !ctx.IsChecked(ref) {
			return false, nil
		}
	}
	return true, v.errors
}

var variableRegexp = regexp.MustCompile(`\${` + "(.+?)(?::=(.+?))?" + `}`)

// replaceVariables replaces variables ${varName}, with support for defaults ${varName:=defaultValue}
func replaceVariables(yaml []byte, variables map[string]string) []byte {
	return variableRegexp.ReplaceAllFunc(yaml, func(b []byte) []byte {
		matches := variableRegexp.FindStringSubmatch(string(b))
		varName := matches[1]
		value := variables[varName]
		if value == "" && len(matches) == 3 {
			value = matches[2]
		}
		return []byte(value)
	})
}

func renderKustomization(path string) ([]*resource.Resource, error) {
	buildOptions := &krusty.Options{
		LoadRestrictions: kustypes.LoadRestrictionsNone,
		PluginConfig:     kustypes.DisabledPluginConfig(),
	}
	k := krusty.MakeKustomizer(buildOptions)
	fs := filesys.MakeFsOnDisk()
	resMap, err := k.Run(fs, path)
	if err != nil {
		return nil, err
	}
	return resMap.Resources(), nil
}

func checkKustomization(ctx *Context, path string) (errors []error) {
	resources, err := renderKustomization(path)
	if err != nil {
		return []error{err}
	}
	for _, resource := range resources {
		resourceYaml, err := resource.AsYAML()
		if err != nil {
			errors = append(errors, err)
			continue
		}
		resourceYaml = replaceVariables(resourceYaml, ctx.Config.ReplacementVars)
		errs := validateResource(ctx, resourceYaml)
		if errs != nil {
			errors = append(errors, errs...)
		}
	}
	return
}
