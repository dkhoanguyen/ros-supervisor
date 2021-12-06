package compose

import (
	"context"

	moby "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

// func NetworkExisted(ctx context.Context, networkName string, dockerClient *client.Client) bool {
// 	_, err := dockerClient.NetworkInspect(ctx, networkName, moby.NetworkInspectOptions{})
// 	if err != nil {
// 		return false
// 	}
// 	return true
// }

func InspectNetwork(ctx context.Context, networkName string, dockerClient *client.Client) moby.NetworkResource {
	info, err := dockerClient.NetworkInspect(ctx, networkName, moby.NetworkInspectOptions{})
	if err != nil {
		panic(err)
	}
	return info
}
