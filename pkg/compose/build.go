package compose

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dkhoanguyen/ros-supervisor/pkg/docker"
	"github.com/docker/cli/cli"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/builder/remotecontext/git"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/pools"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func BuildAll(ctx context.Context, dockerClient *client.Client, project *Project, logger *zap.Logger) error {

	err := BuildCore(ctx, dockerClient, project, logger)
	if err != nil {
		return err
	}
	err = BuildServices(ctx, dockerClient, project, logger)
	if err != nil {
		return err
	}
	return nil
}

func BuildCore(ctx context.Context, dockerClient *client.Client, project *Project, logger *zap.Logger) error {
	logger.Info("Building core")
	_, err := BuildSingle(ctx, dockerClient, project.Name, &project.Core, logger)
	if err != nil {
		logger.Error(fmt.Sprintf("Unable to build core with errror: %s", err))
		return err
	}
	return nil
}

func BuildServices(ctx context.Context, dockerClient *client.Client, project *Project, logger *zap.Logger) error {
	logger.Info("Building services")
	for idx := range project.Services {
		_, err := BuildSingle(ctx, dockerClient, project.Name, &project.Services[idx], logger)
		if err != nil {
			logger.Error(fmt.Sprintf("Unable to build service %s with errror: %s", project.Services[idx].Name, err))
			return err
		}
	}
	return nil
}

func BuildSingle(ctx context.Context, dockerClient *client.Client, projectName string, targetService *docker.Service, logger *zap.Logger) (string, error) {

	logger.Info(fmt.Sprintf("Building service %s", targetService.Name))
	// TODO Investigate build context as a git repo
	buildCtx, err := PrepareLocalBuildContext(projectName, targetService, &archive.TarOptions{}, logger)
	if err != nil {

	}
	buildOpts := PrepareImageBuildOptions(projectName, targetService)
	response, err := dockerClient.ImageBuild(ctx, buildCtx, buildOpts)
	if err != nil {
		logger.Fatal(fmt.Sprintf("Error: %s", err))
	}
	defer response.Body.Close()

	imageID := ""
	progBuff := os.Stdout
	buildBuff := os.Stdout
	aux := func(msg jsonmessage.JSONMessage) {
		var result types.BuildResult
		if err := json.Unmarshal(*msg.Aux, &result); err != nil {
			logger.Error(fmt.Sprintf("Failed to parse aux message: %s", err))
		} else {
			logger.Info(fmt.Sprintf("%s", msg.Stream))
		}
	}

	// Attach imageID to Service.Image

	targetService.Image.ID = imageID
	targetService.Image.Name = projectName + "_" + targetService.Name
	targetService.Image.Tag = "v0.0.1"

	// We need to figure out a way to read message from Message Stream and log those message for debugging build process
	err = jsonmessage.DisplayJSONMessagesStream(response.Body, buildBuff, progBuff.Fd(), true, aux)
	if err != nil {
		if jerr, ok := err.(*jsonmessage.JSONError); ok {
			// If no error code is set, default to 1
			if jerr.Code == 0 {
				jerr.Code = 1
			}
			return "", cli.StatusError{Status: jerr.Message, StatusCode: jerr.Code}
		}
		return "", err
	}

	return imageID, nil
}

// LOCAL BUILD CONTEXT
func PrepareLocalBuildContext(projectName string, targetService *docker.Service, archiveOpts *archive.TarOptions, logger *zap.Logger) (io.ReadCloser, error) {

	// Initial archiving
	archiveCtx, err := archive.TarWithOptions(targetService.BuildOpt.Context, archiveOpts)
	if err != nil {
		logger.Error(fmt.Sprintf("Unable to create local build context with error: %s", err))
		return nil, err
	}

	// Compress archiveCtx
	buildCtx, err := CompressBuiltCtx(archiveCtx)
	if err != nil {
		logger.Error(fmt.Sprintf("Unable to compress local build context with error: %s", err))
		return nil, err
	}

	return buildCtx, nil
}

func PrepareImageBuildOptions(projectName string, targetService *docker.Service) types.ImageBuildOptions {
	// Prepare tag
	tag := []string{projectName + "_" + targetService.Name + ":" + "latest"}
	return types.ImageBuildOptions{
		Tags:           tag,
		SuppressOutput: false,
		Dockerfile:     targetService.BuildOpt.Dockerfile,
		BuildArgs:      targetService.BuildOpt.Args,
		Remove:         true,
	}
}

// Compress the build context for sending to the API
func CompressBuiltCtx(buildCtx io.ReadCloser) (io.ReadCloser, error) {
	pipeReader, pipeWriter := io.Pipe()

	go func() {
		compressWriter, err := archive.CompressStream(pipeWriter, archive.Gzip)
		if err != nil {
			pipeWriter.CloseWithError(err)
		}
		defer buildCtx.Close()

		if _, err := pools.Copy(compressWriter, buildCtx); err != nil {
			pipeWriter.CloseWithError(
				errors.Wrap(err, "failed to compress context"))
			compressWriter.Close()
		}
		compressWriter.Close()
		pipeWriter.Close()
	}()

	return pipeReader, nil
}

