package appscenarios

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	fluxhelmv2 "github.com/fluxcd/helm-controller/api/v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mesosphere/kommander-applications/apptests/constants"
	"github.com/mesosphere/kommander-applications/apptests/environment"
	"github.com/mesosphere/kommander-applications/apptests/flux"
	"github.com/mesosphere/kommander-applications/apptests/scenarios"
)

type knative struct {
	appPathCurrentVersion  string
	appPathPreviousVersion string
}

func (k knative) Name() string {
	return constants.Knative
}

var _ scenarios.AppScenario = (*knative)(nil)

func NewKnative() *knative {
	appPath, _ := absolutePathTo(constants.Knative)
	appPrevVerPath, _ := getkAppsUpgradePath(constants.Knative)
	return &knative{
		appPathCurrentVersion:  appPath,
		appPathPreviousVersion: appPrevVerPath,
	}
}

func (k knative) Install(ctx context.Context, env *environment.Env) error {
	err := k.install(ctx, env, k.appPathCurrentVersion)
	return err
}

func (k knative) InstallPreviousVersion(ctx context.Context, env *environment.Env) error {
	// Simulate the v2.17 helm-controller (Helm v3 SDK, client-side apply)
	// before installing the previous-version chart, but only when the
	// previous-version chart predates the v2.18 cm.yaml fix. See
	// setHelmControllerHelm3Defaults for full rationale.
	needsHelm3, err := k.previousVersionRequiresHelm3Defaults()
	if err != nil {
		return err
	}
	if needsHelm3 {
		if err := setHelmControllerHelm3Defaults(ctx, env, true); err != nil {
			return fmt.Errorf("enable %s feature gate: %w", helm3DefaultsGate, err)
		}
	}
	return k.install(ctx, env, k.appPathPreviousVersion)
}

func (k knative) Upgrade(ctx context.Context, env *environment.Env) error {
	// Restore the v1.5.x helm-controller defaults (Helm v4 SDK, server-side
	// apply) before applying the current-version chart, mirroring the toggle
	// we made in InstallPreviousVersion. No-op when we never enabled the
	// gate (i.e. previous version already had the v2.18+ cm.yaml shape).
	needsHelm3, err := k.previousVersionRequiresHelm3Defaults()
	if err != nil {
		return err
	}
	if needsHelm3 {
		if err := setHelmControllerHelm3Defaults(ctx, env, false); err != nil {
			return fmt.Errorf("disable %s feature gate: %w", helm3DefaultsGate, err)
		}
	}
	return k.install(ctx, env, k.appPathCurrentVersion)
}

// previousVersionRequiresHelm3Defaults reports whether the previous-version
// knative chart predates the kommander-applications v2.18 cm.yaml fix that
// switched eventing.manifest.spec.deployments[].podTemplate.spec.affinity to
// the schema-aligned eventing.manifest.spec.deployments[].affinity.
//
// Knative <1.20.x was shipped by NKP <2.18 with the broken cm.yaml. Knative
// 1.20.x onwards (NKP 2.18+) ships the corrected shape that passes the
// helm-controller v1.5.x SSA path without any workaround.
//
// TODO(NKP-after-2.18-only): remove this helper, the
// setHelmControllerHelm3Defaults dance, and the unconditional call in
// InstallPreviousVersion / Upgrade once we no longer support upgrades from
// pre-2.18 NKP releases. The whole v3-defaults block becomes dead code at
// that point.
func (k knative) previousVersionRequiresHelm3Defaults() (bool, error) {
	prev, err := parseKnativeVersionFromPath(k.appPathPreviousVersion)
	if err != nil {
		return false, fmt.Errorf("parse previous Knative version: %w", err)
	}
	return prev.major == 1 && prev.minor < 20, nil
}

func (k knative) ValidateUpgradeVersionStep() error {
	previous, err := parseKnativeVersionFromPath(k.appPathPreviousVersion)
	if err != nil {
		return fmt.Errorf("parse previous Knative version: %w", err)
	}

	current, err := parseKnativeVersionFromPath(k.appPathCurrentVersion)
	if err != nil {
		return fmt.Errorf("parse current Knative version: %w", err)
	}

	if current.major != previous.major {
		return fmt.Errorf("unsupported Knative upgrade from %s to %s: major version changes are not allowed", previous, current)
	}
	if current.minor < previous.minor {
		return fmt.Errorf("unsupported Knative upgrade from %s to %s: downgrade is not allowed", previous, current)
	}
	if current.minor-previous.minor > 1 {
		return fmt.Errorf("unsupported Knative upgrade from %s to %s: Knative only supports one minor version upgrade at a time", previous, current)
	}

	return nil
}

