package compose

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"go.uber.org/zap"
)

func ListAllContainers(ctx context.Context, dockerClient *client.Client, logger *zap.Logger) ([]types.Container, error) {
	containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		logger.Error(fmt.Sprintf("Unable to list all containers with error : %s", err))
	}
	return containers, err
}

func ListAllImages(ctx context.Context, dockerClient *client.Client, logger *zap.Logger) ([]types.ImageSummary, error) {
	images, err := dockerClient.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		logger.Error(fmt.Sprintf("Unable to list all images with error : %s", err))
	}
	return images, err
}
