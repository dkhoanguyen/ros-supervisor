package supervisor

import (
	"context"
	"fmt"
	"strings"

	"github.com/dkhoanguyen/ros-supervisor/pkg/docker"
	"github.com/dkhoanguyen/ros-supervisor/pkg/github"
	gh "github.com/google/go-github/github"
	"go.uber.org/zap"
)

const (
	PRODUCER    string = "producer"
	CONSUMER    string = "consumer"
	DISTRIBUTOR string = "distributor"
)

type Services []Service
type Service struct {
	Name      string
	Type      string
	DependsOn []string

	DockerService *docker.Service
	Repos         []github.Repo
	UpdateReady   bool
}

func MakeServices(
	rawData map[interface{}]interface{},
	dockerProject *docker.DockerProject,
	ctx context.Context,
	githubClient *gh.Client,
	logger *zap.Logger,
) Services {
	supServices := []Service{}
	services := rawData["services"].(map[interface{}]interface{})

	for serviceName, serviceConfig := range services {
		for idx, service := range dockerProject.Services {
			if strings.Contains(serviceName.(string), service.Name) {
				supService := MakeService(ctx, serviceConfig.(map[interface{}]interface{}),
					&dockerProject.Services[idx], githubClient, logger)
				supService.Name = serviceName.(string)

				supServices = append(supServices, supService)
			}
		}
	}
	return supServices
}

func MakeService(
	ctx context.Context,
	config map[interface{}]interface{},
	dockerService *docker.Service,
	githubClient *gh.Client,
	logger *zap.Logger,
) Service {
	service := Service{}
	repoLists := config["repos"].([]interface{})

	// Repo
	for _, repoData := range repoLists {
		branch := repoData.(map[interface{}]interface{})["branch"].(string)
		url := repoData.(map[interface{}]interface{})["url"].(string)

		if commit, ok := repoData.(map[interface{}]interface{})["current_commit"].(string); ok {
			repo := github.MakeRepository(url, branch, commit)
			service.Repos = append(service.Repos, repo)
		} else {
			repo := github.MakeRepository(url, branch, "")
			repo.GetUpstreamCommitUrl(ctx, githubClient, "", logger)
			service.Repos = append(service.Repos, repo)
		}
	}
	// Type
	service.Type = config["type"].(string)

	// Depends on
	if dependsOnOpt, exist := config["depends_on"].([]interface{}); exist {
		for _, dependsOn := range dependsOnOpt {
			service.DependsOn = append(service.DependsOn, dependsOn.(string))
		}
	}
	// Docker Service
	service.DockerService = dockerService

	return service
}

func (srv *Service) IsUpdateReady(ctx context.Context, gitCli *gh.Client, logger *zap.Logger) bool {
	for _, repo := range srv.Repos {
		repo.UpdateUpStreamCommit(ctx, gitCli, logger)
		if repo.IsUpdateReady() {
			srv.UpdateReady = true
			return true
		}
	}
	srv.UpdateReady = false
	return false
}

func (srv *Service) AttachDockerService(project *docker.DockerProject) {
	for idx, dSrv := range project.Services {
		if strings.Contains(dSrv.Name, srv.Name) {
			srv.DockerService = &project.Services[idx]
			return
		}
	}
}

func (srv *Service) Print() {
	fmt.Printf("Sup Service Name: %s\n", srv.Name)
	fmt.Printf("Type: %s\n", srv.Type)
	fmt.Printf("Depend on: %s\n", srv.DependsOn)
	for _, repo := range srv.Repos {
		fmt.Printf("Repo name: %s\n", repo.Name)
		fmt.Printf("Owner name: %s\n", repo.Owner)
		fmt.Printf("URL: %s\n", repo.Url)
		fmt.Printf("Branch name: %s\n", repo.Branch)
		fmt.Printf("Local Commit: %s\n", repo.CurrentCommit)
		fmt.Printf("----\n")
	}
	fmt.Println("Docker Service:")
	srv.DockerService.Print()
}
