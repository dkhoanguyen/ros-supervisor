package docker

import "strings"

type Services []Service

type Service struct {
	Name          string
	Hostname      string
	User          string
	CapAdd        []string
	CapDrop       []string
	BuildOpt      ServiceBuild
	CgroupParent  string
	Command       ShellCommand
	ContainerName string
	Domainname    string
	DependsOn     []string
	Devices       []string
	EntryPoint    ShellCommand
	Environment   []string
	EnvFile       []string
	Expose        []string
	Image         Image
	Container     Container
	IpcMode       string
	Resources     ServiceResources
	Networks      []ServiceNetwork
	NetworkMode   string
	Ports         []ServicePort
	Privileged    bool
	Sysctls       map[string]string
	Restart       string
	Tty           bool
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

type ServiceResources struct {
	CPUPeriod         int64
	CPUQuota          int64
	CpusetCpus        string
	CpusetMems        string
	MemoryLimit       int64
	MemoryReservation int64
	MemorySwap        int64
	MemorySwappiness  int64
	OomKillDisable    bool
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

func MakeBuildOpt(config map[string]interface{}, path string) ServiceBuild {
	output := ServiceBuild{}
	buildOpt := config["build"].(map[string]interface{})
	output.Context = path
	output.Dockerfile = buildOpt["dockerfile"].(string)

	// Only extract build arg if arg exists
	if buildArgs, exist := buildOpt["args"].([]interface{}); exist {
		formattedArg := make(map[string]*string)
		for _, arg := range buildArgs {
			if _, ok := arg.(string); ok {
				splittedString := strings.Split(arg.(string), "=")
				key := splittedString[0]
				value := arg.(string)[len(key+"=")]
				formattedArg[key] = value
			}
		}
		output.Args = formattedArg
	}

	return output
}

func MakeContainerName(config map[string]interface{}) string {
	return config["container_name"].(string)
}

func MakeDependsOn(config map[string]interface{}) []string {
	output := make([]string, 0)
	if dependsOnOpt, exist := config["depends_on"].([]interface{}); exist {
		for _, dependsOn := range dependsOnOpt {
			output = append(output, dependsOn.(string))
		}
	}
	return output
}
