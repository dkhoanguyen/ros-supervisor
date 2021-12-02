package compose

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/dkhoanguyen/ros-supervisor/models/compose"
	"github.com/dkhoanguyen/ros-supervisor/models/docker"
	"github.com/docker/cli/cli"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/pools"
	"github.com/pkg/errors"
)

func Build(ctx context.Context, dockerClient *client.Client, project compose.Project) {

}

func BuildSingle(ctx context.Context, dockerClient *client.Client, projectName string, targetService docker.Service) (string, error) {

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
		}
	}

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

func PrepareLocalBuildContext(projectName string, targetService docker.Service, archiveOpts *archive.TarOptions) io.ReadCloser {

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

func PrepareImageBuildOptions(projectName string, targetService docker.Service) types.ImageBuildOptions {
	// Prepare tag
	tag := []string{projectName + targetService.Name + ":" + "latest"}
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
