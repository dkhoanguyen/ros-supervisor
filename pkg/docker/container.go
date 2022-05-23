package docker

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/dkhoanguyen/ros-supervisor/internal/utils"
	"github.com/docker/docker/api/types"
	dockerApiTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"go.uber.org/zap"
)

type Container struct {
	Name string `json:"name"`
	ID   string
}

type ContainerConfig struct {
	Name            string `json:"name"`
	Hostname        string `json:"hostname"`
	Domainname      string
	User            string
	AttachStdin     bool
	AttachStderr    bool
	AttachStdout    bool
	ExposedPort     map[string]struct{}
	Tty             bool
	OpenStdIn       bool
	StdinOnce       bool
	Environment     []string
	Cmd             ShellCommand
	Image           string
	WorkingDir      string
	EntryPoint      ShellCommand
	NetworkDisabled bool
	MacAddress      bool
	Labels          Labels
	HealthCheck     HeathCheckConfig
	StopSignal      string
	Volumes         map[string]struct{}
	StopTimeout     int
}

type HeathCheckConfig struct {
}

type HostConfig struct {
	AutoRemove      bool
	Binds           []string
	Devices         []container.DeviceMapping
	Resources       container.Resources
	VolumeDriver    string
	VolumesFrom     []string
	CapAdd          []string
	CapDrop         []string
	ExtraHosts      []string
	GroupAdd        []string
	IpcMode         string
	Cgroup          string
	Links           []string
	OomScoreAdj     int
	PidMode         string
	Privileged      bool
	PublishAllPorts bool
	ReadonlyRootfs  bool
	SecurityOpt     []string
	StorageOpt      map[string]string
	Tmpfs           map[string]string
	Isolation       string
	UTSMode         string
	UsernsMode      string
	ShmSize         int64
	Sysctls         map[string]string
	Runtime         string
	LogConfig       container.LogConfig
}

func MakeContainer(name string) Container {
	return Container{
		Name: name,
	}
}

func MakeContainersFromInfo(cntInfo []types.Container) []Container {
	output := make([]Container, 0)
	for _, cnt := range cntInfo {
		container := Container{
			ID:   cnt.ID,
			Name: cnt.Names[0][1:],
		}
		output = append(output, container)
	}
	return output
}

// ===== CREATE ====== //

func (cnt *Container) Create(
	ctx context.Context,
	dockerCli *client.Client,
	service *Service,
	network *Network,
	env string,
	logger *zap.Logger) error {

	allContainers, err := dockerCli.ContainerList(ctx, dockerApiTypes.ContainerListOptions{
		All: true,
	})

	if err != nil {
		logger.Error("Failed to list all containers")
	}
	for _, cont := range allContainers {
		for _, name := range cont.Names {
			if name == "/"+cnt.Name {
				err := dockerCli.ContainerRemove(ctx, cont.ID, dockerApiTypes.ContainerRemoveOptions{})
				if err != nil {
					logger.Error(fmt.Sprintf("Failed to remove designated container with error: %s", err))
					return err
				}
			}
		}
	}

	containerConfig, networkConfig, hostConfig := prepareContainerCreateOptions(service, network, env)
	container, err := dockerCli.ContainerCreate(ctx, &containerConfig,
		&hostConfig, &networkConfig, nil, cnt.Name)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to create container with error: %s", err))
		return err
	}
	cnt.ID = container.ID
	return nil
}

func prepareContainerCreateOptions(service *Service, network *Network, env string) (container.Config, network.NetworkingConfig, container.HostConfig) {
	containerConfig := prepareContainerConfig(service, env)
	networkConfig := prepareNetworkConfig(service, network, env)
	hostConfig := prepareHostConfig(service, env)
	return containerConfig, networkConfig, hostConfig
}

func prepareContainerConfig(service *Service, env string) container.Config {

	if env == "nightly" || env == "dev" || env == "uat" {
		ipIdx := -1
		rosMasterUriIdx := -1
		for idx, env_value := range service.Environment {
			if strings.Contains(env_value, "ROS_IP") {
				ipIdx = idx
			}
			if strings.Contains(env_value, "ROS_MASTER_URI") {
				rosMasterUriIdx = idx
			}
		}
		if rosMasterUriIdx != -1 {
			service.Environment = append(service.Environment[:rosMasterUriIdx], service.Environment[rosMasterUriIdx+1:]...)
		}
		if ipIdx != -1 {
			service.Environment = append(service.Environment[:ipIdx], service.Environment[ipIdx+1:]...)
		}
	}
	return container.Config{
		Hostname:   service.Hostname,
		Domainname: service.Domainname,
		User:       service.User,
		Tty:        service.Tty,
		Cmd:        strslice.StrSlice(service.Command),
		Entrypoint: strslice.StrSlice(service.EntryPoint),
		Image:      service.Image.Name,
		WorkingDir: service.WorkingDir,
		StopSignal: "SIGTERM",
		Env:        service.Environment,
	}
}

