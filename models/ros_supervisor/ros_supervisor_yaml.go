package ros_supervisor

import (
	"fmt"
	"strings"
)

type RSService struct {
	Name  string
	Repos []string
}

type RosSupervisorYaml struct {
	Services []RSService
}

func MakeRosSupervisorYaml(yfile map[interface{}]interface{}) RosSupervisorYaml {
	rsYaml := RosSupervisorYaml{}
	services := yfile["services"].(map[string]interface{})

	for serviceName, value := range services {
		repoListInterface := value.(map[string]interface{})["repo"]
		repoList := repoListInterface.([]interface{})
		rsService := RSService{
			Name:  serviceName,
			Repos: make([]string, 0),
		}
		for _, repo := range repoList {
			rsService.Repos = append(rsService.Repos, repo.(string))
			repoInfo := strings.Split(repo.(string), "-")
			fmt.Printf("Target branch: %s\n", repoInfo[0])
			fmt.Printf("Target URL: %s\n", repoInfo[1])
		}

		rsYaml.Services = append(rsYaml.Services, rsService)
	}

	return rsYaml

}
