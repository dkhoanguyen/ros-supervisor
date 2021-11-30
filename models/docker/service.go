package docker

type Services []Service

type Service struct {
	Name          string
	BuildOpt      ServiceBuild
	CgroupParent  []string
	Command       ShellCommand
	ContainerName string
	DependsOn     map[string]string
	Devices       []string
	EntryPoint    ShellCommand
	Environment   []string
	EnvFile       []string
	Expose        []string
	Image         string
	MemLimit      int64
	MemSwapLimit  int64
	Networks      []ServiceNetwork
	Ports         []string
	Privileged    bool
	Restart       string
	Volumes       []ServiceVolume
	WorkingDir    string
}

type ServiceBuild struct {
	Context    string
	Dockerfile string
	Arges      map[string]*string
}

type ServiceNetwork struct {
	Name    string
	Aliases []string
	IPv4    string
	IPv6    string
}

type ServiceVolume struct {
	Type string
}

const (
	RestartAlways        = "always"
	RestartOnFailure     = "on-failure"
	RestartNoRetry       = "no"
	RestartUnlessStopped = "unless-stopped"
)
