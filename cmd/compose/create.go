package compose

import (
	"context"
	"strconv"
	"strings"

	"github.com/dkhoanguyen/ros-supervisor/models/compose"
	"github.com/dkhoanguyen/ros-supervisor/models/docker"
	moby "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// Container stuff
func CreateSingleContainer(ctx context.Context, targetService *docker.Service, dockerClient *client.Client) {

}

func PrepareContainerCreateOptions(targetService *docker.Service, targetNetwork *docker.Network) (container.Config, network.NetworkingConfig, container.HostConfig) {
	containerConfig := PrepareContainerConfig(targetService)
	networkConfig := PrepareNetworkConfig(targetService, targetNetwork)
	hostConfig := PrepareHostConfig(targetService)

	return containerConfig, networkConfig, hostConfig
}

func PrepareContainerConfig(targetService *docker.Service) container.Config {
	return container.Config{
		Hostname:   targetService.Hostname,
		Domainname: targetService.Domainname,
		User:       targetService.User,
		Tty:        targetService.Tty,
		Cmd:        strslice.StrSlice(targetService.Command),
		Entrypoint: strslice.StrSlice(targetService.EntryPoint),
		Image:      targetService.Image.Name,
		WorkingDir: targetService.WorkingDir,
		StopSignal: "SIGTERM",
		Env:        targetService.Environment,
	}
}

func PrepareNetworkConfig(targetService *docker.Service, targetNetwork *docker.Network) network.NetworkingConfig {
	// Inspect network first to get the ID

	// Get aliases
	aliases := []string{targetService.Name}

	endPointConfig := map[string]*network.EndpointSettings{
		targetNetwork.Name: {
			IPAddress: targetService.Networks[0].IPv4,
			Aliases:   aliases,
		},
	}
	return network.NetworkingConfig{
		EndpointsConfig: endPointConfig,
	}
}

func prepareVolumeBinding(targetService *docker.Service) []string {
	output := []string{}
	for _, volume := range targetService.Volumes {
		if len(volume.Source) > 0 && len(volume.Destination) > 0 {
			bindMount := volume.Source + ":" + volume.Destination

			if len(volume.Option) > 0 {
				bindMount = bindMount + ":" + volume.Option
			}

			output = append(output, bindMount)
		}
	}
	return output
}

func getRestartPolicy(targetService *docker.Service) container.RestartPolicy {
	var restart container.RestartPolicy
	if targetService.Restart != "" {
		split := strings.Split(targetService.Restart, ":")
		var attemps int
		if len(split) > 1 {
			attemps, _ = strconv.Atoi(split[1])
		}
		restart.Name = split[0]
		restart.MaximumRetryCount = attemps
	}
	return restart

}

func getPortBinding(targetService *docker.Service) nat.PortMap {
	bindingMap := nat.PortMap{}
	for _, port := range targetService.Ports {
		p := nat.Port(port.Target + "/" + port.Protocol)
		bind := bindingMap[p]
		binding := nat.PortBinding{
			HostIP:   port.HostIp,
			HostPort: port.HostPort,
		}
		bind = append(bind, binding)
		bindingMap[p] = bind
	}
	return bindingMap
}

func getResouces(targetService *docker.Service) container.Resources {
	deviceMappingList := []container.DeviceMapping{}
	for _, device := range targetService.Devices {
		deviceSplit := strings.Split(device, ":")
		deviceMapping := container.DeviceMapping{
			CgroupPermissions: "rwm",
		}
		switch len(deviceSplit) {
		case 3:
			deviceMapping.CgroupPermissions = deviceSplit[2]
			fallthrough
		case 2:
			deviceMapping.PathInContainer = deviceSplit[1]
			fallthrough
		case 1:
			deviceMapping.PathInContainer = deviceSplit[0]
		}
		deviceMappingList = append(deviceMappingList, deviceMapping)
	}

	resources := container.Resources{
		CgroupParent:   targetService.CgroupParent,
		Memory:         targetService.MemLimit,
		OomKillDisable: &targetService.OomKillDisable,
		Devices:        deviceMappingList,
	}

	return resources
}

func PrepareHostConfig(targetService *docker.Service) container.HostConfig {
	// Prepare binding
	return container.HostConfig{
		AutoRemove:    false,
		Binds:         prepareVolumeBinding(targetService),
		CapAdd:        targetService.CapAdd,
		CapDrop:       targetService.CapDrop,
		NetworkMode:   container.NetworkMode(targetService.NetworkMode),
		RestartPolicy: getRestartPolicy(targetService),
		LogConfig: container.LogConfig{
			Type: "json-file",
		},
		IpcMode:      container.IpcMode(targetService.IpcMode),
		PortBindings: getPortBinding(targetService),
		Resources:    getResouces(targetService),
		Sysctls:      targetService.Sysctls,
		Privileged:   targetService.Privileged,
	}
}

// Network stuff
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

func CreateNetwork(ctx context.Context, project *compose.Project, dockerClient *client.Client, forceRecreate bool) {
	for _, network := range project.Networks {
		networkOpts := PrepareNetworkOptions(project.Name, &network)
		networkName := project.Name + "_" + network.Name
		_, err := dockerClient.NetworkInspect(ctx, networkName, moby.NetworkInspectOptions{})
		// Only create network if it does not exist
		if err != nil {
			dockerClient.NetworkCreate(ctx, networkName, networkOpts)
		} else {
			if forceRecreate {
				// Delete old network setup and recreate them
				// Note that all containers that use the target network must be killed
			}
		}
	}
}

// Volume stuff
func CreateVolume(ctx context.Context) {

}
