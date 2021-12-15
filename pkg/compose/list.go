package compose

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

func ListAllContainers(ctx context.Context, dockerClient *client.Client) []types.Container {
	containers, err := dockerClient.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		panic(err)
	}
	return containers
}

func ListAllImages(ctx context.Context, dockerClient *client.Client) []types.ImageSummary {
	images, err := dockerClient.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		panic(err)
	}
	return images
}
