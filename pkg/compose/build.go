package compose

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/dkhoanguyen/ros-supervisor/pkg/docker"
	"github.com/docker/cli/cli"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/pools"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func BuildAll(ctx context.Context, dockerClient *client.Client, project *DockerProject, logger *zap.Logger) error {

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

func BuildCore(ctx context.Context, dockerClient *client.Client, project *DockerProject, logger *zap.Logger) error {
	logger.Info("Building core")
	_, err := BuildSingle(ctx, dockerClient, project.Name, &project.Core, logger)
	if err != nil {
		logger.Error(fmt.Sprintf("Unable to build core with errror: %s", err))
		return err
	}
	return nil
}

func BuildServices(ctx context.Context, dockerClient *client.Client, project *DockerProject, logger *zap.Logger) error {
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
