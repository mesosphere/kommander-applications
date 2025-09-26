package applyoci

import (
    "bytes"
    "context"
    "errors"
    "fmt"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"

    "github.com/spf13/cobra"

    appsv1 "k8s.io/api/apps/v1"
    corev1 "k8s.io/api/core/v1"
    networkingv1 "k8s.io/api/networking/v1"
    rbacv1 "k8s.io/api/rbac/v1"
    apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
    "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
    apiruntime "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/cli-runtime/pkg/genericclioptions"

    "github.com/fluxcd/cli-utils/pkg/kstatus/polling"
    helmv2b2 "github.com/fluxcd/helm-controller/api/v2beta2"
    kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
    runclient "github.com/fluxcd/pkg/runtime/client"
    "github.com/fluxcd/pkg/ssa"
    "github.com/fluxcd/pkg/ssa/normalize"
    ssautils "github.com/fluxcd/pkg/ssa/utils"
    sourcev1 "github.com/fluxcd/source-controller/api/v1"
    sourcev1b2 "github.com/fluxcd/source-controller/api/v1beta2"

    "sigs.k8s.io/controller-runtime/pkg/client"
)

var Cmd *cobra.Command //nolint:gochecknoglobals // Cobra commands are global.

const (
    refFlagName       = "ref"
    namespaceFlagName = "namespace"
    timeoutFlagName   = "timeout"
)

func init() { //nolint:gochecknoinits // Initializing cobra application.
    Cmd = &cobra.Command{
        Use:   "apply-oci",
        Short: "Pull a Helm OCI artifact and apply its rendered manifests",
        Args:  cobra.MaximumNArgs(0),
        RunE:  run,
    }

    Cmd.Flags().String(refFlagName, "ghcr.io/fluxcd-community/charts/flux2:latest", "OCI reference to pull (e.g., ghcr.io/org/charts/name:tag)")
    Cmd.Flags().String(namespaceFlagName, "kommander-flux", "Namespace to render/apply into")
    Cmd.Flags().Duration(timeoutFlagName, 5*time.Minute, "Timeout for waiting on applied resources")
}

func run(cmd *cobra.Command, _ []string) error {
    ctx := cmd.Context()
    ociRef := Cmd.Flag(refFlagName).Value.String()
    renderNS := Cmd.Flag(namespaceFlagName).Value.String()
    waitTimeout, err := time.ParseDuration(Cmd.Flag(timeoutFlagName).Value.String())
    if err != nil {
        return err
    }

    tmpDir, err := os.MkdirTemp("", "kommander-oci-*")
    if err != nil {
        return err
    }
    defer os.RemoveAll(tmpDir)

    // Pull chart using ORAS (preferred). Fallback to crane if available.
    if err := orasPull(ctx, ociRef, tmpDir); err != nil {
        // Best-effort fallback to crane
        if craneErr := craneExport(ctx, ociRef, tmpDir); craneErr != nil {
            return fmt.Errorf("failed to pull OCI artifact with oras (%v) and crane (%v)", err, craneErr)
        }
    }

    chartTGZ, err := findChartArchive(tmpDir)
    if err != nil {
        return err
    }

    extractedDir := filepath.Join(tmpDir, "extracted")
    if err := os.MkdirAll(extractedDir, 0o755); err != nil {
        return err
    }

    if err := untar(chartTGZ, extractedDir); err != nil {
        return err
    }

    chartDir, err := firstSubdir(extractedDir)
    if err != nil {
        return err
    }

    renderedPath := filepath.Join(tmpDir, "manifests.yaml")
    if err := helmTemplate(ctx, chartDir, renderNS, renderedPath); err != nil {
        return err
    }

    // Apply rendered manifests via SSA with a split between CRDs/Namespaces and the rest.
    kubeFlags := genericclioptions.NewConfigFlags(true)
    clientOpts := &runclient.Options{}
    if err := applyWithSSA(ctx, kubeFlags, clientOpts, renderedPath, waitTimeout); err != nil {
        return err
    }

    _, _ = fmt.Fprintln(cmd.OutOrStdout(), "Applied manifests from OCI artifact:", ociRef)
    return nil
}

func orasPull(ctx context.Context, ref, outDir string) error {
    if _, err := exec.LookPath("oras"); err != nil {
        return err
    }
    // oras pull supports oci:// prefix
    args := []string{"pull", "--output", outDir, fmt.Sprintf("oci://%s", strings.TrimPrefix(ref, "oci://"))}
    c := exec.CommandContext(ctx, "oras", args...) //nolint:gosec // controlled args
    c.Stdout = io.Discard
    c.Stderr = io.Discard
    return c.Run()
}

func craneExport(ctx context.Context, ref, outDir string) error {
    if _, err := exec.LookPath("crane"); err != nil {
        return err
    }
    // Export all layers as a tar stream, then extract
    tarPath := filepath.Join(outDir, "artifact.tar")
    f, err := os.Create(tarPath)
    if err != nil {
        return err
    }
    _ = f.Close()

    c := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf("crane export %s - > %s", shellEscape(ref), shellEscape(tarPath))) //nolint:gosec // shell used for pipe
    c.Stdout = io.Discard
    c.Stderr = io.Discard
    if err := c.Run(); err != nil {
        return err
    }
    return untar(tarPath, outDir)
}

func shellEscape(s string) string {
    return strings.ReplaceAll(s, "'", "'\\''")
}

