package docker

import "github.com/docker/docker/api/types"

type Container struct {
	Name   string `json:"name"`
	Config types.Container
}
