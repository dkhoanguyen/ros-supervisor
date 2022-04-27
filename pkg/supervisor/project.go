package supervisor

import (
	"fmt"

	"github.com/dkhoanguyen/ros-supervisor/pkg/github"
	"go.uber.org/zap"
)

type ProjectContext struct {
	UseGitContext bool
	ProjectPath   string
	TargetRepo    github.Repo
}

func MakeProject(rawData map[interface{}]interface{}, logger *zap.Logger) ProjectContext {
	ctx := ProjectContext{}
	rawCtx := rawData["context"].(map[interface{}]interface{})

	ctx.UseGitContext = rawCtx["use_git_context"].(bool)
	ctx.TargetRepo = github.MakeRepository(rawCtx["url"].(string), rawCtx["branch"].(string), "")
	return ctx
}

func (pj *ProjectContext) PrepareContextFromGit(projectDir string, logger *zap.Logger) string {
	if pj.UseGitContext {
		logger.Info("Cloning project dir")
		projectPath, err := pj.TargetRepo.Clone(projectDir, logger)
		if err != nil {
			logger.Error(fmt.Sprintf("Failed to clone project due to error %s", err))
		}
		pj.ProjectPath = projectPath
		return projectPath
	} else {
		return pj.TargetRepo.GetFullPath(projectDir, logger)
	}
}
