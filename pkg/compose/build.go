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
)

func Build(ctx context.Context, dockerClient *client.Client, project *Project) {
	for idx := range project.Services {
		_, err := BuildSingle(ctx, dockerClient, project.Name, &project.Services[idx])
		if err != nil {
			panic(err)
		}
	}
}

func BuildSingle(ctx context.Context, dockerClient *client.Client, projectName string, targetService *docker.Service) (string, error) {

	// TODO Investigate build context as a git repo
	buildCtx := PrepareLocalBuildContext(projectName, targetService, &archive.TarOptions{})
	buildOpts := PrepareImageBuildOptions(projectName, targetService)
	response, err := dockerClient.ImageBuild(ctx, buildCtx, buildOpts)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()

	imageID := ""
	progBuff := os.Stdout
	buildBuff := os.Stdout
	aux := func(msg jsonmessage.JSONMessage) {
		var result types.BuildResult
		if err := json.Unmarshal(*msg.Aux, &result); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to parse aux message: %s", err)
		} else {
			imageID = result.ID
			targetService.Image.ID = imageID[7:]
		}
	}

	// Attach imageID to Service.Image

	targetService.Image.ID = imageID
	targetService.Image.Name = projectName + "_" + targetService.Name
	targetService.Image.Tag = "v0.0.1"

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

func PrepareLocalBuildContext(projectName string, targetService *docker.Service, archiveOpts *archive.TarOptions) io.ReadCloser {

	// Initial archiving
	archiveCtx, err := archive.TarWithOptions(targetService.BuildOpt.Context, archiveOpts)
	if err != nil {
		panic(err)
	}

	// Compress archiveCtx
	buildCtx, error := CompressBuiltCtx(archiveCtx)
	if error != nil {
		panic(error)
	}

	return buildCtx
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
			return
		}
		compressWriter.Close()
		pipeWriter.Close()
	}()

	return pipeReader, nil
}
