package docker

type Networks []Network

type Network struct {
	Name   string `json:"name"`
	Driver string `json:"driver"`
	Ipam   IPAM
}

type IPAM struct {
	Driver string     `json:"driver"`
	Config IPAMConfig `json:"config"`
}

type IPAMConfig struct {
	Subnet  string `json:"subnet"`
	IpRange string `json:"iprange"`
	Gateway string
}
