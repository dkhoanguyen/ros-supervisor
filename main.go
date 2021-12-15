package main

import (
	"github.com/dkhoanguyen/ros-supervisor/pkg/supervisor"
)

func main() {
	supervisor.Execute()
	// supervisor.StartSupervisor(ctx, &rs, dockerCli, gitClient)
}
