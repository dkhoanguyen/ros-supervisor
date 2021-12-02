package compose

import (
	"io/ioutil"
	"log"
	"strings"

	"github.com/dkhoanguyen/ros-supervisor/models/docker"
	"gopkg.in/yaml.v3"
)

type Project struct {
	Name        string          `json:"name"`
	WorkingDir  string          `json:"working_dir"`
	Services    docker.Services `json:"services"`
	Networks    docker.Networks `json:"networks"`
	Volumes     docker.Volumes  `json:"volumes"`
	ComposeFile []byte          `json:"compose_file"`
}

func (project Project) ServiceNames() []string {
	var names []string
	for _, service := range project.Services {
		names = append(names, service.Name)
	}
	return names
}

func (project Project) NetworkNames() []string {
	var names []string
	for _, network := range project.Networks {
		names = append(names, network.Name)
	}
	return names
}

func CreateProject(dockerComposePath, projectPath string) Project {
	outputProject := Project{}
	composeFile, err := ioutil.ReadFile(dockerComposePath)
	if err != nil {
		log.Fatal(err)
	}
	rawData := make(map[interface{}]interface{})
	err2 := yaml.Unmarshal(composeFile, &rawData)
	if err2 != nil {
		log.Fatal(err2)
	}
	slicedProjectPath := strings.Split(projectPath, "/")

	outputProject.Name = slicedProjectPath[len(slicedProjectPath)-1]
	outputProject.WorkingDir = projectPath

	outputProject.Services = extractServices(rawData, projectPath)
	outputProject.Networks = extractNetworks(rawData)
	outputProject.Volumes = extractVolumes(rawData)

	outputProject.ComposeFile = composeFile

	return outputProject
}

func extractServices(rawData map[interface{}]interface{}, projectPath string) docker.Services {
	outputServices := docker.Services{}
	rawServices := rawData["services"].(map[string]interface{})
	for serviceName, serviceConfig := range rawServices {
		dService := docker.Service{}

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
		}

		// Environment variables
		if envVarsOpt, ok := serviceConfig.(map[string]interface{})["environment"].([]interface{}); ok {
			for _, envVars := range envVarsOpt {
				dService.Environment = append(dService.Environment, envVars.(string))
			}
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
				dService.Volumes = append(dService.Volumes, docker.ServiceVolume{
					Mount: volume.(string),
				})
			}
		} else {
			dService.Volumes = append(dService.Volumes, docker.ServiceVolume{})
		}

		outputServices = append(outputServices, dService)
	}

	return outputServices
}

func extractNetworks(rawData map[interface{}]interface{}) docker.Networks {
	outputNetworks := docker.Networks{}

	if rawNetworks, ok := rawData["networks"].(map[string]interface{}); ok {
		for networkName, _ := range rawNetworks {
			dNetwork := docker.Network{}
			dNetwork.Name = networkName
			outputNetworks = append(outputNetworks, dNetwork)
		}
	}

	return outputNetworks
}

func extractVolumes(rawData map[interface{}]interface{}) docker.Volumes {
	outputVolumes := docker.Volumes{}
	if rawVolumes, ok := rawData["volumes"].(map[string]interface{}); ok {
		for volumeName, _ := range rawVolumes {
			dVolume := docker.Volume{}
			dVolume.Name = volumeName
			outputVolumes = append(outputVolumes, dVolume)
		}
	}
	return outputVolumes
}
