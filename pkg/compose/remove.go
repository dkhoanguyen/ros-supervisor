package compose

import (
	"context"
	"fmt"

	"github.com/dkhoanguyen/ros-supervisor/pkg/docker"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
)

func RemoveServices(ctx context.Context, dockerClient *client.Client, project *Project, logger *zap.Logger) error {
	for idx := range project.Services {
		err := RemoveService(ctx, dockerClient, &project.Services[idx], logger)
		if err != nil {
			return err
		}
	}
	return nil
}

func RemoveService(ctx context.Context, dockerClient *client.Client, service *docker.Service, logger *zap.Logger) error {
	containerID := service.Container.ID
	return RemoveServiceByID(ctx, dockerClient, containerID, logger)
}

func RemoveServiceByID(ctx context.Context, dockerClient *client.Client, containerID string, logger *zap.Logger) error {
	err := dockerClient.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{})
	if err != nil {
		logger.Error(fmt.Sprintf("Unable to remove container with error:%s", err))
	}
	return err
}
