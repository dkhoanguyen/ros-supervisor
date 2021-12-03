package docker

type Image struct {
	ID         string
	Repository string
	Tag        string
	Created    string
}

type ImageContainerConfig struct {
	HostName   string `json:"hostName"`
	DomainName string `json:"domainName"`
}
