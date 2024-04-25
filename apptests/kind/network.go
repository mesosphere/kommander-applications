package kind

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/mesosphere/kommander-applications/apptests/docker"
)

var ErrMisconfiguredNetwork = errors.New("misconfigured kind network")

// GetDockerNetworkName returns docker network name for kind cluster.
func GetDockerNetworkName() string {
	// default network name
	kindNetwork := "kind"

	// env var for override network name
	if network, ok := os.LookupEnv("KIND_EXPERIMENTAL_DOCKER_NETWORK"); ok {
		kindNetwork = network
	}

	return kindNetwork
}

// EnsureDockerNetworkExist ensures docker network exist with given configurations.
func EnsureDockerNetworkExist(ctx context.Context, subnet string, internal bool) (*docker.NetworkResource, error) {
	dapi, err := docker.NewAPI()
	if err != nil {
		return nil, fmt.Errorf("could not create docker api:%w", err)
	}

	name := GetDockerNetworkName()

	ok, networkResource, err := dapi.GetNetwork(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get docker network %s: %w", name, err)
	}

	if ok && networkResource.Internal != internal {
		return nil, fmt.Errorf("internal flag does not match for network %s: %w", name, ErrMisconfiguredNetwork)
	}

	if ok && subnet != "" {
		ipamConfigs := networkResource.IPAM.Config

		if len(ipamConfigs) == 0 {
			return nil, fmt.Errorf("subnet configuration is missing for network %s: %w", name, ErrMisconfiguredNetwork)
		}

		// we take only the first
		actual := ipamConfigs[0].Subnet

		if actual != subnet {
			return nil, fmt.Errorf("subnet expected %s actual %s for network %s: %w", actual, subnet, name, ErrMisconfiguredNetwork)
		}
	}

	if !ok {
		networkResource, err = dapi.CreateNetwork(context.Background(), name, internal, subnet)

		if err != nil {
			return nil, fmt.Errorf("failed to create network %s: %w", name, err)
		}
	}

	return networkResource, nil
}

// EnsureNetworkIsDeleted ensures that the specified docker network either does not exist or is deleted.
func EnsureNetworkIsDeleted(ctx context.Context, name string) error {
	dapi, err := docker.NewAPI()
	if err != nil {
		return fmt.Errorf("could not create docker api:%w", err)
	}

	ok, networkResource, err := dapi.GetNetwork(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get docker network %s: %w", name, err)
	}

	if ok {
		err = dapi.DeleteNetwork(context.Background(), networkResource)

		if err != nil {
			return fmt.Errorf("failed to delete network %s: %w", name, err)
		}
	}

	return nil
}

// kindExperimentalNetworkLock prevents from running `CreateClusterInNetwork` in
// parallel as the function is setting a global environment variable
// `KIND_EXPERIMENTAL_DOCKER_NETWORK` that is valid for creating single cluster.
var kindExperimentalNetworkLock sync.Mutex //nolint:gochecknoglobals // prevents parallel execution of WithKindExperimentalDockerNetwork

// WithKindExperimentalDockerNetwork executes provided function with the environment
// variable `KIND_EXPERIMENTAL_DOCKER_NETWORK` set to provided network name. It
// also ensures that only 1 function is executed so it can be safely used in
// parallel code.
func WithKindExperimentalDockerNetwork(networkName string, run func() error) error {
	kindExperimentalNetworkLock.Lock()
	defer kindExperimentalNetworkLock.Unlock()

	defer func(revertTo string) {
		if err := os.Setenv("KIND_EXPERIMENTAL_DOCKER_NETWORK", revertTo); err != nil {
			log.Printf("Failed to revert back docker KIND_EXPERIMENTAL_DOCKER_NETWORK env variable: %s", err)
		}
	}(os.Getenv("KIND_EXPERIMENTAL_DOCKER_NETWORK"))

	if err := os.Setenv("KIND_EXPERIMENTAL_DOCKER_NETWORK", networkName); err != nil {
		return fmt.Errorf("failed create cluster with network by setting KIND_EXPERIMENTAL_DOCKER_NETWORK=%q: %w", networkName, err)
	}

	return run()
}
