package main

import (
	"bytes"
	"path/filepath"

	fluxkustomizev1beta2 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	"github.com/fluxcd/kustomize-controller/controllers"
	"github.com/fluxcd/pkg/runtime/dependency"
	"sigs.k8s.io/kustomize/api/krusty"
	kustypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

func handleFluxCustomization(ctx *Context, k *fluxkustomizev1beta2.Kustomization) {
	ctx.FluxKustomizations[k.Name] = k
}

func checkFluxKustomizations(ctx *Context) {
	kustomizationList := make([]dependency.Dependent, 0, len(ctx.FluxKustomizations))
	for _, kustomization := range ctx.FluxKustomizations {
		kustomizationList = append(kustomizationList, kustomization)
	}
	sorted, err := dependency.Sort(kustomizationList)
	if err != nil {
		ctx.Error(err, "")
		return
	}
	for _, ref := range sorted {
		ctx.StartOperation("Kustomization " + ref.Name)
		checkFluxKustomization(ctx, ctx.FluxKustomizations[ref.Name], "")
		ctx.EndOperation(true)
	}
}

func checkFluxKustomization(ctx *Context, kustomization *fluxkustomizev1beta2.Kustomization, path string) {
	if kustomization.Spec.SourceRef.Kind != "GitRepository" || kustomization.Spec.SourceRef.Name != "management" {
		ctx.Error(nil, "Can't handle Flux customizations pointing to other repos yet")
		return
	}
	kPath := filepath.Join(ctx.TempDir, kustomization.Spec.Path)
	err := controllers.NewGenerator(*kustomization).WriteFile(kPath)
	if err != nil {
		ctx.Error(err, "")
		return
	}
	checkKustomization(ctx, kPath)
}

func checkKustomization(ctx *Context, path string) {
	relPath, _ := filepath.Rel(ctx.TempDir, path)
	ctx.V(1).Infof("Validating Kustomization in %q", relPath)
	buildOptions := &krusty.Options{
		LoadRestrictions: kustypes.LoadRestrictionsNone,
		PluginConfig:     kustypes.DisabledPluginConfig(),
	}
	k := krusty.MakeKustomizer(buildOptions)
	fs := filesys.MakeFsOnDisk()
	resMap, err := k.Run(fs, path)
	if err != nil {
		ctx.Error(err, "")
		return
	}

	resources := resMap.Resources()
	for _, resource := range resources {
		resourceYaml, err := resource.AsYAML()
		if err != nil {
			ctx.Error(err, "")
			continue
		}
		for k, v := range ctx.Config.ReplacementVars {
			resourceYaml = bytes.ReplaceAll(resourceYaml, []byte("${"+k+"}"), []byte(v))
		}
		validateResource(ctx, resourceYaml)
	}
}
