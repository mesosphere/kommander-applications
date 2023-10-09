package flux

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fluxcd/flux2/v2/pkg/manifestgen"
	"github.com/fluxcd/flux2/v2/pkg/manifestgen/install"
	runclient "github.com/fluxcd/pkg/runtime/client"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type Options struct {
	KubeconfigArgs    *genericclioptions.ConfigFlags
	KubeclientOptions *runclient.Options
	Namespace         string
	Components        []string
}

// Install installs flux components in the given namespace on the cluster using the given kubeconfig and client options.
func Install(ctx context.Context, opts Options) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// make default options for installing flux components
	options := install.MakeDefaultOptions()
	options.Namespace = opts.Namespace
	options.Components = opts.Components

	// generate flux manifest
	manifest, err := install.Generate(options, "")
	if err != nil {
		return err
	}

	// create a temporary directory for the manifest
	tmpDir, err := manifestgen.MkdirTempAbs("", opts.Namespace)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	// write the manifest to the temporary directory
	if _, err := manifest.WriteFile(tmpDir); err != nil {
		return fmt.Errorf("install failed: %w", err)
	}

	_, err = Apply(
		ctx,
		opts.KubeconfigArgs,
		opts.KubeclientOptions,
		tmpDir,
		filepath.Join(tmpDir, manifest.Path))

	return nil
}
