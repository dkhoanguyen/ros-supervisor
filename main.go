package main

import (
	"github.com/dkhoanguyen/ros-supervisor/pkg/supervisor"
)

func main() {
	rs := supervisor.RosSupervisor{}
	dockerCli, gitCli := supervisor.PrepareSupervisor(&rs)
	supervisor.StartSupervisor(&rs, dockerCli, gitCli)
	// project := compose.CreateProject("docker-compose.yml", "/home/khoa/research/code/github/ros_docker/")
	// // compose.DisplayProject(&project)

	// cli, err := client.NewClientWithOpts(client.FromEnv)
	// if err != nil {
	// 	panic(err)
	// }

	// ctx, cancel := context.WithTimeout(context.Background(), time.Second*120)
	// defer cancel()

	// dc.Build(ctx, cli, &project)
	// dc.CreateContainers(ctx, &project, cli)
	// // dc.StartAllServiceContainer(ctx, cli, &project)
	// gitClient := github.NewClient(nil)
	// // ctx := context.Background()

	// supervisor := supervisor.CreateRosSupervisor(ctx, gitClient, "ros-supervisor.yml", &project)
	// supervisor.DisplayProject()

	// data, err := yaml.Marshal(&supervisor.SupervisorServices)
	// ioutil.WriteFile("words.yaml", data, 0)
}
