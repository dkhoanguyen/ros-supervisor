package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
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

	dc.Build(ctx, cli, &project)
	// fmt.Printf("Image Name: %s\n", project.Services[0].Image.Name)
	dc.CreateContainers(ctx, &project, cli)
	// dc.InspectNetwork(ctx, "ros_docker_ros", cli)

	// imageID, _ := dc.BuildSingle(ctx, cli, "ros_docker", project.Services[0])
	// fmt.Printf("Image ID: %s\n", imageID)

	// StartContainer(cli)

	// containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	// if err != nil {
	// 	panic(err)
	// }

	// for _, c := range containers {
	// 	// test, err := cli
	// 	// if err != nil {
	// 	// 	panic(err)
	// 	// }
	// 	container := models.MakeNodeContainer(c.Names[0], c.ID[:10], c.State)
	// 	fmt.Printf("Name: %s\n", container.GetName())
	// 	fmt.Printf("Status: %s\n", container.GetState())
	// }

	// images, err := cli.ImageList(context.Background(), types.ImageListOptions{})
	// if err != nil {
	// 	panic(err)
	// }

	// for _, image := range images {
	// 	fmt.Printf("Image: %d\n", image.Created)
	// }

	// client := github.NewClient(nil)
	// orgs, _, err := client.Organizations.List(context.Background(), "dkhoanguyen", nil)
	// ctx := context.Background()

	// psOpts := compose.psOptions{}
	// ts := oauth2.StaticTokenSource(
	// 	&oauth2.Token{AccessToken: "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"},
	// )

	// tc := oauth2.NewClient(ctx, ts)

	// client := github.NewClient(nil)

	// list all repositories for the authenticated user
	// repos, _, err := client.Repositories.List(ctx, "dkhoanguyen", nil)

	// for _, repo := range repos {
	// 	fmt.Printf("Repo Name: %s\n", *repo.Name)
	// 	fmt.Printf("Default branch : %s\n", *repo.DefaultBranch)
	// }
	// commits, _, err := client.Repositories.ListCommits(ctx, "gapaul", "dobot_magician_driver", nil)
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Printf("No commits : %d\n", len(commits))

	// // Get latest commits
	// latestCommits := commits[0]
	// fmt.Printf("Latest commit : %s\n", *latestCommits.SHA)
	// currentLocalCommits := commits[1]
	// fmt.Printf("Current commit : %s\n", *currentLocalCommits.SHA)

	// yfile := yml.ReadYaml("ros-supervisor.yml")

	// rsYaml := models.MakeRosSupervisorYaml(yfile)
	// for _, service := range rsYaml.Services {
	// 	fmt.Printf("Target: %s\n", service.Name)
	// }

	// lazyInit := api.NewServiceProxy()
	// lazyInit.WithService(compose.NewComposeService(cli.Client(), cli.ConfigFile()))

	// 	if env_vars_interface == nil {
	// 		continue
	// 	}
	// 	env_vars := env_vars_interface.([]interface{})
	// 	for _, env_var := range env_vars {
	// 		if strings.Contains(env_var.(string), "TARGET_REPO") {
	// 			fmt.Printf("%s - %s\n", service, env_var)
	// 			target_repo := env_var.(string)[len("TARGET_REPO=https://github.com/") : len(env_var.(string))-len(".git")]
	// 			user_repo := strings.Split(target_repo, "/")
	// 			fmt.Printf("User: %s\n", user_repo[0])
	// 			fmt.Printf("Repo: %s\n", user_repo[1])

	// 			commits, _, err := client.Repositories.ListCommits(ctx, user_repo[0], user_repo[1], nil)

	// 			if err != nil {
	// 				panic(err)
	// 			}
	// 			fmt.Printf("No commits : %d\n", len(commits))

	// 			// Get latest commits
	// 			latestCommits := commits[0]
	// 			fmt.Printf("Latest commit : %s\n", *latestCommits.SHA)
	// 			currentLocalCommits := commits[1]
	// 			fmt.Printf("Current commit : %s\n", *currentLocalCommits.SHA)
	// 		}
	// 	}
	// }

	// client.
	// project,err :=
}
