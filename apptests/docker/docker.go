package docker

import (
	"errors"
	"fmt"
	"io"

	"github.com/docker/cli/cli/config"
	clitypes "github.com/docker/cli/cli/config/types"
	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

var (
	_ API = &docker{}

	ErrMissingParameter    = errors.New("missing parameter")
	ErrMissingSubnetConfig = errors.New("missing subnet configuration")
)

type API interface {
	NetworkAPI
	//ImageAPI
	//ContainerAPI
	//ExecAPI
	//CopyAPI
}

type docker struct {
	*client.Client
}

func NewAPI() (API, error) {
	dc, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	return &docker{dc}, nil
}

func (d docker) credentialsForImage(image string) (types.AuthConfig, error) {
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return types.AuthConfig{}, fmt.Errorf("failed to parse image name: %w", err)
	}

	registry := reference.Domain(ref)

	configFile := config.LoadDefaultConfigFile(io.Discard)

	// Have to try https://index.docker.io/v1/ if docker.io is the registry due to the way Docker CLI
	// has special handling for docker.io compared to other registries.
	registryNamesToTry := []string{registry}
	if registry == "docker.io" {
		registryNamesToTry = append(registryNamesToTry, "https://index.docker.io/v1/")
	}

	for _, reg := range registryNamesToTry {
		authConfig, err := configFile.GetAuthConfig(reg)
		if err != nil {
			return types.AuthConfig{}, fmt.Errorf(
				"failed to get auth config for %s: %w",
				registry,
				err,
			)
		}

		// Ignore authConfig if it's empty, above call doesn't return an error for missing auth config.
		if (authConfig != clitypes.AuthConfig{}) {
			return types.AuthConfig{
				Auth:          authConfig.Auth,
				Username:      authConfig.Username,
				Password:      authConfig.Password,
				Email:         authConfig.Email,
				ServerAddress: authConfig.ServerAddress,
				IdentityToken: authConfig.IdentityToken,
				RegistryToken: authConfig.RegistryToken,
			}, nil
		}
	}

	return types.AuthConfig{}, nil
}
