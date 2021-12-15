package docker

type Volumes []Volume

type Volume struct {
	Name string `json:"name"`
}
