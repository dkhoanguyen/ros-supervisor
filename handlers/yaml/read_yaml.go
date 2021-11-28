package yaml

import (
	"io/ioutil"
	"log"

	"gopkg.in/yaml.v3"
)

func ReadYaml(path string) map[interface{}]interface{} {
	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	data := make(map[interface{}]interface{})
	err2 := yaml.Unmarshal(yamlFile, &data)
	if err2 != nil {
		log.Fatal(err2)
	}
	return data
}