// GITHUB BUILD CONTEXT
func PrepareGitBuildContext(gitURL string, dockerfileName string, targetService *docker.Service, archiveOpts *archive.TarOptions, logger *zap.Logger) (io.ReadCloser, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, errors.Wrapf(err, "unable to find 'git'")
	}
	absContextDir, err := git.Clone(gitURL)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to 'git clone' to temporary context directory")
	}

	absContextDir, err = ResolveAndValidateContextPath(absContextDir)
	if err != nil {
		return nil, err
	}
	relDockerfile, err := getDockerfileRelPath(absContextDir, dockerfileName)
	if err == nil && strings.HasPrefix(relDockerfile, ".."+string(filepath.Separator)) {
		return nil, errors.Errorf("the Dockerfile (%s) must be within the build context", dockerfileName)
	}

	// Initial archiving
	archiveCtx, err := archive.TarWithOptions(targetService.BuildOpt.Context, archiveOpts)
	if err != nil {
		logger.Error(fmt.Sprintf("Unable to create local build context with error: %s", err))
		return nil, err
	}

	// Compress archiveCtx
	buildCtx, err := CompressBuiltCtx(archiveCtx)
	if err != nil {
		logger.Error(fmt.Sprintf("Unable to compress local build context with error: %s", err))
		return nil, err
	}

	return buildCtx, nil
}

func ResolveAndValidateContextPath(givenContextDir string) (string, error) {
	absContextDir, err := filepath.Abs(givenContextDir)
	if err != nil {
		return "", errors.Errorf("unable to get absolute context directory of given context directory %q: %v", givenContextDir, err)
	}

	// The context dir might be a symbolic link, so follow it to the actual
	// target directory.
	//
	// FIXME. We use isUNC (always false on non-Windows platforms) to workaround
	// an issue in golang. On Windows, EvalSymLinks does not work on UNC file
	// paths (those starting with \\). This hack means that when using links
	// on UNC paths, they will not be followed.
	absContextDir, err = filepath.EvalSymlinks(absContextDir)
	if err != nil {
		return "", errors.Errorf("unable to evaluate symlinks in context path: %v", err)
	}

	stat, err := os.Lstat(absContextDir)
	if err != nil {
		return "", errors.Errorf("unable to stat context directory %q: %v", absContextDir, err)
	}

	if !stat.IsDir() {
		return "", errors.Errorf("context must be a directory: %s", absContextDir)
	}
	return absContextDir, err
}

func getDockerfileRelPath(absContextDir, givenDockerfile string) (string, error) {
	var err error

	if givenDockerfile == "-" {
		return givenDockerfile, nil
	}

	absDockerfile := givenDockerfile
	if absDockerfile == "" {
		// No -f/--file was specified so use the default relative to the
		// context directory.
		absDockerfile = filepath.Join(absContextDir, "Dockerfile")

		// Just to be nice ;-) look for 'dockerfile' too but only
		// use it if we found it, otherwise ignore this check
		if _, err = os.Lstat(absDockerfile); os.IsNotExist(err) {
			altPath := filepath.Join(absContextDir, strings.ToLower("Dockerfile"))
			if _, err = os.Lstat(altPath); err == nil {
				absDockerfile = altPath
			}
		}
	}

	// If not already an absolute path, the Dockerfile path should be joined to
	// the base directory.
	if !filepath.IsAbs(absDockerfile) {
		absDockerfile = filepath.Join(absContextDir, absDockerfile)
	}

	// Evaluate symlinks in the path to the Dockerfile too.
	//
	// FIXME. We use isUNC (always false on non-Windows platforms) to workaround
	// an issue in golang. On Windows, EvalSymLinks does not work on UNC file
	// paths (those starting with \\). This hack means that when using links
	// on UNC paths, they will not be followed.
	absDockerfile, err = filepath.EvalSymlinks(absDockerfile)
	if err != nil {
		return "", errors.Errorf("unable to evaluate symlinks in Dockerfile path: %v", err)
	}

	if _, err := os.Lstat(absDockerfile); err != nil {
		if os.IsNotExist(err) {
			return "", errors.Errorf("Cannot locate Dockerfile: %q", absDockerfile)
		}
		return "", errors.Errorf("unable to stat Dockerfile: %v", err)
	}

	relDockerfile, err := filepath.Rel(absContextDir, absDockerfile)
	if err != nil {
		return "", errors.Errorf("unable to get relative Dockerfile path: %v", err)
	}

	return relDockerfile, nil
}
