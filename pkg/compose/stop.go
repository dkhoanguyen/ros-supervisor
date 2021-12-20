package compose

import (
	"context"
	"fmt"

	"github.com/dkhoanguyen/ros-supervisor/pkg/docker"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
)

func StopServices(ctx context.Context, dockerClient *client.Client, project *Project) error {
	for idx := range project.Services {
		err := StopService(ctx, dockerClient, &project.Services[idx])
		if err != nil {
			return err
		}
	}
	return nil
}

func StopService(ctx context.Context, dockerClient *client.Client, targetService *docker.Service) error {
	containerID := targetService.Container.ID
	return dockerClient.ContainerStop(ctx, containerID, nil)
}

func StopServiceByID(ctx context.Context, dockerClient *client.Client, containerID string, logger *zap.Logger) error {
	err := dockerClient.ContainerStop(ctx, containerID, nil)
	if err != nil {
		logger.Error(fmt.Sprintf("Unable to stop container %s: %v", containerID, err))
	}
	return err
}
