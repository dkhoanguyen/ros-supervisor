package github

import (
	"context"
	"strings"

	"github.com/google/go-github/github"
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

func (r *Repo) GetCurrentLocalCommit(ctx context.Context, githubClient *github.Client, commit string) string {
	if commit != "" {
		r.CurrentCommit = commit
		return commit
	} else {
		allCommits, _, err := githubClient.Repositories.ListCommits(ctx, r.Owner, r.Name, &github.CommitsListOptions{})
		if err != nil {
			panic(err)
		}
		r.CurrentCommit = *allCommits[0].SHA
		return r.CurrentCommit
	}
}

func (r *Repo) UpdateUpStreamCommit(ctx context.Context, githubClient *github.Client) string {
	allCommits, _, err := githubClient.Repositories.ListCommits(ctx, r.Owner, r.Name, &github.CommitsListOptions{})
	if err != nil {
		panic(err)
	}
	r.UpstreamCommit = *allCommits[0].SHA
	return r.UpstreamCommit
}

func (r *Repo) IsUpdateReady() bool {
	return r.UpstreamCommit != r.CurrentCommit
}
