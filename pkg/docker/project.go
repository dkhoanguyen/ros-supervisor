package docker

import (
	"context"
	"fmt"
	"io/ioutil"
	"sort"
	"strings"

	"github.com/docker/docker/client"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type DockerProject struct {
	Name        string `json:"name"`
	WorkingDir  string `json:"working_dir"`
	Core        Service
	Services    Services `json:"services"`
	Networks    Networks `json:"networks"`
	Volumes     Volumes  `json:"volumes"`
	Configs     Configs  `json:"configs"`
	ComposeFile []byte   `json:"compose_file"`
}

func (dp *DockerProject) ServiceNames() []string {
	var names []string
	for _, service := range dp.Services {
		names = append(names, service.Name)
	}
	return names
}

func (dp *DockerProject) NetworkNames() []string {
	var names []string
	for _, network := range dp.Networks {
		names = append(names, network.Name)
	}
	return names
}

func (dp *DockerProject) GetService(name string) Service {
	for _, service := range dp.Services {
		if service.Name == name {
			return service
		}
	}
	return Service{}
}

// Restructure services based on dependencies
func (dp *DockerProject) RestructureServices(logger *zap.Logger) {
	logger.Info("Organising services based on dependencies hierarchy")
	restructureServices := Services{}
	numDepends := make([]int, 0)
	numDepends = append(numDepends, 0)
	find := func(element int, arr []int) bool {
		for _, d := range arr {
			if element == d {
				return true
			}
		}
		return false
	}
	for _, service := range dp.Services {
		if !find(len(service.DependsOn), numDepends) {
			numDepends = append(numDepends, len(service.DependsOn))
		}
	}

	sort.Ints(numDepends)

	for _, nd := range numDepends {
		for _, service := range dp.Services {
			if len(service.DependsOn) == nd {
				restructureServices = append(restructureServices, service)
			}
		}
	}

	dp.Services = restructureServices
}

func (dp *DockerProject) DisplayProject() {
	for _, service := range dp.Services {
		fmt.Printf("Service Name: %s\n", service.Name)
		fmt.Printf("Build Context: %s\n", service.BuildOpt.Context)
		fmt.Printf("Build Dockerfile: %s\n", service.BuildOpt.Dockerfile)
		fmt.Printf("Build ContainerName: %s\n", service.ContainerName)
		fmt.Printf("Depends On: %s\n", service.DependsOn)
		fmt.Printf("Networks Name: %s\n", service.Networks[0].Name)
		fmt.Printf("IPV4: %s\n", service.Networks[0].IPv4)
		for _, env := range service.Environment {
			fmt.Printf("Env: %s\n", env)
		}
		fmt.Printf("Image ID: %s\n", service.Image.ID)
		fmt.Printf("Container ID: %s\n", service.Container.ID)
		fmt.Printf("=====\n")
	}

	for _, networks := range dp.Networks {
		fmt.Printf("Network Name: %s\n", networks.Name)
		fmt.Printf("Network Driver: %s\n", networks.Driver)
		fmt.Printf("Network IPAM Subnet: %s\n", networks.Ipam.Config[0].Subnet)
		fmt.Printf("Network IPAM Gateway: %s\n", networks.Ipam.Config[0].Gateway)
		fmt.Printf("=====\n")
	}

	for _, volume := range dp.Volumes {
		fmt.Printf("Volume Name: %s\n", volume.Name)
	}
}

// Factory Methods
func MakeDockerProject(composePath, projectPath string, logger *zap.Logger) *DockerProject {
	dp := DockerProject{}

	composeFile, err := ioutil.ReadFile(composePath)
	if err != nil {
		logger.Fatal("Unable to read docker-compose file.")
	}
	rawData := make(map[interface{}]interface{})
	err2 := yaml.Unmarshal(composeFile, &rawData)
	if err2 != nil {
		logger.Fatal("Unable to extract docker-compose file.")
	}

	slicedProjectPath := strings.Split(projectPath, "/")

	dp.Name = slicedProjectPath[len(slicedProjectPath)-2]
	dp.WorkingDir = projectPath
	dp.ComposeFile = composeFile

	// Make Services, Volumes, and Networks
	// Extracting Core and Services
	logger.Debug("Extracting all services")

	rawServiceData := rawData["services"].(map[string]interface{})
	for name, config := range rawServiceData {
		if name == "core" {
			logger.Info("Extracting core separately")
			dp.Core = MakeService(config.(map[string]interface{}),
				name, projectPath, logger)
			continue
		}
		dService := MakeService(config.(map[string]interface{}),
			name, projectPath, logger)
		dp.Services = append(dp.Services, dService)
	}

	// Extract Network
	dp.Networks = MakeNetwork(rawData, logger)

	// Extract Volumes
	dp.Volumes = MakeVolume(rawData, logger)

	// Reorganise services based on dependencies
	dp.RestructureServices(logger)

	return &dp
}

// Build project
func (p *DockerProject) BuildProjectImages(
	ctx context.Context,
	dockerCli *client.Client,
	excludeCore bool,
	logger *zap.Logger,
) error {

	// Build Core
	if !excludeCore {
		name := p.Name + "_" + p.Core.Name
		img := MakeImage(name, "latest")
		err := img.Build(ctx, dockerCli, &p.Core, logger)
		if err != nil {
			// TODO: Resolve error here
			return err
		}
		p.Core.Image = img
	}

	// Build other services
	for _, srv := range p.Services {
		name := p.Name + "_" + srv.Name
		img := MakeImage(name, "latest")
		err := img.Build(ctx, dockerCli, &srv, logger)
		if err != nil {
			// TODO: Resolve error here
			return err
		}
		srv.Image = img
	}
	return nil
}

func (p *DockerProject) CreateProjectContainers(
	ctx context.Context,
	dockerCli *client.Client,
	excludeCore bool,
	logger *zap.Logger,
) error {

	// Create Core
	if !excludeCore {
		name := p.Name + "_" + p.Core.Name
		cnt := MakeContainer(name)
		net := p.Networks[0]
		err := cnt.Create(ctx, dockerCli, &p.Core, &net, logger)
		if err != nil {
			// TODO: Resolve error here
			return err
		}
		p.Core.Container = cnt
	}

	for _, srv := range p.Services {
		name := p.Name + "_" + srv.Name
		cnt := MakeContainer(name)
		// TODO: Extract network from all networks
		net := p.Networks[0]
		err := cnt.Create(ctx, dockerCli, &srv, &net, logger)
		if err != nil {
			// TODO: Resolve error here
			return err
		}

		srv.Container = cnt
	}
	return nil
}

func (p *DockerProject) StartProjectContainers(
	ctx context.Context,
	dockerCli *client.Client,
	excludeCore bool,
	logger *zap.Logger) error {

	if !excludeCore {
		err := p.Core.Container.Start(ctx, dockerCli, logger)
		if err != nil {
			// TODO: Resolve error here
			return err
		}
	}

	for _, srv := range p.Services {
		// Start Container
		err := srv.Container.Start(ctx, dockerCli, logger)
		if err != nil {
			// TODO: Resolve error here
			return err
		}
	}
	return nil
}