type knativeVersion struct {
	major int
	minor int
	patch int
	raw   string
}

func (v knativeVersion) String() string {
	return v.raw
}

func parseKnativeVersionFromPath(path string) (knativeVersion, error) {
	version := strings.TrimPrefix(filepath.Base(filepath.Clean(path)), "v")
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return knativeVersion{}, fmt.Errorf("expected semantic version directory, got %q", filepath.Base(path))
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return knativeVersion{}, fmt.Errorf("parse major version %q: %w", parts[0], err)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return knativeVersion{}, fmt.Errorf("parse minor version %q: %w", parts[1], err)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return knativeVersion{}, fmt.Errorf("parse patch version %q: %w", parts[2], err)
	}

	return knativeVersion{
		major: major,
		minor: minor,
		patch: patch,
		raw:   version,
	}, nil
}

func (k knative) install(ctx context.Context, env *environment.Env, appPath string) error {
	// apply defaults config maps first
	defaultKustomization := filepath.Join(appPath, "/defaults")
	if _, err := os.Stat(defaultKustomization); err == nil {
		err := env.ApplyKustomizations(ctx, defaultKustomization, map[string]string{
			"appVersion":       "app-version-knative",
			"releaseNamespace": kommanderNamespace,
		})
		if err != nil {
			return err
		}
	}

	// Apply the helmrelease kustomizations
	err := env.ApplyKustomizations(ctx, appPath, map[string]string{
		"releaseName":      "knative",
		"appVersion":       "app-version-knative",
		"releaseNamespace": kommanderNamespace,
	})
	if err != nil {
		return err
	}

	return nil
}

// InstallIstioHelmDependency installs istio-helm which is required by Knative.
// It applies the helmrelease subdirectory directly rather than the top-level
// Flux Kustomization wrappers, which reference a GitRepository "management"
// that does not exist in test clusters.
func (k knative) InstallIstioHelmDependency(ctx context.Context, env *environment.Env) error {
	istioHelmPath, err := absolutePathTo("istio-helm")
	if err != nil {
		return fmt.Errorf("failed to get path for istio-helm: %w", err)
	}

	substMap := map[string]string{
		"releaseName":      "istio-helm",
		"appVersion":       "app-version-istio-helm",
		"releaseNamespace": kommanderNamespace,
		"caIssuerName":     "kommander-ca",
	}

	// Apply defaults for istio-helm
	defaultKustomization := filepath.Join(istioHelmPath, "/defaults")
	if _, err := os.Stat(defaultKustomization); err == nil {
		err := env.ApplyKustomizations(ctx, defaultKustomization, map[string]string{
			"appVersion":       "app-version-istio-helm",
			"releaseNamespace": kommanderNamespace,
		})
		if err != nil {
			return fmt.Errorf("failed to apply defaults for istio-helm: %w", err)
		}
	}

	// Apply pre-install resources directly
	preInstallPath := filepath.Join(istioHelmPath, "pre-install")
	if _, err := os.Stat(preInstallPath); err == nil {
		err := env.ApplyYAML(ctx, preInstallPath, substMap)
		if err != nil {
			return fmt.Errorf("failed to apply pre-install for istio-helm: %w", err)
		}
	}

	// Apply istio-helm-gateway-namespace directly
	gatewayNsPath := filepath.Join(istioHelmPath, "istio-helm-gateway-namespace")
	if _, err := os.Stat(gatewayNsPath); err == nil {
		err := env.ApplyYAML(ctx, gatewayNsPath, substMap)
		if err != nil {
			return fmt.Errorf("failed to apply gateway namespace for istio-helm: %w", err)
		}
	}

	// The istiod chart creates ServiceMonitor resources; install the CRD so
	// Helm can render the manifests in test clusters that lack kube-prometheus-stack.
	if err := installServiceMonitorCRD(ctx, env); err != nil {
		return fmt.Errorf("failed to install ServiceMonitor CRD: %w", err)
	}

	// Create config overrides to disable features that depend on components
	// not present in test clusters (cert-manager CA issuer, etc.).
	if err := createIstioHelmTestOverrides(ctx, env); err != nil {
		return fmt.Errorf("failed to create istio-helm test overrides: %w", err)
	}

	// Apply the helmrelease subdirectory directly (OCIRepositories, HelmReleases,
	// ConfigMap) instead of the top-level kustomization which creates Flux
	// Kustomization wrappers that require a GitRepository source.
	helmreleasePath := filepath.Join(istioHelmPath, "helmrelease")
	err = env.ApplyKustomizations(ctx, helmreleasePath, substMap)
	if err != nil {
		return fmt.Errorf("failed to apply istio-helm helmrelease: %w", err)
	}

	// The istio-helm-base HelmRelease depends on kube-prometheus-stack, which is
	// not installed in test clusters. Remove that dependency so the install can
	// proceed without it.
	if err := k.removeIstioHelmDependsOn(ctx, env); err != nil {
		return fmt.Errorf("failed to patch istio-helm dependencies: %w", err)
	}

	return nil
}

