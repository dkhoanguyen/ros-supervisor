package compose

import (
	"fmt"
	"io/ioutil"
	"sort"
	"strings"

	"github.com/dkhoanguyen/ros-supervisor/pkg/docker"
	"github.com/docker/docker/api/types/network"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"
)

type Project struct {
	Name        string `json:"name"`
	WorkingDir  string `json:"working_dir"`
	Core        docker.Service
	Services    docker.Services `json:"services"`
	Networks    docker.Networks `json:"networks"`
	Volumes     docker.Volumes  `json:"volumes"`
	Configs     docker.Configs  `json:"configs"`
	ComposeFile []byte          `json:"compose_file"`
}

func (project *Project) ServiceNames() []string {
	var names []string
	for _, service := range project.Services {
		names = append(names, service.Name)
	}
	return names
}

func (project *Project) NetworkNames() []string {
	var names []string
	for _, network := range project.Networks {
		names = append(names, network.Name)
	}
	return names
}

func (project Project) GetService(name string) docker.Service {
	for _, service := range project.Services {
		if service.Name == name {
			return service
		}
	}
	return docker.Service{}
}

// Restructure services based on dependencies
func (project *Project) RestructureServices(logger *zap.Logger) {
	logger.Info("Organising services based on dependencies hierarchy")
	restructureServices := docker.Services{}
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
	for _, service := range project.Services {
		if !find(len(service.DependsOn), numDepends) {
			numDepends = append(numDepends, len(service.DependsOn))
		}
	}

	sort.Ints(numDepends)

	for _, nd := range numDepends {
		for _, service := range project.Services {
			if len(service.DependsOn) == nd {
				restructureServices = append(restructureServices, service)
			}
		}
	}

	project.Services = restructureServices
}

func CreateProject(dockerComposePath, projectPath string, logger *zap.Logger) Project {
	outputProject := Project{}
	composeFile, err := ioutil.ReadFile(dockerComposePath)
	if err != nil {
		logger.Fatal("Unable to read docker-compose file.")
	}
	rawData := make(map[interface{}]interface{})
	err2 := yaml.Unmarshal(composeFile, &rawData)
	if err2 != nil {
		logger.Fatal("Unable to extract docker-compose file.")
	}
	slicedProjectPath := strings.Split(projectPath, "/")

	outputProject.Name = slicedProjectPath[len(slicedProjectPath)-2]
	outputProject.WorkingDir = projectPath

	outputProject.Services, outputProject.Core = extractServices(rawData, projectPath, logger)
	outputProject.Networks = extractNetworks(rawData, logger)
	outputProject.Volumes = extractVolumes(rawData, logger)

	outputProject.ComposeFile = composeFile

	outputProject.RestructureServices(logger)

	return outputProject
}

func extractSingleService(serviceName string, serviceConfig interface{}, projectPath string, logger *zap.Logger) docker.Service {
	dService := docker.Service{}
	logger.Info(fmt.Sprintf("Extracting %s", serviceName))

	// Service name
	dService.Name = serviceName

	// Build options and setups
	buildOpt := serviceConfig.(map[string]interface{})["build"].(map[string]interface{})
	dService.BuildOpt.Context = projectPath
	dService.BuildOpt.Dockerfile = buildOpt["dockerfile"].(string)

	// Container name
	dService.ContainerName = serviceConfig.(map[string]interface{})["container_name"].(string)

	// Depends On
	if dependsOn, ok := serviceConfig.(map[string]interface{})["depends_on"].([]interface{}); ok {
		for _, dp := range dependsOn {
			// fmt.Printf("%s\n", dp.(string))
			dService.DependsOn = append(dService.DependsOn, dp.(string))
		}
	} else {
		// Crucial as this is required to sort all services based on dependencies
		dService.DependsOn = make([]string, 0)
	}

	// Environment variables
	if envVarsOpt, ok := serviceConfig.(map[string]interface{})["environment"].([]interface{}); ok {
		for _, envVars := range envVarsOpt {
			dService.Environment = append(dService.Environment, envVars.(string))
		}
	}

	// Restart
	if restartOpt, ok := serviceConfig.(map[string]interface{})["restart"].(string); ok {
		dService.Restart = restartOpt
	}

	// Networks
	if networkOpts, ok := serviceConfig.(map[string]interface{})["networks"].(map[string]interface{}); ok {
		for name, network := range networkOpts {
			// fmt.Printf("%s:%s\n", name, network.(map[string]interface{})["ipv4_address"].(string))
			dService.Networks = append(dService.Networks, docker.ServiceNetwork{
				Name: name,
				IPv4: network.(map[string]interface{})["ipv4_address"].(string),
			})
		}
	}

	if volumeOpts, ok := serviceConfig.(map[string]interface{})["volumes"].([]interface{}); ok {
		for _, volume := range volumeOpts {
			// fmt.Printf("%s\n", volume.(string))
			fromStringToVolume := func(volStr string) docker.ServiceVolume {
				separateValues := strings.Split(volStr, ":")
				if len(separateValues) >= 2 {
					return docker.ServiceVolume{
						Type:        docker.VolumeTypeBind,
						Source:      separateValues[0],
						Destination: separateValues[1],
					}
				}
				return docker.ServiceVolume{}
			}
			dService.Volumes = append(dService.Volumes, fromStringToVolume(volume.(string)))
		}
	} else {
		dService.Volumes = append(dService.Volumes, docker.ServiceVolume{})
	}

	return dService
}

