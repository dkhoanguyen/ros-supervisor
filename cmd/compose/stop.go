package compose

import (
	"context"

	"github.com/dkhoanguyen/ros-supervisor/models/compose"
	"github.com/dkhoanguyen/ros-supervisor/models/docker"
	"github.com/docker/docker/client"
)

func StopServices(ctx context.Context, dockerClient *client.Client, project *compose.Project) {
	for idx := range project.Services {
		StopService(ctx, dockerClient, &project.Services[idx])
	}
}

func StopService(ctx context.Context, dockerClient *client.Client, targetService *docker.Service) {
	containerID := targetService.Container.ID
	dockerClient.ContainerStop(ctx, containerID, nil)
}

func StopServiceByID(ctx context.Context, dockerClient *client.Client, containerID string) {
	dockerClient.ContainerStop(ctx, containerID, nil)
}
