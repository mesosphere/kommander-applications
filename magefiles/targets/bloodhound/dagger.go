package bloodhound

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dagger.io/dagger"

	"github.com/mesosphere/daggers/dagger/options"
)

const (
	// url template for downloading bloodhound cli
	bloodhoundURLTemplate = "https://downloads.mesosphere.io/dkp-bloodhound/dkp-bloodhound_v%s_linux_amd64.tar.gz"

	// standard source path.
	srcDir = "/src"
)

// Run runs the ginkgo run command with given options.
func Run(ctx context.Context, client *dagger.Client, workdir *dagger.Directory, opts ...Option) (string, error) {
	cfg, err := loadConfigFromEnv()
	if err != nil {
		return "", err
	}

	for _, o := range opts {
		cfg = o(cfg)
	}

	container, err := getContainer(ctx, client, &cfg)
	if err != nil {
		return "", err
	}

	args := []string{"dkp-bloodhound"}

	if cfg.Verbose > 0 {
		args = append(args, fmt.Sprintf("--verbose=%d", cfg.Verbose))
	}

	args = append(args, cfg.Args...)

	container = container.
		WithMountedDirectory(srcDir, workdir).
		WithWorkdir(srcDir).
		WithEnvVariable("CACHE_BUSTER", time.Now().String()). // Workaround for stop caching after this step
		Exec(dagger.ContainerExecOpts{Args: args})

	output, err := container.Stdout().Contents(ctx)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(output), nil
}

// getContainer returns a dagger container instance with bloodhound cli as entrypoint.
func getContainer(ctx context.Context, client *dagger.Client, cfg *config) (*dagger.Container, error) {
	var err error

	// Source url for downloading the bloodhound cli.
	srcURL := fmt.Sprintf(bloodhoundURLTemplate, cfg.BloodHoundVersion)

	// Destination file to download tar file contains Github CLI
	dstFile := "/tmp/bloodhound_linux_amd64.tar.gz"

	var customizers []options.ContainerCustomizer

	customizers = append(customizers, options.DownloadFile(srcURL, dstFile))

	container := client.
		Container().From("debian:bullseye-slim").
		Exec(dagger.ContainerExecOpts{Args: []string{"sh", "-c", "apt-get update && apt-get install -y curl"}}).
		Exec(dagger.ContainerExecOpts{Args: []string{"sh", "-c", "rm -rf /var/lib/apt/lists/*"}})

	for _, customizer := range customizers {
		container, err = customizer(container, client)
		if err != nil {
			return nil, err
		}
	}

	container = container.
		Exec(dagger.ContainerExecOpts{Args: []string{"tar", "-xf", dstFile, "-C", "/usr/local/bin/"}}).
		Exec(dagger.ContainerExecOpts{Args: []string{"ls", "-la", "/usr/local/bin"}}).
		Exec(dagger.ContainerExecOpts{Args: []string{"chmod", "+x", "/usr/local/bin/dkp-bloodhound"}}).
		Exec(dagger.ContainerExecOpts{Args: []string{"rm", "-rf", "/tmp/*"}})

	_, err = container.ExitCode(ctx)
	if err != nil {
		return nil, err
	}

	return container, nil
}
