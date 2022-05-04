package docker

// Interface for docker main components
// Containers
// Images
// Networks
// Volumes
// Config
// Secrets

type DockerComponent interface {
	Build()
	Create()
}
