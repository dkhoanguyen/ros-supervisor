package cmd

import (
	"context"

	"github.com/docker/docker/client"
	"go.uber.org/zap"
)

type DockerComponent interface {
	Build(ctx context.Context, dkCli *client.Client, logger *zap.Logger) error
}
