package supervisor

import (
	"context"
	"strings"

	"github.com/dkhoanguyen/ros-supervisor/pkg/docker"
	"github.com/dkhoanguyen/ros-supervisor/pkg/github"
	gh "github.com/google/go-github/github"
	"go.uber.org/zap"
)

type Services []Service
type Service struct {
	Name string

	DockerService docker.Service
	Repos         []github.Repo
	UpdateReady   bool
}

func MakeServices(rawData map[interface{}]interface{}, ctx context.Context, githubClient *gh.Client, logger *zap.Logger) Services {
	supServices := []Service{}
	services := rawData["services"].(map[interface{}]interface{})

	for serviceName, serviceConfig := range services {
		supService := Service{}
		supService.Name = serviceName.(string)
		repoLists := serviceConfig.([]interface{})
		for _, repoData := range repoLists {
			branch := repoData.(map[interface{}]interface{})["branch"].(string)
			url := repoData.(map[interface{}]interface{})["url"].(string)

			if commit, ok := repoData.(map[interface{}]interface{})["current_commit"].(string); ok {
				repo := github.MakeRepository(url, branch, commit)
				supService.Repos = append(supService.Repos, repo)
			} else {
				repo := github.MakeRepository(url, branch, "")
				repo.GetUpstreamCommitUrl(ctx, githubClient, "", logger)
				supService.Repos = append(supService.Repos, repo)
			}
		}
		supServices = append(supServices, supService)
	}

	return supServices
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
	for _, dSrv := range project.Services {
		if strings.Contains(dSrv.Name, srv.Name) {
			srv.DockerService = dSrv
			return
		}
	}
}
