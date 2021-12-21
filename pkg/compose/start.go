package compose

import (
	"context"
	"fmt"

	"github.com/dkhoanguyen/ros-supervisor/pkg/docker"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
)

func StartAllServiceContainer(ctx context.Context, dockerClient *client.Client, project *Project, logger *zap.Logger) error {
	for idx := range project.Services {
		err := StartSingleServiceContainer(ctx, dockerClient, &project.Services[idx], logger)
		if err != nil {
			return err
		}
	}
	return nil
}

func StartSingleServiceContainer(ctx context.Context, dockerClient *client.Client, targetService *docker.Service, logger *zap.Logger) error {
	containerID := targetService.Container.ID
	if err := dockerClient.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		logger.Error(fmt.Sprintf("Unable to start container %s with error: %s", targetService.Container.Name, err))
		return err
	}
	return nil
}
