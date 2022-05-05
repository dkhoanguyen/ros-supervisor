package docker

import "github.com/docker/docker/api/types/network"

type Networks []Network

type Network struct {
	Name           string `json:"name"`
	ID             string
	CheckDuplicate bool
	Labels         Labels
	Internal       bool
	Attachable     bool
	Driver         string `json:"driver"`
	Ipam           network.IPAM
	EnableIPv6     bool
}

// IPAMConfig represents IPAM configuration
func MakeNetwork() {

}
