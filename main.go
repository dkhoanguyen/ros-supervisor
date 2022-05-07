package main

import (
	"context"
	"log"

	"github.com/dkhoanguyen/ros-supervisor/internal/env"
	"github.com/dkhoanguyen/ros-supervisor/internal/logging"
	"github.com/dkhoanguyen/ros-supervisor/pkg/supervisor"
)

func main() {
	// Main should prepare stuff and kickstart processes
	// First start with context
	ctx := context.Background()

	// Then extract env vars for configurations
	envConfig, err := env.LoadConfig(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Build logger
	logger := logging.MakeLogger(envConfig)

	// Make Ros Supervisor
	rs := supervisor.MakeRosSupervisor(ctx, envConfig, logger)

	// Router and handlers
	rs.Run(&ctx, envConfig, logger)
}
