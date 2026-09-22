package docker

import (
	"errors"

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
