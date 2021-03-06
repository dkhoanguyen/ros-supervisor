package compose

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/dkhoanguyen/ros-supervisor/pkg/docker"
	moby "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"go.uber.org/zap"
)

// Container stuff
func CreateContainers(ctx context.Context, dockerClient *client.Client, project *Project, logger *zap.Logger) error {

	err := CreateCoreContainer(ctx, dockerClient, project, logger)
	if err != nil {
		return err
	}

	logger.Info("Creating container for services")
	for idx := range project.Services {
		_, err := CreateSingleContainer(ctx, project.Name, &project.Services[idx], &project.Networks[0], dockerClient, logger)
		if err != nil {
			logger.Fatal(fmt.Sprintf("Unable to create container for service %s with error: %s", project.Services[idx].Name, err))
			return err
		}
	}
	return nil
}

func CreateCoreContainer(ctx context.Context, dockerClient *client.Client, project *Project, logger *zap.Logger) error {

	err := CreateNetwork(ctx, project, dockerClient, true, logger)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Unable to create network %s with error: %s", project.Networks[0].Name, err))
		return err
	}

	logger.Info("Creating container for core")
	_, err = CreateSingleContainer(ctx, project.Name, &project.Core, &project.Networks[0], dockerClient, logger)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Unable to create container for core with error: %s", err))
		return err
	}
	return nil
}

func CreateServiceContainers(ctx context.Context, dockerClient *client.Client, project *Project, logger *zap.Logger) error {

	err := CreateNetwork(ctx, project, dockerClient, false, logger)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Unable to create network %s with error: %s", project.Networks[0].Name, err))
		return err
	}

	logger.Info("Creating container for services")
	for idx := range project.Services {
		_, err := CreateSingleContainer(ctx, project.Name, &project.Services[idx], &project.Networks[0], dockerClient, logger)
		if err != nil {
			logger.Fatal(fmt.Sprintf("Unable to create container for service %s with error: %s", project.Services[idx].Name, err))
			return err
		}
	}
	return nil
}

func CreateSingleContainer(ctx context.Context, projectName string, targetService *docker.Service, targetNetwork *docker.Network, dockerClient *client.Client, logger *zap.Logger) (string, error) {

	containerName := projectName + "_" + targetService.Name
	allContainers, err := dockerClient.ContainerList(ctx, moby.ContainerListOptions{
		All: true,
	})
	if err != nil {
		logger.Error("Failed to list all containers")
	}
	for _, cont := range allContainers {
		for _, name := range cont.Names {
			if name == "/"+containerName {
				err := dockerClient.ContainerRemove(ctx, cont.ID, moby.ContainerRemoveOptions{})
				if err != nil {
					logger.Error(fmt.Sprintf("Failed to remove designated container with error: %s", err))
					return "", err
				}
			}
		}
	}
	containerConfig, networkConfig, hostConfig := PrepareContainerCreateOptions(targetService, targetNetwork)
	container, err := dockerClient.ContainerCreate(ctx, &containerConfig, &hostConfig, &networkConfig, nil, containerName)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to create container with error: %s", err))
		return "", err
	}
	targetService.Container = docker.Container{
		ID:   container.ID,
		Name: containerName,
	}

	return container.ID, nil
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
			NetworkID: targetNetwork.ID,
			IPAddress: targetService.Networks[0].IPv4,
			Aliases:   aliases,
			IPAMConfig: &network.EndpointIPAMConfig{
				IPv4Address: targetService.Networks[0].IPv4,
			},
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

func CreateNetwork(ctx context.Context, project *Project, dockerClient *client.Client, forceRecreate bool, logger *zap.Logger) error {
	for idx, network := range project.Networks {
		networkOpts := PrepareNetworkOptions(project.Name, &network)
		networkName := network.Name
		info, err := dockerClient.NetworkInspect(ctx, networkName, moby.NetworkInspectOptions{})
		// Only create network if it does not exist
		if err != nil {
			resp, err := dockerClient.NetworkCreate(ctx, networkName, networkOpts)
			if err != nil {
				// Error means there could be a network existed already
				logger.Error(fmt.Sprintf("Unable to create network %s with error: %v", networkName, err))
				return err
			}
			network.ID = resp.ID
		} else {
			if forceRecreate {
				// Delete old network setup and recreate them
				// Note that all containers that use the target network must be killed
				err := dockerClient.NetworkRemove(ctx, info.ID)
				if err != nil {
					logger.Error(fmt.Sprintf("Unable to remove network %s with error: %v", networkName, err))
					return err
				}
				logger.Info(fmt.Sprintf("Recreating network %s", networkName))
				resp, err := dockerClient.NetworkCreate(ctx, networkName, networkOpts)
				if err != nil {
					logger.Error(fmt.Sprintf("Unable to create network %s with error: %v", networkName, err))
					return err
				}
				project.Networks[idx].ID = resp.ID
			} else {
				// Maybe extract existing values
				networkRes, err := dockerClient.NetworkList(ctx, moby.NetworkListOptions{})
				if err != nil {
					logger.Error(fmt.Sprintf("Unable to list network %s with error: %v", networkName, err))
					return err
				}

				for _, net := range networkRes {
					if net.Name == networkName {
						logger.Info("Extracting Network Info")
						project.Networks[idx].ID = net.ID
						return nil
					}
				}
			}
		}
	}
	return nil
}

// Volume stuff
func CreateVolume(ctx context.Context) {

}
