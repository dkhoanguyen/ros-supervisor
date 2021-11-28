package models

type DCService struct {
	Name    string
	Env     []string
	Volumes []string
	IPV4    string
}

type DCNetwork struct {
	Name   string
	Driver string
	IPAM   struct {
		Config struct {
			Subnet  string
			Gateway string
		}
	}
}

type DockerComposeYaml struct {
	Services []DCService
	Network  []DCNetwork
}

func MakeDockerComposeYaml(yfile map[interface{}]interface{}) DockerComposeYaml {
	dcYaml := DockerComposeYaml{}
	services := yfile["services"].(map[string]interface{})

	for serviceName, value := range services {
		dcService := DCService{}
		dcService.Name = serviceName
		// Extract services
		// Extrac env vars
		serviceEnvList := value.(map[string]interface{})["environment"].([]interface{})
		for _, envVars := range serviceEnvList {
			dcService.Env = append(dcService.Env, envVars.(string))
		}

		// Extract mount volumes
		serviceVolumeList := value.(map[string]interface{})["volumes"].([]interface{})
		for _, volumes := range serviceVolumeList {
			dcService.Volumes = append(dcService.Volumes, volumes.(string))
		}
		// Extract IPV4
		// dcService.IPV4 = value.(map[string]interface{})["networks"]["ros"]["ipv4_address"].(string)

		dcYaml.Services = append(dcYaml.Services, dcService)
	}
	// network := yfile["networks"].(map[string]interface{})
	return dcYaml
}
