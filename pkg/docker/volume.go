package docker

import "go.uber.org/zap"

type Volumes []Volume

type Volume struct {
	Name string `json:"name"`
}

func MakeVolume(rawData map[interface{}]interface{}, logger *zap.Logger) Volumes {
	logger.Debug("Extracting volumes")

	outputVolumes := Volumes{}
	if rawVolumes, exist := rawData["volumes"].(map[string]interface{}); exist {
		for volumeName := range rawVolumes {
			dVolume := Volume{}
			dVolume.Name = volumeName
			outputVolumes = append(outputVolumes, dVolume)
		}
	}
	return outputVolumes
}
