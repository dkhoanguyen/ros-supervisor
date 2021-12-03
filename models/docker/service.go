package docker

type Services []Service

type Service struct {
	Name          string
	BuildOpt      ServiceBuild
	CgroupParent  []string
	Command       ShellCommand
	ContainerName string
	DependsOn     []string
	Devices       []string
	EntryPoint    ShellCommand
	Environment   []string
	EnvFile       []string
	Expose        []string
	Image         Image
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
	Args       map[string]*string
}

type ServiceNetwork struct {
	Name    string
	Aliases []string
	IPv4    string
	IPv6    string
}

type ServiceVolume struct {
	Mount string
}

const (
	RestartAlways        = "always"
	RestartOnFailure     = "on-failure"
	RestartNoRetry       = "no"
	RestartUnlessStopped = "unless-stopped"
)

func GetBuildConfig(rawBuildConfig map[string]interface{}) ServiceBuild {
	return ServiceBuild{
		Context:    rawBuildConfig["context"].(string),
		Dockerfile: rawBuildConfig["dockerfile"].(string),
	}
}
