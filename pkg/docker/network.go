package docker

import (
	"github.com/docker/docker/api/types/network"
	"go.uber.org/zap"
)

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
func MakeNetwork(rawData map[interface{}]interface{}, logger *zap.Logger) Networks {
	output := Networks{}
	logger.Debug("Extracting networks")

	if rawNetworks, exist := rawData["networks"].(map[string]interface{}); exist {
		for name, networkData := range rawNetworks {
			nw := Network{}
			nw.Name = name
			nw.CheckDuplicate = true
			nw.EnableIPv6 = false
			nw.Internal = false
			nw.Driver = networkData.(map[string]interface{})["driver"].(string)

			ipam := networkData.(map[string]interface{})["ipam"]
			ipamConfig := ipam.(map[string]interface{})["config"].([]interface{})

			for _, config := range ipamConfig {
				ipamConfig := network.IPAMConfig{
					Subnet:  config.(map[string]interface{})["subnet"].(string),
					Gateway: config.(map[string]interface{})["gateway"].(string),
				}
				nw.Ipam.Config = append(nw.Ipam.Config, ipamConfig)
			}
			output = append(output, nw)
		}
	}
	return output
}
