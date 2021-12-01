package docker

type Image struct {
	ID      string `json:"id"`
	RepoTag map[string]string
	Parent  string
	Created string
}

type ImageContainerConfig struct {
	HostName   string `json:"hostName"`
	DomainName string `json:"domainName"`
}
