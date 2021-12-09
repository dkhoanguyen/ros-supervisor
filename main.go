package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	dc "github.com/dkhoanguyen/ros-supervisor/cmd/compose"
	"github.com/dkhoanguyen/ros-supervisor/models/compose"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/pools"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/pkg/errors"
)

func imageBuild(dockerClient *client.Client, project compose.Project) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*120)
	defer cancel()

	tar, err := archive.TarWithOptions(project.Services[0].BuildOpt.Context, &archive.TarOptions{})
	if err != nil {
		return err
	}

	buildCtx, _ := compress(tar)
	opts := types.ImageBuildOptions{
		Dockerfile: project.Services[0].BuildOpt.Dockerfile,
		Tags:       []string{"core:v0.0.1"},
		Remove:     true,
		NoCache:    true,
	}
	res, err := dockerClient.ImageBuild(ctx, buildCtx, opts)
	if err != nil {
		fmt.Printf("Test111\n")
		panic(err)
		// return err
	}
	fmt.Printf("Test2222\n")

	defer res.Body.Close()

	err = print(res.Body)

	return nil
}

// Compress the build context for sending to the API
func compress(buildCtx io.ReadCloser) (io.ReadCloser, error) {
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

func StartContainer(dockerClient *client.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*120)
	defer cancel()

	resp, err := dockerClient.ContainerCreate(ctx, &container.Config{
		Image: "latest",
	}, nil, nil, nil, "")
	if err != nil {
		panic(err)
	}

	if err := dockerClient.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	statusCh, errCh := dockerClient.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			panic(err)
		}
	case <-statusCh:
	}

	out, err := dockerClient.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		panic(err)
	}

	stdcopy.StdCopy(os.Stdout, os.Stderr, out)

	return nil
}

type ErrorLine struct {
	Error       string      `json:"error"`
	ErrorDetail ErrorDetail `json:"errorDetail"`
}

type ErrorDetail struct {
	Message string `json:"message"`
}

func print(rd io.Reader) error {
	var lastLine string

	scanner := bufio.NewScanner(rd)
	for scanner.Scan() {
		lastLine = scanner.Text()
		fmt.Println(scanner.Text())
	}

	errLine := &ErrorLine{}
	json.Unmarshal([]byte(lastLine), errLine)
	if errLine.Error != "" {
		return errors.New(errLine.Error)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func main() {
	project := compose.CreateProject("docker-compose.yml", "/home/khoa/research/code/github/ros_docker/")
	// compose.DisplayProject(&project)

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*120)
	defer cancel()

	containers := dc.ListAllContainers(ctx, cli)
	for _, container := range containers {
		names := container.Names
		for _, name := range names {
			if strings.Contains(name, project.Name) {
				ID := container.ID
				dc.StopServiceByID(ctx, cli, ID)
			}
		}
	}
	dc.Build(ctx, cli, &project)
	dc.CreateContainers(ctx, &project, cli)
	dc.StartAllServiceContainer(ctx, cli, &project)
	fmt.Print("test")
}
