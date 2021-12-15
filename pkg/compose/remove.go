package compose

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

func RemoveServices(ctx context.Context, dockerClient *client.Client, project *Project) {
	for idx := range project.Services {
		containerID := project.Services[idx].Container.ID
		RemoveService(ctx, dockerClient, containerID)
	}
}

func RemoveService(ctx context.Context, dockerClient *client.Client, containerID string) {
	dockerClient.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{})
}