// createIstioHelmTestOverrides creates the optional istio-helm-config-overrides
// ConfigMap to disable features that require components not present in test
// clusters (cert-manager CA issuer for the cacert-job, etc.).
func createIstioHelmTestOverrides(ctx context.Context, env *environment.Env) error {
	client, err := ctrlClient.New(env.K8sClient.Config(), ctrlClient.Options{})
	if err != nil {
		return err
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "istio-helm-config-overrides",
			Namespace: kommanderNamespace,
		},
		Data: map[string]string{
			"values.yaml": "security:\n  enabled: false\n",
		},
	}

	err = client.Create(ctx, cm)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// installServiceMonitorCRD creates a minimal ServiceMonitor CRD so the istiod
// Helm chart can render without kube-prometheus-stack being installed.
func installServiceMonitorCRD(ctx context.Context, env *environment.Env) error {
	client, err := ctrlClient.New(env.K8sClient.Config(), ctrlClient.Options{})
	if err != nil {
		return err
	}

	preserveUnknown := true
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "servicemonitors.monitoring.coreos.com",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "monitoring.coreos.com",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:   "servicemonitors",
				Singular: "servicemonitor",
				Kind:     "ServiceMonitor",
				ListKind: "ServiceMonitorList",
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1",
					Served:  true,
					Storage: true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type:                   "object",
							XPreserveUnknownFields: &preserveUnknown,
						},
					},
				},
			},
		},
	}

	err = client.Create(ctx, crd)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// removeIstioHelmDependsOn patches the istio-helm-base HelmRelease to remove
// the dependsOn on kube-prometheus-stack, which is not present in test clusters.
func (k knative) removeIstioHelmDependsOn(ctx context.Context, env *environment.Env) error {
	genericClient, err := ctrlClient.New(env.K8sClient.Config(), ctrlClient.Options{
		Scheme: flux.NewScheme(),
	})
	if err != nil {
		return err
	}

	hr := &fluxhelmv2.HelmRelease{}
	err = genericClient.Get(ctx, ctrlClient.ObjectKey{
		Name:      "istio-helm-base",
		Namespace: kommanderNamespace,
	}, hr)
	if err != nil {
		return fmt.Errorf("could not get istio-helm-base HelmRelease: %w", err)
	}

	filtered := make([]fluxhelmv2.DependencyReference, 0, len(hr.Spec.DependsOn))
	for _, dep := range hr.Spec.DependsOn {
		if dep.Name != "kube-prometheus-stack" {
			filtered = append(filtered, dep)
		}
	}
	hr.Spec.DependsOn = filtered

	// Use MergePatch via Update to write back the cleared dependsOn
	hr.ManagedFields = nil
	hr.TypeMeta = metav1.TypeMeta{
		Kind:       fluxhelmv2.HelmReleaseKind,
		APIVersion: fluxhelmv2.GroupVersion.String(),
	}
	return genericClient.Update(ctx, hr)
}

// helm-controller deployment coordinates. helm-controller is shipped by
// kommander-flux and runs in this namespace with this deployment name.
const (
	helmControllerNamespace = "kommander-flux"
	helmControllerName      = "helm-controller"
	helm3DefaultsGate       = "UseHelm3Defaults"
)

