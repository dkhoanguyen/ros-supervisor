package compose

import (
	"context"

	"github.com/dkhoanguyen/ros-supervisor/models/compose"
	"github.com/dkhoanguyen/ros-supervisor/models/docker"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

func StartAllServiceContainer(ctx context.Context, dockerClient *client.Client, project *compose.Project) {
	for idx := range project.Services {
		StartSingleServiceContainer(ctx, dockerClient, &project.Services[idx])
	}
}

func StartSingleServiceContainer(ctx context.Context, dockerClient *client.Client, targetService *docker.Service) {
	containerID := targetService.Container.ID
	if err := dockerClient.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}
}
