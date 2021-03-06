package docker

type Services []Service

type Service struct {
	Name           string
	Hostname       string
	User           string
	CapAdd         []string
	CapDrop        []string
	BuildOpt       ServiceBuild
	CgroupParent   string
	Command        ShellCommand
	ContainerName  string
	Domainname     string
	DependsOn      []string
	Devices        []string
	EntryPoint     ShellCommand
	Environment    []string
	EnvFile        []string
	Expose         []string
	Image          Image
	Container      Container
	IpcMode        string
	MemLimit       int64
	MemSwapLimit   int64
	Networks       []ServiceNetwork
	NetworkMode    string
	OomKillDisable bool
	Ports          []ServicePort
	Privileged     bool
	Sysctls        map[string]string
	Restart        string
	Tty            bool
	Volumes        []ServiceVolume
	WorkingDir     string
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
	Type        string
	Source      string
	Destination string
	Option      string
}

type ServicePort struct {
	Target   string
	Protocol string
	HostIp   string
	HostPort string
}

const (
	VolumeTypeBind  = "bind"
	VolumeTypeMount = "mount"
)

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
