package compose

import (
	"context"
	"fmt"

	"github.com/dkhoanguyen/ros-supervisor/pkg/docker"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
)

func StartAll(ctx context.Context, dockerClient *client.Client, project *Project, logger *zap.Logger) error {

	err := StartCore(ctx, dockerClient, project, logger)
	if err != nil {
		return err
	}

	err = StartServices(ctx, dockerClient, project, logger)
	if err != nil {
		return err
	}
	return nil
}

func StartCore(ctx context.Context, dockerClient *client.Client, project *Project, logger *zap.Logger) error {
	logger.Info("Starting core container")
	err := StartSingleServiceContainer(ctx, dockerClient, &project.Core, logger)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Failed to start core container with error: %s", err))
		return err
	}
	return nil
}

func StartServices(ctx context.Context, dockerClient *client.Client, project *Project, logger *zap.Logger) error {
	logger.Info("Starting all service containers")
	for idx := range project.Services {
		err := StartSingleServiceContainer(ctx, dockerClient, &project.Services[idx], logger)
		if err != nil {
			logger.Fatal(fmt.Sprintf("Failed to start service %s container with error: %s", project.Services[idx].Name, err))
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
