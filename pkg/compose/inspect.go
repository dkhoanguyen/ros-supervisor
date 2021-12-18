package compose

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

func InspectNetwork(ctx context.Context, networkName string, dockerClient *client.Client) types.NetworkResource {
	info, err := dockerClient.NetworkInspect(ctx, networkName, types.NetworkInspectOptions{})
	if err != nil {
		panic(err)
	}
	return info
}

func InspectContainer(ctx context.Context, containerID string, dockerClient *client.Client) types.ContainerJSON {
	info, err := dockerClient.ContainerInspect(ctx, containerID)
	if err != nil {
		panic(err)
	}

	return info
}
