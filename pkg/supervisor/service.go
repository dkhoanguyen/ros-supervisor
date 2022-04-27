package supervisor

import (
	"context"

	"github.com/dkhoanguyen/ros-supervisor/pkg/github"
	gh "github.com/google/go-github/github"
	"go.uber.org/zap"
)

type SupervisorServices []SupervisorService
type SupervisorService struct {
	ServiceName   string
	ContainerName string
	ContainerID   string
	Repos         []github.Repo
	UpdateReady   bool
}

func MakeServices(rawData map[interface{}]interface{}, ctx context.Context, githubClient *gh.Client, logger *zap.Logger) SupervisorServices {
	supServices := SupervisorServices{}
	services := rawData["services"].(map[interface{}]interface{})

	for serviceName, serviceConfig := range services {
		supService := SupervisorService{}
		supService.ServiceName = serviceName.(string)
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
