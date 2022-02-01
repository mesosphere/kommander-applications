package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"

	fluxhelmv2beta1 "github.com/fluxcd/helm-controller/api/v2beta1"
	fluxkustomizev1beta2 "github.com/fluxcd/kustomize-controller/api/v1beta2"
	fluxsourcesv1beta1 "github.com/fluxcd/source-controller/api/v1beta1"
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/mesosphere/dkp-cli-runtime/core/cmd/root"
	"github.com/spf13/cobra"
)

var errValidationFailed = fmt.Errorf("FAIL")

func main() {
	cmd, opts := root.NewCommand(os.Stdout, os.Stderr)
	cmd.SilenceErrors = true
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := NewContext(opts.Output)

		// hide internal helm logging
		log.SetOutput(io.Discard)

		_, thisFile, _, _ := goruntime.Caller(0)
		rootPath, _ := filepath.Abs(filepath.Join(filepath.Dir(thisFile), "..", ".."))

		// need to create additional files in-tree, so copy everything into temporary directory
		var err error
		tempDir, err := os.MkdirTemp("", "validate-manifests")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tempDir)
		err = exec.Command("cp", "-r", filepath.Join(rootPath, "common"), tempDir).Run()
		if err != nil {
			return err
		}
		err = exec.Command("cp", "-r", filepath.Join(rootPath, "services"), tempDir).Run()
		if err != nil {
			return err
		}
		ctx.TempDir = tempDir

		ctx.StartOperation("loading additional CRDs")
		loadAdditionalCRDs(ctx)
		ctx.EndOperation(true)
		if ctx.Failed {
			return errValidationFailed
		}
		if ctx.Config.EnableLegacyCertmanagerGroup {
			ctx.CRDSchemas["certmanager.k8s.io/v1alpha1/Certificate"] = ctx.CRDSchemas["cert-manager.io/v1/Certificate"]
			ctx.CRDSchemas["certmanager.k8s.io/v1alpha1/Issuer"] = ctx.CRDSchemas["cert-manager.io/v1/Issuer"]
		}

		ctx.StartOperation("checking Helm repositories")
		checkKustomization(ctx, filepath.Join(tempDir, "common", "base"))
		ctx.EndOperation(true)
		checkServices(ctx, filepath.Join(tempDir, "services"))
		checkFluxKustomizations(ctx)
		checkQueuedHelmReleases(ctx)

		if ctx.AnyFailed {
			return errValidationFailed
		} else {
			ctx.Info("PASS")
			return nil
		}
	}

	err := cmd.Execute()
	if err != nil {
		opts.Output.Error(err, "")
		os.Exit(1)
	}
}

func checkServices(ctx *Context, servicesPath string) {
	serviceDirs, err := os.ReadDir(servicesPath)
	if err != nil {
		ctx.Error(err, "")
		return
	}
	for _, serviceDir := range serviceDirs {
		if serviceDir.IsDir() {
			if ctx.Config.SkipApplications[serviceDir.Name()] {
				continue
			}
			servicePath := filepath.Join(servicesPath, serviceDir.Name())
			checkService(ctx, servicePath)
		}
	}
}

func checkService(ctx *Context, servicePath string) {
	versionDirs, err := os.ReadDir(servicePath)
	if err != nil {
		ctx.Error(err, "")
		return
	}

	// TODO(fr): check metadata.yaml

	for _, versionDir := range versionDirs {
		if versionDir.IsDir() {
			serviceVersionPath := filepath.Join(servicePath, versionDir.Name())

			testName, _ := filepath.Rel(ctx.TempDir, serviceVersionPath)
			ctx.StartOperation(testName)
			{
				defaultsDir := filepath.Join(serviceVersionPath, "defaults/")
				if _, err := os.Stat(defaultsDir); err == nil {
					checkKustomization(ctx, defaultsDir)
				}
				checkKustomization(ctx, serviceVersionPath)
			}
			ctx.EndOperation(true)
		}
	}
}

func validateResource(ctx *Context, resourceYaml []byte) {
	r := yaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(resourceYaml)))
	for {
		doc, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			ctx.Error(err, "")
			return
		}

		obj, _, err := ctx.Decoder.Decode(doc, nil, nil)
		if err != nil {
			if runtime.IsNotRegisteredError(err) {
				validateResourceAgainstCRDs(ctx, doc)
				continue
			}
			// not a manifest, e.g. comments before yaml separator
			if runtime.IsMissingKind(err) {
				continue
			}
			ctx.Error(err, "")
			ctx.Error(nil, string(doc))
			continue
		}

		if x, ok := obj.(metav1.Object); ok {
			ctx.V(2).Infof("Validating resource %q (%s)", x.GetName(), obj.GetObjectKind().GroupVersionKind())
		} else {
			ctx.V(2).Infof("Validating resource of type %q", obj.GetObjectKind().GroupVersionKind())
		}

		switch obj := obj.(type) {
		case *v1.ConfigMap:
			ctx.ConfigMaps[obj.Name] = obj.Data
		case *apiextensionsv1.CustomResourceDefinition:
			addCRDv1(ctx, obj)
		case *apiextensionsv1beta1.CustomResourceDefinition:
			addCRDv1beta1(ctx, obj)
		case *fluxsourcesv1beta1.HelmRepository:
			addHelmRepo(ctx, obj)
		case *fluxhelmv2beta1.HelmRelease:
			handleHelmRelease(ctx, obj)
		case *fluxkustomizev1beta2.Kustomization:
			handleFluxCustomization(ctx, obj)
		}
	}
}
