package main

import (
	"fmt"
	"path/filepath"

	fluxhelmv2beta1 "github.com/fluxcd/helm-controller/api/v2beta1"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/dependency"
	"github.com/fluxcd/pkg/runtime/transform"
	fluxsourcesv1beta1 "github.com/fluxcd/source-controller/api/v1beta1"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func addHelmRepo(ctx *Context, helmRepo *fluxsourcesv1beta1.HelmRepository) {
	ctx.V(1).Infof("Adding Helm repository: %s", helmRepo.Name)
	ctx.SetData(metasToRef(metav1.TypeMeta{Kind: helmRepo.Kind}, helmRepo.ObjectMeta), helmRepo)
}

func handleHelmRelease(ctx *Context, helmRelease *fluxhelmv2beta1.HelmRelease) {
	if len(helmRelease.Spec.DependsOn) == 0 {
		checkHelmRelease(ctx, helmRelease)
	} else {
		ctx.HelmReleaseQueue[helmRelease.Name] = HelmReleaseFix{helmRelease}
	}
}

func checkQueuedHelmReleases(ctx *Context) {
	helmReleaseList := make([]dependency.Dependent, 0, len(ctx.HelmReleaseQueue))
	for _, helmRelease := range ctx.HelmReleaseQueue {
		helmReleaseList = append(helmReleaseList, helmRelease)
	}
	sorted, err := dependency.Sort(helmReleaseList)
	if err != nil {
		ctx.Error(err, "")
		return
	}
	for _, ref := range sorted {
		ctx.StartOperation("Queued HelmRelease " + ref.Name)
		checkHelmRelease(ctx, ctx.HelmReleaseQueue[ref.Name].HelmRelease)
		ctx.EndOperation(true)
	}
}

func checkHelmRelease(ctx *Context, helmRelease *fluxhelmv2beta1.HelmRelease) {
	ctx.V(1).Infof("Validating Helm release: %q", helmRelease.Name)
	rendered, err := renderHelmRelease(ctx, helmRelease)
	if err != nil {
		ctx.Error(err, "")
		return
	}
	validateResource(ctx, rendered)
}

// HelmReleaseFix wraps HelmRelease to fix a bug in its implementation of dependency.Dependent.
// Can be removed when fixed upstream.
type HelmReleaseFix struct {
	*fluxhelmv2beta1.HelmRelease
}

func (in HelmReleaseFix) GetDependsOn() (types.NamespacedName, []dependency.CrossNamespaceDependencyReference) {
	return types.NamespacedName{
		Namespace: in.Namespace,
		Name:      in.Name,
	}, in.Spec.DependsOn
}

func renderHelmRelease(ctx *Context, helmRelease *fluxhelmv2beta1.HelmRelease) ([]byte, error) {
	actionConfig := new(action.Configuration)
	err := actionConfig.Init(genericclioptions.NewConfigFlags(false), "", "memory", func(string, ...interface{}) {})
	if err != nil {
		return nil, err
	}
	client := action.NewInstall(actionConfig)
	client.DryRun = true
	client.ReleaseName = helmRelease.Spec.ReleaseName
	if client.ReleaseName == "" {
		client.ReleaseName = helmRelease.Name
	}
	client.Replace = true // Skip the name check
	client.ClientOnly = true
	client.IncludeCRDs = true
	client.Namespace = helmRelease.Namespace
	if helmRelease.Spec.TargetNamespace != "" {
		client.Namespace = helmRelease.Spec.TargetNamespace
	}

	chart, err := getChart(ctx, client, helmRelease)
	if err != nil {
		return nil, err
	}

	chartValues, err := getChartValues(ctx, helmRelease)
	if err != nil {
		return nil, err
	}

	release, err := client.Run(chart, chartValues)
	if err != nil {
		return nil, err
	}
	return []byte(release.Manifest), nil
}

func getChart(ctx *Context, client *action.Install, helmRelease *fluxhelmv2beta1.HelmRelease) (*chart.Chart, error) {
	chart := helmRelease.Spec.Chart.Spec.Chart
	switch helmRelease.Spec.Chart.Spec.SourceRef.Kind {
	case "HelmRepository":
		ref := meta.NamespacedObjectKindReference(helmRelease.Spec.Chart.Spec.SourceRef)
		helmRepo, ok := ctx.GetData(ref).(*fluxsourcesv1beta1.HelmRepository)
		if !ok {
			return nil, fmt.Errorf("HelmRepository %q not found", helmRelease.Spec.Chart.Spec.SourceRef.Name)
		}
		client.RepoURL = helmRepo.Spec.URL
		cliConfig := cli.New()
		chartPath, err := client.LocateChart(chart, cliConfig)
		if err != nil {
			return nil, err
		}
		return loader.Load(chartPath)
	case "GitRepository":
		if helmRelease.Spec.Chart.Spec.SourceRef.Name != "management" {
			return nil, fmt.Errorf("Can't handle GitRepository customizations pointing to other repos yet")
		}
		return loader.Load(filepath.Join(ctx.TempDir, chart))
	default:
		return nil, fmt.Errorf("Helm source %q not implemented in test yet.", helmRelease.Spec.Chart.Spec.SourceRef.Kind)
	}
}

func getChartValues(ctx *Context, helmRelease *fluxhelmv2beta1.HelmRelease) (map[string]interface{}, error) {
	chartValues := map[string]interface{}{}
	for _, valuesFrom := range helmRelease.Spec.ValuesFrom {
		switch valuesFrom.Kind {
		case "ConfigMap":
			ref := meta.NamespacedObjectKindReference{
				APIVersion: "v1",
				Kind:       valuesFrom.Kind,
				Name:       valuesFrom.Name,
				Namespace:  helmRelease.Namespace,
			}
			configMap, ok := ctx.GetData(ref).(*v1.ConfigMap)
			if !ok {
				if valuesFrom.Optional {
					continue
				}
				return nil, fmt.Errorf("Referenced ConfigMap %q not found", valuesFrom.Name)
			}
			valueString, ok := configMap.Data[valuesFrom.GetValuesKey()]
			if !ok {
				if valuesFrom.Optional {
					continue
				}
				return nil, fmt.Errorf("Referenced ConfigMap key %q not found in %q", valuesFrom.GetValuesKey(), valuesFrom.Name)
			}
			if valuesFrom.TargetPath != "" {
				return nil, fmt.Errorf("targetPath in valuesFrom not supported in tests, needs to be extended")
			}
			values, err := chartutil.ReadValues([]byte(valueString))
			if err != nil {
				return nil, fmt.Errorf("Unable to deserialize chart values: %w", err)
			}
			chartValues = transform.MergeMaps(chartValues, values)
		default:
			return nil, fmt.Errorf("valuesFrom with type %q not yet supported in tests, needs to be extended", valuesFrom.Kind)
		}
	}
	chartValues = transform.MergeMaps(chartValues, helmRelease.GetValues())
	return chartValues, nil
}