func findChartArchive(dir string) (string, error) {
    // Common ORAS layout for Helm chart artifacts: file named "chart" or a single *.tgz
    // Prefer file named chart
    chartPath := filepath.Join(dir, "chart")
    if st, err := os.Stat(chartPath); err == nil && !st.IsDir() {
        // Rename to .tgz to make extraction simpler
        dest := chartPath + ".tgz"
        if err := os.Rename(chartPath, dest); err != nil {
            return "", err
        }
        return dest, nil
    }

    matches, err := filepath.Glob(filepath.Join(dir, "*.tgz"))
    if err != nil {
        return "", err
    }
    if len(matches) == 0 {
        return "", errors.New("no Helm chart archive found in pulled artifact")
    }
    return matches[0], nil
}

func untar(archivePath, destDir string) error {
    if _, err := exec.LookPath("tar"); err != nil {
        return err
    }
    c := exec.Command("tar", "-xzf", archivePath, "-C", destDir) //nolint:gosec // expected args
    c.Stdout = io.Discard
    c.Stderr = io.Discard
    return c.Run()
}

func firstSubdir(dir string) (string, error) {
    entries, err := os.ReadDir(dir)
    if err != nil {
        return "", err
    }
    for _, e := range entries {
        if e.IsDir() {
            return filepath.Join(dir, e.Name()), nil
        }
    }
    return "", fmt.Errorf("no directory found in %s", dir)
}

func helmTemplate(ctx context.Context, chartDir, namespace, outPath string) error {
    if _, err := exec.LookPath("helm"); err != nil {
        return err
    }
    // helm template <release> <chartDir> --namespace <ns>
    args := []string{"template", filepath.Base(chartDir), chartDir, "--namespace", namespace}
    c := exec.CommandContext(ctx, "helm", args...) //nolint:gosec // controlled args
    var stdout bytes.Buffer
    c.Stdout = &stdout
    c.Stderr = io.Discard
    if err := c.Run(); err != nil {
        return err
    }
    return os.WriteFile(outPath, stdout.Bytes(), 0o644)
}

func applyWithSSA(ctx context.Context, rcg genericclioptions.RESTClientGetter, opts *runclient.Options, manifestPath string, wait time.Duration) error {
    file, err := os.Open(manifestPath)
    if err != nil {
        return err
    }
    defer file.Close()

    objs, err := ssautils.ReadObjects(file)
    if err != nil {
        return err
    }
    if len(objs) == 0 {
        return fmt.Errorf("no Kubernetes objects found in: %s", manifestPath)
    }
    if err := normalize.UnstructuredList(objs); err != nil {
        return err
    }

    var stageOne []*unstructured.Unstructured
    var stageTwo []*unstructured.Unstructured
    for _, u := range objs {
        if ssautils.IsClusterDefinition(u) {
            stageOne = append(stageOne, u)
        } else {
            stageTwo = append(stageTwo, u)
        }
    }

    if len(stageOne) > 0 {
        if _, err := applySet(ctx, rcg, opts, stageOne); err != nil {
            return err
        }
        if err := waitForSet(rcg, opts, stageOne, wait); err != nil {
            return err
        }
    }

    if len(stageTwo) > 0 {
        if _, err := applySet(ctx, rcg, opts, stageTwo); err != nil {
            return err
        }
    }
    return nil
}

func newManager(rcg genericclioptions.RESTClientGetter, opts *runclient.Options) (*ssa.ResourceManager, error) {
    cfg, err := rcg.ToRESTConfig()
    if err != nil {
        return nil, fmt.Errorf("kubernetes configuration load failed: %w", err)
    }
    cfg.QPS = opts.QPS
    cfg.Burst = opts.Burst

    restMapper, err := rcg.ToRESTMapper()
    if err != nil {
        return nil, err
    }
    kubeClient, err := client.New(cfg, client.Options{Mapper: restMapper, Scheme: newScheme()})
    if err != nil {
        return nil, err
    }
    kubePoller := polling.NewStatusPoller(kubeClient, restMapper, polling.Options{})
    return ssa.NewResourceManager(kubeClient, kubePoller, ssa.Owner{Field: "flux", Group: "fluxcd.io"}), nil
}

func applySet(ctx context.Context, rcg genericclioptions.RESTClientGetter, opts *runclient.Options, objects []*unstructured.Unstructured) (*ssa.ChangeSet, error) {
    man, err := newManager(rcg, opts)
    if err != nil {
        return nil, err
    }
    return man.ApplyAll(ctx, objects, ssa.DefaultApplyOptions())
}

func waitForSet(rcg genericclioptions.RESTClientGetter, opts *runclient.Options, objects []*unstructured.Unstructured, timeout time.Duration) error {
    man, err := newManager(rcg, opts)
    if err != nil {
        return err
    }
    return man.WaitForSet(ssautils.ObjectsToObjMetadataSet(objects), ssa.WaitOptions{Interval: 2 * time.Second, Timeout: timeout})
}

func newScheme() *apiruntime.Scheme {
    scheme := apiruntime.NewScheme()
    _ = apiextensionsv1.AddToScheme(scheme)
    _ = corev1.AddToScheme(scheme)
    _ = rbacv1.AddToScheme(scheme)
    _ = appsv1.AddToScheme(scheme)
    _ = networkingv1.AddToScheme(scheme)
    _ = sourcev1b2.AddToScheme(scheme)
    _ = sourcev1.AddToScheme(scheme)
    _ = kustomizev1.AddToScheme(scheme)
    _ = helmv2b2.AddToScheme(scheme)
    return scheme
}

