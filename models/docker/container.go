package docker

import "github.com/docker/docker/api/types/container"

type ContainerConfig struct {
	Name            string `json:"name"`
	Hostname        string `json:"hostname"`
	Domainname      string
	User            string
	AttachStdin     bool
	AttachStderr    bool
	AttachStdout    bool
	ExposedPort     map[string]struct{}
	Tty             bool
	OpenStdIn       bool
	StdinOnce       bool
	Environment     []string
	Cmd             ShellCommand
	Image           string
	WorkingDir      string
	EntryPoint      ShellCommand
	NetworkDisabled bool
	MacAddress      bool
	Labels          Labels
	HealthCheck     HeathCheckConfig
	StopSignal      string
	Volumes         map[string]struct{}
	StopTimeout     int
}

type HeathCheckConfig struct {
}

type HostConfig struct {
	AutoRemove      bool
	Binds           []string
	Devices         []container.DeviceMapping
	Resources       container.Resources
	VolumeDriver    string
	VolumesFrom     []string
	CapAdd          []string
	CapDrop         []string
	ExtraHosts      []string
	GroupAdd        []string
	IpcMode         string
	Cgroup          string
	Links           []string
	OomScoreAdj     int
	PidMode         string
	Privileged      bool
	PublishAllPorts bool
	ReadonlyRootfs  bool
	SecurityOpt     []string
	StorageOpt      map[string]string
	Tmpfs           map[string]string
	Isolation       string
	UTSMode         string
	UsernsMode      string
	ShmSize         int64
	Sysctls         map[string]string
	Runtime         string
	LogConfig       container.LogConfig
}
