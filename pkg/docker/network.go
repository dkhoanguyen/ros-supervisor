package docker

import (
	"context"
	"fmt"

	dockerApiTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
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

func (ntwk *Network) Create(
	ctx context.Context,
	dockerCli *client.Client,
	forceReCreate bool,
	logger *zap.Logger) error {

	networkOpts := ntwk.prepareNetworkOptions()
	networkName := ntwk.Name

	info, err := dockerCli.NetworkInspect(ctx, networkName, dockerApiTypes.NetworkInspectOptions{})
	if err != nil {
		resp, err := dockerCli.NetworkCreate(ctx, networkName, networkOpts)
		if err != nil {
			// Error means there could be a network existed already
			logger.Error(fmt.Sprintf("Unable to create network %s with error: %v", networkName, err))
			return err
		}
		ntwk.ID = resp.ID
	} else {
		if forceReCreate {
			// Delete old network setup and recreate them
			// Note that all containers that use the target network must be killed
			err := dockerCli.NetworkRemove(ctx, info.ID)
			if err != nil {
				logger.Error(fmt.Sprintf("Unable to remove network %s with error: %v", networkName, err))
				return err
			}
			logger.Info(fmt.Sprintf("Recreating network %s", networkName))
			resp, err := dockerCli.NetworkCreate(ctx, networkName, networkOpts)
			if err != nil {
				logger.Error(fmt.Sprintf("Unable to create network %s with error: %v", networkName, err))
				return err
			}
			ntwk.ID = resp.ID
		} else {
			// Maybe extract existing values
			networkRes, err := dockerCli.NetworkList(ctx, dockerApiTypes.NetworkListOptions{})
			if err != nil {
				logger.Error(fmt.Sprintf("Unable to list network %s with error: %v", networkName, err))
				return err
			}

			for _, net := range networkRes {
				if net.Name == networkName {
					logger.Info("Extracting Network Info")
					ntwk.ID = net.ID
					return nil
				}
			}
		}
	}
	return nil
}

func (ntwk *Network) prepareNetworkOptions() dockerApiTypes.NetworkCreate {
	return dockerApiTypes.NetworkCreate{
		CheckDuplicate: ntwk.CheckDuplicate,
		Labels:         ntwk.Labels,
		Driver:         ntwk.Driver,
		Internal:       ntwk.Internal,
		Attachable:     ntwk.Attachable,
		IPAM:           &ntwk.Ipam,
		EnableIPv6:     ntwk.EnableIPv6,
	}
}
