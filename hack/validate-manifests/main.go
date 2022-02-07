package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"

	"github.com/mesosphere/dkp-cli-runtime/core/cmd/root"
	"github.com/mesosphere/dkp-cli-runtime/core/output"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {
	cmd, opts := root.NewCommand(os.Stdout, os.Stderr)
	cmd.SilenceErrors = true
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		_, thisFile, _, _ := goruntime.Caller(0)
		rootPath, _ := filepath.Abs(filepath.Join(filepath.Dir(thisFile), "..", ".."))
		return check(rootPath, opts.Output, DefaultConfig())
	}

	err := cmd.Execute()
	if err != nil {
		opts.Output.Error(err, "")
		os.Exit(1)
	}
}

var errValidationFailed = fmt.Errorf("FAIL")

func check(rootPath string, output output.Output, config Config) error {
	ctx := NewContext(output, config)

	// hide internal helm logging
	log.SetOutput(io.Discard)

	// need to create additional files in-tree, so copy everything into temporary directory
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
	ctx.RootDir = tempDir

	ctx.StartOperation("Loading additional CRDs")
	errs := loadAdditionalCRDs(ctx)
	ctx.EndOperation(len(errs) == 0)
	for _, err := range errs {
		ctx.Error(err, "")
	}
	if len(errs) > 0 {
		return errValidationFailed
	}

	if ctx.Config.EnableLegacyCertmanagerGroup {
		ctx.SetCRDSchema(
			metav1.TypeMeta{APIVersion: "certmanager.k8s.io/v1alpha1", Kind: "Certificate"},
			ctx.GetCRDSchema(metav1.TypeMeta{APIVersion: "cert-manager.io/v1", Kind: "Certificate"}),
		)
		ctx.SetCRDSchema(
			metav1.TypeMeta{APIVersion: "certmanager.k8s.io/v1alpha1", Kind: "Issuer"},
			ctx.GetCRDSchema(metav1.TypeMeta{APIVersion: "cert-manager.io/v1", Kind: "Issuer"}),
		)
	}

	ctx.StartOperation("Checking Helm repositories")
	errs = checkKustomization(ctx, filepath.Join(tempDir, "common", "base"))
	ctx.EndOperation(len(errs) == 0)
	for _, err := range errs {
		ctx.Error(err, "")
	}
	if len(errs) > 0 {
		return errValidationFailed
	}

	success := checkServices(ctx, filepath.Join(tempDir, "services"))

	ctx.StartOperation("Checking Flux resources")
	runnerSuccess := ctx.Runner.Run(ctx)
	ctx.EndOperation(runnerSuccess)
	if !success || !runnerSuccess {
		return errValidationFailed
	}

	ctx.Info("PASS")
	return nil
}

func checkServices(ctx *Context, servicesPath string) bool {
	success := true
	serviceDirs, err := os.ReadDir(servicesPath)
	if err != nil {
		ctx.Error(err, "")
		return false
	}
	for _, serviceDir := range serviceDirs {
		if serviceDir.IsDir() {
			if ctx.Config.SkipApplications[serviceDir.Name()] {
				continue
			}
			servicePath := filepath.Join(servicesPath, serviceDir.Name())
			if !checkService(ctx, servicePath) {
				success = false
			}
		}
	}
	return success
}

func checkService(ctx *Context, servicePath string) bool {
	success := true
	versionDirs, err := os.ReadDir(servicePath)
	if err != nil {
		ctx.Error(err, "")
		return false
	}

	// TODO(fr): check metadata.yaml

	for _, versionDir := range versionDirs {
		if versionDir.IsDir() {
			serviceVersionPath := filepath.Join(servicePath, versionDir.Name())

			testName, _ := filepath.Rel(ctx.RootDir, serviceVersionPath)
			ctx.StartOperation(testName)
			defaultsDir := filepath.Join(serviceVersionPath, "defaults/")
			if _, err := os.Stat(defaultsDir); err == nil {
				errs := checkKustomization(ctx, defaultsDir)
				if len(errs) > 0 {
					success = false
					ctx.EndOperation(false)
				}
				for _, err := range errs {
					ctx.Error(err, "")
				}
			}
			errs := checkKustomization(ctx, serviceVersionPath)
			ctx.EndOperation(len(errs) == 0)
			for _, err := range errs {
				success = false
				ctx.Error(err, "")
			}
		}
	}
	return success
}
