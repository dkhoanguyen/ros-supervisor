package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/docker/cli/cli"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/pools"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type Image struct {
	ID      string
	Name    string
	Tag     string
	Created string
}

type ImageContainerConfig struct {
	HostName   string `json:"hostName"`
	DomainName string `json:"domainName"`
}

func (img *Image) Build(
	ctx context.Context,
	dockerCli *client.Client,
	service *Service,
	logger *zap.Logger) error {

	logger.Info(fmt.Sprintf("Building service %s", service.Name))
	buildCtx, err := prepareLocalBuildContext(service, &archive.TarOptions{}, logger)
	if err != nil {

	}
	// TODO: Version as tag
	buildOpts := prepareImageBuildOptions(service, "latest")
	response, err := dockerCli.ImageBuild(ctx, buildCtx, buildOpts)
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

	// Update image ID
	img.ID = imageID

	// We need to figure out a way to read message from Message Stream and log those message for debugging build process
	err = jsonmessage.DisplayJSONMessagesStream(response.Body, buildBuff, progBuff.Fd(), true, aux)
	if err != nil {
		if jerr, ok := err.(*jsonmessage.JSONError); ok {
			// If no error code is set, default to 1
			if jerr.Code == 0 {
				jerr.Code = 1
			}
			return cli.StatusError{Status: jerr.Message, StatusCode: jerr.Code}
		}
		return err
	}

	return nil
}

func MakeImage(name string, tag string) Image {
	return Image{
		Name: name,
		Tag:  tag,
	}
}

// LOCAL BUILD CONTEXT
func prepareLocalBuildContext(service *Service, archiveOpts *archive.TarOptions, logger *zap.Logger) (io.ReadCloser, error) {

	// Initial archiving
	archiveCtx, err := archive.TarWithOptions(service.BuildOpt.Context, archiveOpts)
	if err != nil {
		logger.Error(fmt.Sprintf("Unable to create local build context with error: %s", err))
		return nil, err
	}

	// Compress archiveCtx
	buildCtx, err := compressBuiltCtx(archiveCtx)
	if err != nil {
		logger.Error(fmt.Sprintf("Unable to compress local build context with error: %s", err))
		return nil, err
	}

	return buildCtx, nil
}

func prepareImageBuildOptions(targetService *Service, version string) types.ImageBuildOptions {
	// Prepare tag
	// TODO: We should tag images with software version - using git commit hash
	tag := []string{targetService.Name + ":" + "latest"}
	return types.ImageBuildOptions{
		Tags:           tag,
		SuppressOutput: false,
		Dockerfile:     targetService.BuildOpt.Dockerfile,
		BuildArgs:      targetService.BuildOpt.Args,
		Remove:         true,
	}
}

// Compress the build context for sending to the API
func compressBuiltCtx(buildCtx io.ReadCloser) (io.ReadCloser, error) {
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
