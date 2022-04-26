package utils

import (
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

func ReadYaml(path string) (map[interface{}]interface{}, error) {
	configFile, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	rawData := make(map[interface{}]interface{})
	err = yaml.Unmarshal(configFile, &rawData)
	if err != nil {
		return nil, err
	}
	return rawData, nil
}
