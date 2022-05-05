package docker

import (
	"strconv"
	"strings"
)

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

func MakeService(config map[string]interface{}, name string) Service {
	output := Service{}
	output.Name = name
	return output
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
				value := arg.(string)[len(key+"="):]
				formattedArg[key] = &value
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

func MakeDeployResources(config map[string]interface{}) ServiceResources {
	resources := ServiceResources{}
	if deployOpt, ok := config["deploy"].(map[string]interface{}); ok {
		if resourcesOpt, ok := deployOpt["resources"].(map[string]interface{}); ok {
			limitOpt := resourcesOpt["limits"].(map[string]interface{})
			// CPU usage
			var cpuPeriod float64 = 100000                                   // Default value of 100000
			cpuQuota, _ := strconv.ParseFloat(limitOpt["cpus"].(string), 64) // Combination of period and quota to determine cpu limitation
			resources.CPUQuota = int64(cpuQuota * cpuPeriod)
			resources.CPUPeriod = int64(cpuPeriod)

			// Memory usage
			memoryInString := limitOpt["memory"].(string)
			memory, _ := strconv.ParseInt(memoryInString[:len(memoryInString)-1], 10, 64)
			suffix := string(memoryInString[len(memoryInString)-1])
			switch {
			case suffix == "k" || suffix == "K":
				memory = memory * 1024
			case suffix == "m" || suffix == "M":
				memory = memory * 1048576
			case suffix == "g" || suffix == "G":
				memory = memory * 1073741824
			}
			resources.MemoryLimit = memory
		}
	}
	return resources
}

func MakeEnviroment(config map[string]interface{}) []string {
	env := make([]string, 0)
	// Environment variables
	if envVarsOpt, ok := config["environment"].([]interface{}); ok {
		for _, envVars := range envVarsOpt {
			env = append(env, envVars.(string))
		}
	}
	return env
}

func MakeRestartOpt(config map[string]interface{}) string {
	output := ""
	if restartOpt, ok := config["restart"].(string); ok {
		output = restartOpt
	}
	return output
}

func MakePortBinding(config map[string]interface{}) []ServicePort {
	ports := make([]ServicePort, 0)
	if portOpt, ok := config["ports"].([]interface{}); ok {
		for _, portData := range portOpt {
			// We need to properly split the string to port and host ip address
			splittedPort := strings.Split(portData.(string), ":")
			port := ServicePort{
				Target:   splittedPort[0],
				Protocol: "tcp",
				HostIp:   "0.0.0.0",
				HostPort: splittedPort[1],
			}
			ports = append(ports, port)
		}
	}
	return ports
}

func MakePrivileged(config map[string]interface{}) bool {
	if privileged, ok := config["privileged"].(bool); ok {
		return privileged
	}
	return false
}

func MakeTTY(config map[string]interface{}) bool {
	if tty, ok := config["tty"].(bool); ok {
		return tty
	}
	return false
}
