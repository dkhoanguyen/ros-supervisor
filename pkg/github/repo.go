package github

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/dkhoanguyen/ros-supervisor/internal/utils"
	"github.com/go-git/go-git/v5"
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

// Factory method
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

func (r *Repo) GetUpstreamCommitUrl(ctx context.Context, githubClient *github.Client, commit string, logger *zap.Logger) (string, error) {
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

func (r *Repo) Clone(directory string, logger *zap.Logger) (string, error) {
	url := r.Url
	branch := r.Branch

	// Check if project already exists
	directory = fmt.Sprintf("%s%s", directory, r.Name)
	fmt.Println(directory)
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		// If not then clone it
		logger.Info("Project does not exist, cloning it...")
		pingSuccess := utils.Ping("www.github.com", 3, logger)

		// We should handle cases where there is no internet connectivity
		if !pingSuccess {
			logger.Warn("Fail to ping github.com")
			return "", errors.New("Cannot clone due to no internet connectivity")
		}
		_, err := gogit.PlainClone(directory, false, &gogit.CloneOptions{
			URL:           url,
			ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branch)),
			SingleBranch:  true,
		})
		if err != nil {
			logger.Error(fmt.Sprintf("Cannot clone %s due to error %s", r.Name, err))
			return "", err
		}
	} else {
		logger.Info("Project exists")
		currentCommit := r.GetLocalCommit(directory, logger)
		upstreamCommit := r.GetUpstreamCommit(directory, logger)
		currentBranch := r.GetCurrentBranch(directory, logger)
		reclone := false

		// First check branch. If branch is different from the specified branch then reclone
		if currentBranch == "" || currentCommit != r.Branch {
			logger.Info("Different branch. Reclone with correct branch")
			reclone = true
		}

		if !reclone {
			// Then compare current commit and upstream commit. If they are different then reclone
			if currentCommit != upstreamCommit || currentCommit == "" {
				logger.Info("Different commit. Reclone with the latest upstream commit")
				reclone = true
			}
		}

		if reclone {
			// Remove the cloned project
			// Repo is removed here because for some reasons we are unable to pull it due to non-fast-forward error
			// As such we need to remove the repo and reclone it
			// Before doing so, it's probably better to check if we can still ping github.com
			pingSuccess := utils.Ping("www.github.com", 3, logger)

			// We should handle cases where there is no internet connectivity
			if !pingSuccess {
				logger.Warn("Fail to ping github.com. Potentially no internet connectivity. Using old setup")
				return directory, errors.New("Cannot clone due to no internet connectivity")
			}

			// Delete repo and reclone it
			// NOTE: This should be changed to either pull or fetch merge in the future
			err = os.RemoveAll(directory)
			if err != nil {
				logger.Error(fmt.Sprintf("Cannot remove project directory with error %v", err))
			}
			_, err := gogit.PlainClone(directory, false, &gogit.CloneOptions{
				URL:           url,
				ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", branch)),
				SingleBranch:  true,
			})
			if err != nil {
				logger.Error(fmt.Sprintf("Cannot clone %s due to error %s", r.Name, err))
				return directory, err
			}

		}
	}
	r.Directory = directory
	return directory, nil
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

func (r *Repo) GetLocalCommit(directory string, logger *zap.Logger) string {
	repo, err := gogit.PlainOpen(directory)
	if err != nil {
		logger.Error(fmt.Sprintf("Cannot open %s due to error %s", r.Name, err))
		return ""
	}
	headRef, err := repo.Head()
	cIter, err := repo.Log(&git.LogOptions{From: headRef.Hash()})
	commit, err := cIter.Next()
	if err != nil {
		logger.Error(fmt.Sprintf("Cannot get %s local commit due to error %s", r.Name, err))
		return ""
	}
	return commit.Hash.String()
}

func (r *Repo) GetUpstreamCommit(directory string, logger *zap.Logger) string {
	repo, err := gogit.PlainOpen(directory)
	if err != nil {
		logger.Error(fmt.Sprintf("Cannot open %s due to error %s", r.Name, err))
		return ""
	}
	remoteRefs, err := repo.References()
	commit, err := remoteRefs.Next()
	commit, err = remoteRefs.Next()
	if err != nil {
		logger.Error(fmt.Sprintf("Cannot get %s local commit due to error %s", r.Name, err))
		return ""
	}
	return commit.Hash().String()
}

func (r *Repo) GetCurrentBranch(directory string, logger *zap.Logger) string {
	repo, err := gogit.PlainOpen(directory)
	if err != nil {
		logger.Error(fmt.Sprintf("Cannot open %s due to error %s", r.Name, err))
		return ""
	}

	headRef, err := repo.Head()
	return headRef.Name().Short()
}

func (r *Repo) Pull(directory string, logger *zap.Logger) {
	repo, err := gogit.PlainOpen(directory)
	if err != nil {
		logger.Error(fmt.Sprintf("Cannot open %s due to error %s", r.Name, err))
	}

	err = repo.Fetch(&gogit.FetchOptions{RemoteName: "origin"})
	if err != nil {
		logger.Error(fmt.Sprintf("Cannot fetch %s due to error %s", r.Name, err))
	}
}
