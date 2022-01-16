package github

import (
	"context"
	"fmt"
	"os"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/go-github/github"
	"go.uber.org/zap"
)

type Repo struct {
	Name           string
	Directory      string
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

func (r *Repo) Clone(directory string, logger *zap.Logger) string {
	url := r.Url
	branch := r.Branch

	// Check if project already exists
	directory = fmt.Sprintf("%s%s/", directory, r.Name)
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		// If not then clone it
		_, err := gogit.PlainClone(directory, false, &gogit.CloneOptions{
			URL:           url,
			ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branch)),
			SingleBranch:  true,
		})
		if err != nil {
			logger.Fatal(fmt.Sprintf("Cannot clone %s due to error %s", r.Name, err))
		}
	} else {
		// Open the repo
		repo, err := gogit.PlainOpen(directory)
		if err != nil {
			logger.Fatal(fmt.Sprintf("Cannot open %s due to error %s", r.Name, err))
		}
		// Get the working directory for the repository
		w, err := repo.Worktree()
		if err != nil {
			logger.Fatal(fmt.Sprintf("Cannot get worktree of %s due to error %s", r.Name, err))
		}

		// Pull the latest changes from the origin remote and merge into the current branch
		err = w.Pull(&gogit.PullOptions{RemoteName: "origin"})
		if err != nil {
			logger.Warn(fmt.Sprintf("Cannot pull the latest commit of %s due to error %s", r.Name, err))
		}
	}
	r.Directory = directory
	return directory
}

func (r *Repo) GetFullPath(directory string, logger *zap.Logger) string {
	directory = fmt.Sprintf("%s%s/", directory, r.Name)
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		r.Directory = directory
		return directory
	} else {
		logger.Error(fmt.Sprintf("Project path %s does not exist", directory))
		return ""
	}
}

func (r *Repo) CloneMain(logger *zap.Logger) {

}
