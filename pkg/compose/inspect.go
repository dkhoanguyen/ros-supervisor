package compose

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
)

func InspectNetwork(ctx context.Context, networkName string, dockerClient *client.Client, logger *zap.Logger) (types.NetworkResource, error) {
	info, err := dockerClient.NetworkInspect(ctx, networkName, types.NetworkInspectOptions{})
	if err != nil {
		logger.Error(fmt.Sprintf("Unable to inspect network with error : %s", err))
	}
	return info, err
}

func InspectContainer(ctx context.Context, containerID string, dockerClient *client.Client, logger *zap.Logger) (types.ContainerJSON, error) {
	info, err := dockerClient.ContainerInspect(ctx, containerID)
	if err != nil {
		logger.Error(fmt.Sprintf("Unable to inspect container with error : %s", err))
	}

	return info, err
}
