package compose

import (
	"io/ioutil"
	"log"

	"github.com/dkhoanguyen/ros-supervisor/models/docker"
	"gopkg.in/yaml.v3"
)

type Project struct {
	Name        string          `json:"name"`
	WorkingDir  string          `json:"working_dir"`
	Services    docker.Services `json:"services"`
	Networks    docker.Networks `json:"networks"`
	Volumes     docker.Volumes  `json:"volumes"`
	ComposeFile string          `json:"compose_file"`
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

func CreateProject(path string) Project {
	outputProject := Project{}
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	rawData := make(map[interface{}]interface{})
	err2 := yaml.Unmarshal(yamlFile, &rawData)
	if err2 != nil {
		log.Fatal(err2)
	}
	outputProject.Services = extractServices(rawData)
	outputProject.Networks = extractNetworks(rawData)
	outputProject.Volumes = extractVolumes(rawData)
}

func extractServices(rawData map[interface{}]interface{}) docker.Services {
	outputServices := docker.Services{}
	rawServices := rawData["services"].(map[string]interface{})
	for serviceName, _ := range rawServices {
		dService := docker.Service{}
		dService.Name = serviceName
		dService.Config.Name = serviceName
		outputServices = append(outputServices, dService)
	}

	return outputServices
}

func extractNetworks(rawData map[interface{}]interface{}) docker.Networks {
	outputNetworks := docker.Networks{}
	rawNetworks := rawData["networks"].(map[string]interface{})

	for networkName, _ := range rawNetworks {
		dNetwork := docker.Network{}
		dNetwork.Name = networkName
		outputNetworks = append(outputNetworks, dNetwork)
	}

	return outputNetworks
}

func extractVolumes(rawData map[interface{}]interface{}) docker.Volumes {
	outputVolumes := docker.Volumes{}
	rawVolumes := rawData["volumes"].(map[string]interface{})

	for volumeName, _ := range rawVolumes {
		dVolume := docker.Volume{}
		dVolume.Name = volumeName
		outputVolumes = append(outputVolumes, dVolume)
	}

	return outputVolumes
}