func extractServices(rawData map[interface{}]interface{}, projectPath string, logger *zap.Logger) (docker.Services, docker.Service) {
	logger.Debug("Extracting all services")
	outputServices := docker.Services{}
	coreService := docker.Service{}
	rawServices := rawData["services"].(map[string]interface{})

	for serviceName, serviceConfig := range rawServices {
		if serviceName == "core" {
			logger.Info("Extracting core separately")
			coreService = extractSingleService(serviceName, serviceConfig, projectPath, logger)
			continue
		}

		dService := extractSingleService(serviceName, serviceConfig, projectPath, logger)
		outputServices = append(outputServices, dService)
	}

	return outputServices, coreService
}

func extractNetworks(rawData map[interface{}]interface{}, logger *zap.Logger) docker.Networks {

	logger.Debug("Extracting networks")
	outputNetworks := docker.Networks{}

	if rawNetworks, ok := rawData["networks"].(map[string]interface{}); ok {
		for networkName, rawNetwork := range rawNetworks {
			dNetwork := docker.Network{}
			dNetwork.Name = networkName
			dNetwork.CheckDuplicate = true
			dNetwork.EnableIPv6 = false
			dNetwork.Internal = false
			dNetwork.Driver = rawNetwork.(map[string]interface{})["driver"].(string)

			ipam := rawNetwork.(map[string]interface{})["ipam"]
			ipamConfig := ipam.(map[string]interface{})["config"].([]interface{})

			for _, config := range ipamConfig {
				ipamConfig := network.IPAMConfig{
					Subnet:  config.(map[string]interface{})["subnet"].(string),
					Gateway: config.(map[string]interface{})["gateway"].(string),
				}
				dNetwork.Ipam.Config = append(dNetwork.Ipam.Config, ipamConfig)
			}
			outputNetworks = append(outputNetworks, dNetwork)
		}
	}
	return outputNetworks
}

func extractVolumes(rawData map[interface{}]interface{}, logger *zap.Logger) docker.Volumes {
	logger.Debug("Extracting volumes")
	outputVolumes := docker.Volumes{}
	if rawVolumes, ok := rawData["volumes"].(map[string]interface{}); ok {
		for volumeName := range rawVolumes {
			dVolume := docker.Volume{}
			dVolume.Name = volumeName
			outputVolumes = append(outputVolumes, dVolume)
		}
	}
	return outputVolumes
}

func extractConfig(rawData map[interface{}]interface{}) {

}

func extractSecrets(rawData map[interface{}]interface{}) {

}

func DisplayProject(project *Project) {
	for _, service := range project.Services {
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

	for _, networks := range project.Networks {
		fmt.Printf("Network Name: %s\n", networks.Name)
		fmt.Printf("Network Driver: %s\n", networks.Driver)
		fmt.Printf("Network IPAM Subnet: %s\n", networks.Ipam.Config[0].Subnet)
		fmt.Printf("Network IPAM Gateway: %s\n", networks.Ipam.Config[0].Gateway)
		fmt.Printf("=====\n")
	}

	for _, volume := range project.Volumes {
		fmt.Printf("Volume Name: %s\n", volume.Name)
	}
}