// setHelmControllerHelm3Defaults toggles helm-controller's `UseHelm3Defaults`
// feature gate by patching the controller Deployment's --feature-gates arg and
// waiting for the rollout to complete.
//
// Why this exists: helm-controller v1.5.0 (Feb 2026) bumped to the Helm v4 SDK
// and flipped the default apply method to server-side apply. SSA validates
// patches via the structured-merge-diff typed parser against the target CRD's
// OpenAPI v3 schema. The 1.19.5 knative cm.yaml in the upgrade-from
// kommander-applications checkout (v2.17.0) wraps eventing-webhook affinity
// under a non-existent `podTemplate.spec.affinity` field. The 1.19.5
// KnativeEventing CRD does not declare `podTemplate`, so SSA fails install:
//
//	"failed to create typed patch object (...; KnativeEventing):
//	 .spec.deployments[0].podTemplate: field not declared in schema"
//
// To faithfully simulate a v2.17 -> v2.18 upgrade we install the previous
// version under the Helm v3 defaults (client-side apply, the behavior of
// helm-controller v1.4.x that shipped with v2.17), then restore the v1.5.x
// defaults before the upgrade so that the upgrade exercises the SSA path that
// v2.18 customers will use.
//
// This is gated by previousVersionRequiresHelm3Defaults so it only runs for
// pre-2.18 from-versions; 2.18+ ships the schema-aligned cm.yaml and works
// with the v1.5.x SSA default unmodified.
//
// Reference: https://github.com/fluxcd/helm-controller/blob/v1.5.0/CHANGELOG.md
func setHelmControllerHelm3Defaults(ctx context.Context, env *environment.Env, enable bool) error {
	c, err := ctrlClient.New(env.K8sClient.Config(), ctrlClient.Options{
		Scheme: flux.NewScheme(),
	})
	if err != nil {
		return fmt.Errorf("create client for helm-controller patch: %w", err)
	}

	key := ctrlClient.ObjectKey{Name: helmControllerName, Namespace: helmControllerNamespace}
	dep := &appsv1.Deployment{}
	if err := c.Get(ctx, key, dep); err != nil {
		return fmt.Errorf("get helm-controller deployment: %w", err)
	}

	if len(dep.Spec.Template.Spec.Containers) == 0 {
		return fmt.Errorf("helm-controller deployment has no containers")
	}
	container := &dep.Spec.Template.Spec.Containers[0]

	newArgs, changed := setFeatureGate(container.Args, helm3DefaultsGate, enable)
	if !changed {
		return nil
	}
	container.Args = newArgs

	if err := c.Update(ctx, dep); err != nil {
		return fmt.Errorf("update helm-controller deployment: %w", err)
	}

	return waitForDeploymentRollout(ctx, c, key, 3*time.Minute)
}

// setFeatureGate adds or removes a single feature gate from a Kubernetes-style
// `--feature-gates=foo=true,bar=false` arg list. Returns the new args slice and
// whether the slice differs from the input.
//
// Behavior:
//   - enable=true:  ensure `gate=true` is present in the --feature-gates arg
//     (adding the arg if missing).
//   - enable=false: ensure `gate` is not present in the --feature-gates arg
//     (and remove the arg entirely if it would otherwise be empty).
func setFeatureGate(args []string, gate string, enable bool) ([]string, bool) {
	const prefix = "--feature-gates="

	for i, arg := range args {
		if !strings.HasPrefix(arg, prefix) {
			continue
		}
		gates := parseFeatureGates(arg[len(prefix):])
		_, present := gates[gate]
		switch {
		case enable && present && gates[gate] == "true":
			return args, false
		case !enable && !present:
			return args, false
		}
		if enable {
			gates[gate] = "true"
		} else {
			delete(gates, gate)
		}
		out := make([]string, len(args))
		copy(out, args)
		if encoded := encodeFeatureGates(gates); encoded != "" {
			out[i] = prefix + encoded
		} else {
			out = append(out[:i], out[i+1:]...)
		}
		return out, true
	}

	if !enable {
		return args, false
	}
	return append(append([]string{}, args...), prefix+gate+"=true"), true
}

func parseFeatureGates(s string) map[string]string {
	gates := make(map[string]string)
	for _, part := range strings.Split(s, ",") {
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			gates[kv[0]] = kv[1]
		}
	}
	return gates
}

func encodeFeatureGates(gates map[string]string) string {
	if len(gates) == 0 {
		return ""
	}
	keys := make([]string, 0, len(gates))
	for k := range gates {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+gates[k])
	}
	return strings.Join(parts, ",")
}

// waitForDeploymentRollout blocks until the deployment's spec generation has
// been observed and all replicas are updated and available, or the timeout
// elapses.
func waitForDeploymentRollout(ctx context.Context, c ctrlClient.Client, key ctrlClient.ObjectKey, timeout time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		dep := &appsv1.Deployment{}
		if err := c.Get(waitCtx, key, dep); err != nil {
			return fmt.Errorf("get deployment %s/%s: %w", key.Namespace, key.Name, err)
		}
		desired := int32(1)
		if dep.Spec.Replicas != nil {
			desired = *dep.Spec.Replicas
		}
		if dep.Status.ObservedGeneration >= dep.Generation &&
			dep.Status.UpdatedReplicas == desired &&
			dep.Status.AvailableReplicas == desired &&
			dep.Status.Replicas == desired {
			return nil
		}
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("timed out waiting for deployment %s/%s rollout: %w",
				key.Namespace, key.Name, waitCtx.Err())
		case <-time.After(pollInterval):
		}
	}
}