func prepareNetworkConfig(service *Service, targetNetwork *Network, env string) network.NetworkingConfig {
	if env == utils.DEVELOPMENT || env == utils.NIGHTLY || env == utils.UAT {
		// If the current working environment is dev-related
		// the we fuse the service network with host settings
		endPointConfig := map[string]*network.EndpointSettings{}
		return network.NetworkingConfig{
			EndpointsConfig: endPointConfig,
		}
	} else {
		// Inspect network first to get the ID

		// Get aliases
		aliases := []string{service.Name}

		endPointConfig := map[string]*network.EndpointSettings{
			targetNetwork.Name: {
				NetworkID: targetNetwork.ID,
				IPAddress: service.Networks[0].IPv4,
				Aliases:   aliases,
				IPAMConfig: &network.EndpointIPAMConfig{
					IPv4Address: service.Networks[0].IPv4,
				},
			},
		}
		return network.NetworkingConfig{
			EndpointsConfig: endPointConfig,
		}
	}
}

func prepareVolumeBinding(service *Service) []string {
	output := []string{}
	for _, volume := range service.Volumes {
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

func getRestartPolicy(service *Service) container.RestartPolicy {
	var restart container.RestartPolicy
	if service.Restart != "" {
		split := strings.Split(service.Restart, ":")
		var attemps int
		if len(split) > 1 {
			attemps, _ = strconv.Atoi(split[1])
		}
		restart.Name = split[0]
		restart.MaximumRetryCount = attemps
	}
	return restart

}

func getPortBinding(service *Service) nat.PortMap {
	bindingMap := nat.PortMap{}
	for _, port := range service.Ports {
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

func getResouces(service *Service) container.Resources {
	serviceResources := service.Resources
	deviceMappingList := []container.DeviceMapping{}
	for _, device := range service.Devices {
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
		CgroupParent:   service.CgroupParent,
		OomKillDisable: &serviceResources.OomKillDisable,
		Devices:        deviceMappingList,
		CPUPeriod:      serviceResources.CPUPeriod,
		CPUQuota:       serviceResources.CPUQuota,
		CpusetCpus:     serviceResources.CpusetCpus,
		Memory:         serviceResources.MemoryLimit,
	}

	return resources
}

func prepareHostConfig(service *Service, env string) container.HostConfig {
	// Prepare binding
	extraHost := make([]string, 0)
	if env == utils.PRODUCTION {
		extraHost = service.ExtraHosts
	}

	return container.HostConfig{
		AutoRemove:    false,
		Binds:         prepareVolumeBinding(service),
		CapAdd:        service.CapAdd,
		CapDrop:       service.CapDrop,
		ExtraHosts:    extraHost,
		NetworkMode:   container.NetworkMode("host"),
		RestartPolicy: getRestartPolicy(service),
		LogConfig: container.LogConfig{
			Type: "json-file",
		},
		IpcMode:      container.IpcMode(service.IpcMode),
		PortBindings: getPortBinding(service),
		Resources:    getResouces(service),
		Sysctls:      service.Sysctls,
		Privileged:   service.Privileged,
	}
}

// ===== START ===== //

func (cnt *Container) Start(ctx context.Context, dockerCli *client.Client, logger *zap.Logger) error {
	Id := cnt.ID
	if err := dockerCli.ContainerStart(ctx, Id, types.ContainerStartOptions{}); err != nil {
		logger.Error(fmt.Sprintf("Unable to start container %s with error: %s", cnt.Name, err))
		return err
	}
	return nil
}

// ====== STOP ====== //

func (cnt *Container) Stop(ctx context.Context, dockerCli *client.Client, logger *zap.Logger) error {
	logger.Info(fmt.Sprintf("Stopping container %s", cnt.Name))
	Id := cnt.ID
	if err := dockerCli.ContainerStop(ctx, Id, nil); err != nil {
		logger.Error(fmt.Sprintf("Unable to stop container %s: %v", Id, err))
		return err
	}
	return nil
}

// ====== REMOVE ====== //
func (cnt *Container) Remove(ctx context.Context, dockerCli *client.Client, logger *zap.Logger) error {
	logger.Info(fmt.Sprintf("Removing container %s", cnt.Name))
	Id := cnt.ID
	if err := dockerCli.ContainerRemove(ctx, Id, types.ContainerRemoveOptions{}); err != nil {
		logger.Error(fmt.Sprintf("Unable to remove container with error:%s", err))
		return err
	}
	return nil
}

// ======= LIST ======= //
func ListAllContainers(ctx context.Context, dockerCli *client.Client, logger *zap.Logger) ([]types.Container, error) {
	containers, err := dockerCli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		logger.Error(fmt.Sprintf("Unable to list all containers with error : %s", err))
	}
	return containers, err
}

// ======= INSPECT ======= //
func (cnt *Container) Inspect(
	ctx context.Context,
	dockerCli *client.Client,
	logger *zap.Logger,
) (types.ContainerJSON, error) {
	info, err := dockerCli.ContainerInspect(ctx, cnt.ID)
	if err != nil {
		logger.Error(fmt.Sprintf("Unable to inspect container with error : %s", err))
	}

	return info, err
}
