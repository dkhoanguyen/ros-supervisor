package models

type RosNode struct {
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
	IPAdress string `json:"ip_address"`
	RepoList []MonitorRepository
}

func MakeRosNode(name, hostname, ip_address string, repos []MonitorRepository) *RosNode {
	return &RosNode{
		Name:     name,
		Hostname: hostname,
		IPAdress: ip_address,
		RepoList: repos,
	}
}

func (n *RosNode) GetName() string {
	return n.Name
}

func (n *RosNode) GetHostname() string {
	return n.Hostname
}
