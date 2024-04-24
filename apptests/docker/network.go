package docker

import (
	"context"
	"fmt"

	"inet.af/netaddr"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	dockernetwork "github.com/docker/docker/api/types/network"

	"github.com/mesosphere/kommander-applications/apptests/net"
)

// NetworkResource is a simple type wrapper for docker api NetworkResource.
type NetworkResource types.NetworkResource

type NetworkAPI interface {
	// GetNetwork returns the information for a specific network configured in the docker host.
	GetNetwork(ctx context.Context, name string) (bool, *NetworkResource, error)

	// CreateNetwork creates a new network in the docker host.
	CreateNetwork(ctx context.Context, name string, internal bool, subnet string) (*NetworkResource, error)

	// DeleteNetwork removes an existent network from the docker host.
	DeleteNetwork(ctx context.Context, network *NetworkResource) error

	// ConnectToNetwork connects a container to a network.
	ConnectToNetwork(ctx context.Context, networkID, containerID string) error
}

func (d *docker) GetNetwork(ctx context.Context, name string) (bool, *NetworkResource, error) {
	results, err := d.NetworkList(ctx, types.NetworkListOptions{Filters: filters.NewArgs(filters.Arg("name", name))})
	if err != nil {
		return false, nil, fmt.Errorf("couldn't list networks: %w", err)
	}

	if len(results) == 0 {
		return false, nil, nil
	}

	// (aweris) getting results[0] is fine, we can ignore duplicate network name possibility
	return true, (*NetworkResource)(&results[0]), nil
}

func (d *docker) CreateNetwork(ctx context.Context, name string, internal bool, subnet string) (*NetworkResource, error) {
	// (aweris) In internal networks we're using manually assigned container ip addresses to communicate and this is only
	// possible when subnet is manually configured in network creation
	if internal && subnet == "" {
		return nil, fmt.Errorf("%w: subnet is required for internal networks", ErrMissingParameter)
	}

	config := types.NetworkCreate{
		CheckDuplicate: true,
		Driver:         "bridge",
		Internal:       internal,
	}

	if len(subnet) > 0 {
		prefix, err := netaddr.ParseIPPrefix(subnet)
		if err != nil {
			return nil, fmt.Errorf("invalid subnet: %w", err)
		}

		config.IPAM = &dockernetwork.IPAM{
			Config: []dockernetwork.IPAMConfig{
				{
					Subnet: subnet,
				},
			},
		}

		// Create gateway address in internal network so that we can set internal
		// DNS resolver to a predicable IP address. By default the gateway is set
		// to a first IP address from the created subnet.
		// E.g. for 172.24.0.0/16 the gateway IP is 172.24.0.1
		if internal {
			// Create gateway IP address as first IP in the network range
			gatewayIP := prefix.Range().From().Next().String()
			config.IPAM.Config[0].Gateway = gatewayIP
		}
	}

	resp, err := d.NetworkCreate(ctx, name, config)
	if err != nil {
		return nil, err
	}

	resource, err := d.NetworkInspect(ctx, resp.ID, types.NetworkInspectOptions{})
	if err != nil {
		return nil, err
	}

	return (*NetworkResource)(&resource), nil
}

// ConnectToNetwork connects a container to a network. no config is provided.
func (d *docker) ConnectToNetwork(ctx context.Context, networkID, containerID string) error {
	return d.NetworkConnect(ctx, networkID, containerID, nil)
}

func (d *docker) DeleteNetwork(ctx context.Context, network *NetworkResource) error {
	return d.NetworkRemove(ctx, network.ID)
}

func (n *NetworkResource) Subnet() (*net.Subnet, error) {
	ipamConfigs := n.IPAM.Config

	if len(ipamConfigs) == 0 {
		return nil, ErrMissingSubnetConfig
	}

	return net.ParseSubnet(ipamConfigs[0].Subnet)
}
