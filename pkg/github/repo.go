package github

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/github"
	"go.uber.org/zap"
)

type Repo struct {
	Name           string
	Owner          string
	Url            string
	Branch         string
	CurrentCommit  string
	UpstreamCommit string
}

func MakeRepository(url, branch, currentCommit string) Repo {
	splitUrl := strings.Split(url, "/")
	name := ""
	owner := ""
	for indx, phrase := range splitUrl {
		if phrase == "github.com" {
			owner = splitUrl[indx+1]
			name = splitUrl[indx+2]
		}
	}
	return Repo{
		Name:   name,
		Owner:  owner,
		Url:    url,
		Branch: branch,
	}
}

func (r *Repo) GetCurrentLocalCommit(ctx context.Context, githubClient *github.Client, commit string, logger *zap.Logger) (string, error) {
	if commit != "" {
		r.CurrentCommit = commit
		return commit, nil
	} else {
		allCommits, _, err := githubClient.Repositories.ListCommits(ctx, r.Owner, r.Name, &github.CommitsListOptions{})
		if err != nil {
			logger.Error(fmt.Sprintf("Unable to query upstream commits: %v", err))
			return "", err
		}
		r.CurrentCommit = *allCommits[0].SHA
		return r.CurrentCommit, nil
	}
}

func (r *Repo) UpdateUpStreamCommit(ctx context.Context, githubClient *github.Client, logger *zap.Logger) (string, error) {
	allCommits, _, err := githubClient.Repositories.ListCommits(ctx, r.Owner, r.Name, &github.CommitsListOptions{})
	if err != nil {
		logger.Error(fmt.Sprintf("Unable to query upstream commits: %v", err))
	}
	r.UpstreamCommit = *allCommits[0].SHA
	return r.UpstreamCommit, err
}

func (r *Repo) IsUpdateReady() bool {
	return r.UpstreamCommit != r.CurrentCommit
}
