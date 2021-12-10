package compose

import (
	"context"

	"github.com/dkhoanguyen/ros-supervisor/models/compose"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

func RemoveServices(ctx context.Context, dockerClient *client.Client, project *compose.Project) {
	for idx := range project.Services {
		containerID := project.Services[idx].Container.ID
		RemoveService(ctx, dockerClient, containerID)
	}
}

func RemoveService(ctx context.Context, dockerClient *client.Client, containerID string) {
	dockerClient.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{})
}
