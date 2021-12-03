package compose

import (
	"context"

	"github.com/dkhoanguyen/ros-supervisor/models/compose"
	"github.com/dkhoanguyen/ros-supervisor/models/docker"
	moby "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

func CreateContainer(ctx context.Context) {

}

func PrepareNetworkOptions(projectName string, targetNetwork *docker.Network) moby.NetworkCreate {
	return moby.NetworkCreate{
		CheckDuplicate: targetNetwork.CheckDuplicate,
		Labels:         targetNetwork.Labels,
		Driver:         targetNetwork.Driver,
		Internal:       targetNetwork.Internal,
		Attachable:     targetNetwork.Attachable,
		IPAM:           &targetNetwork.Ipam,
		EnableIPv6:     targetNetwork.EnableIPv6,
	}
}

func CreateNetwork(ctx context.Context, project *compose.Project, dockerClient *client.Client) {
	for _, network := range project.Networks {
		networkOpts := PrepareNetworkOptions(project.Name, &network)
		networkName := project.Name + "_" + network.Name
		dockerClient.NetworkCreate(ctx, networkName, networkOpts)
	}

}

func CreateVolume(ctx context.Context) {

}
