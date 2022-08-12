package supervisor

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"time"
	"unsafe"

	"github.com/dkhoanguyen/ros-supervisor/internal/env"
	"github.com/dkhoanguyen/ros-supervisor/internal/resolvable"
	"github.com/dkhoanguyen/ros-supervisor/internal/utils"
	"github.com/dkhoanguyen/ros-supervisor/pkg/db"
	"github.com/dkhoanguyen/ros-supervisor/pkg/docker"
	"github.com/dkhoanguyen/ros-supervisor/pkg/handlers/health"
	handler "github.com/dkhoanguyen/ros-supervisor/pkg/handlers/v1/supervisor"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
	gh "github.com/google/go-github/github"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"
)

const (
	ORCHESTRATOR string = "orchestrator" // Master
	PERFORMER    string = "performer"    // Slave
)

type RosSupervisor struct {
	DockerCli  *client.Client
	GitCli     *gh.Client
	ProjectCtx ProjectContext
	Db         *db.Database

	DockerProject      *docker.DockerProject
	SupervisorServices []Service

	Producers    []*Service
	Distributors []*Service
	Consumers    []*Service

	ProjectPath    string
	MonitorTimeout time.Duration
	ConfigFile     []byte
	Env            string
	Role           string
}

type SupervisorCommand struct {
	Update bool `json:"update"`
}

func (sp *RosSupervisor) Run(
	ctx *context.Context,
	envConfig *env.Config,
	logger *zap.Logger) {
	// We need to redesign this handler
	cmd := handler.SupervisorCommand{
		UpdateCore:     false,
		UpdateServices: false,
	}
	router := gin.Default()
	router.SetTrustedProxies(nil)

	router.GET("/health/liveness", health.LivenessGet)
	router.POST("/cmd", handler.MakeCommand(*ctx, &cmd))
	go router.Run("172.21.0.2:8080")

	for {
		// If the supervisor_services does not exist, wait for update signal (which should only trigger once the file is received and placed appropriately)
		if _, err := os.Stat("/supervisor/project/"); err != nil {
			logger.Info("/supervisor/project/ dir does not exist. Creating one now ...")
			err := os.Mkdir("/supervisor/project/", os.ModePerm)
			if err != nil {
				logger.Fatal(fmt.Sprintf("%s", err))
			}
			continue
		}

		// Maybe it's better to use json data instead of this - ie send compose over http POST in json format
		if utils.FileExists("/supervisor/project/docker-compose.yml") &&
			utils.FileExists("/supervisor/project/ros-supervisor.yml") {
			sp.ReadDockerProject(ctx, envConfig, logger)
			sp.UpdateDockerProject(ctx, &cmd, logger)
			sp.Supervise(ctx, &cmd, logger)
		} else {
			time.Sleep(2 * time.Second)
		}
	}
}

func (sp *RosSupervisor) ReadDockerProject(
	ctx *context.Context,
	envConfig *env.Config,
	logger *zap.Logger) {

	projectDir := envConfig.SupervisorProjectPath
	composeFilePath := envConfig.SupervisorComposeFile
	configFilePath := envConfig.SupervisorConfigFile
	hostmachineName := envConfig.HostMachineName

	fmt.Println(envConfig.DBPath)
	dtb := db.MakeDatabase(envConfig.DBPath)
	sp.Db = &dtb

	// If use_git_context then get the latest commit and use it as the build context
	sp.ProjectCtx = MakeProjectCtx(configFilePath, logger)
	sp.ProjectPath = sp.ProjectCtx.PrepareContextFromGit(projectDir, logger)

	sp.DockerProject = docker.MakeDockerProject(composeFilePath, sp.ProjectPath, hostmachineName, envConfig.DevEnv, sp.Db, logger)
	sp.Env = envConfig.DevEnv

	sp.SupervisorServices = MakeServices(configFilePath, sp.DockerProject, *ctx, sp.GitCli, logger)
	sp.OrganiseServices()
}

func (sp *RosSupervisor) UpdateDockerProject(
	ctx *context.Context,
	cmd *handler.SupervisorCommand,
	logger *zap.Logger) {

	// TODO: Query from db
	// TODO: Load data to db and remove the use of files
	// TODO: Important - We should read from database and somehow compare the build context with the
	// original build context from db, ie the current build context and only stop - build - create - run
	// services with changes and updates
	// TODO: Also important - We should have a hierarchy system for the update procedure
	// - User requested update via API call
	// - Update via recently pushed commit to git

	_, err := os.Stat("/supervisor/supervisor_services.yml")
	if err != nil || cmd.UpdateServices || cmd.UpdateCore {
		sp.StopAllRunningServices(ctx, cmd, logger)
		if _, err = os.Stat("/supervisor/supervisor_services.yml"); err != nil {
			// File does not exist
			// Check to see if there is any container with the given name is running
			// If yes then stop and remove all of them to rebuild the project
			// In the future, for ROS integration we should keep core running, as it is
			// rarely changed
			sp.FirstRun(ctx, cmd, logger)
		} else {
			// If we receive update request from user, build and update services based on the received request
			// Note only build if valid cmd is received
			sp.RebuildAndUpdateServices(ctx, cmd, logger)
		}

		// Reset update flag
		cmd.UpdateCore = false
		cmd.UpdateServices = false

		// Write to db
		for _, service := range sp.SupervisorServices {
			serviceRawData, _ := yaml.Marshal(service)
			db.UpdateServiceRawData(service.Name, serviceRawData, sp.Db)
		}

		data, _ := yaml.Marshal(sp.SupervisorServices)
		ioutil.WriteFile("/supervisor/supervisor_services.yml", data, 0777)

	} else {
		// File exist or no update request receives -> Start the process normally
		// Extract existing info
		// Here supervisor should start each service manaually based on the type and the number of
		// dependencies
		sp.NormalStart(ctx, cmd, logger)

		// Extract from db
		for idx, service := range sp.DockerProject.Services {
			queriedService, err := db.GetServiceByNameAndVersion(service.Name, "1.0.0", sp.Db)
			if err != nil {
				logger.Error(fmt.Sprintf("Unable to retrieve service %s", service.Name))
			}
			yaml.Unmarshal(queriedService.ProcessedRawData, &sp.SupervisorServices[idx])
		}
	}

	for idx := range sp.SupervisorServices {
		sp.SupervisorServices[idx].AttachDockerService(sp.DockerProject)
	}
	fmt.Println(unsafe.Sizeof(sp))
}

func (sp *RosSupervisor) FirstRun(
	ctx *context.Context,
	cmd *handler.SupervisorCommand,
	logger *zap.Logger) {
	// Fresh start when there is no project currently running
	localCtx, cancel := context.WithCancel(*ctx)
	defer cancel()

	// If this is the first run - build all services including core
	logger.Info("Building core and services")
	sp.DockerProject.BuildProjectImages(localCtx, sp.DockerCli, false, logger)
	sp.DockerProject.CreateProjectContainers(localCtx, sp.DockerCli, false, sp.Env, logger)

	// Start Core first
	err := sp.DockerProject.Core.Container.Start(localCtx, sp.DockerCli, logger)
	if err != nil {
		// TODO: Resolve error here
	}

	// Then start by dependencies
	sp.StartServicesByType(localCtx, logger)

	// Once done, update database with imageID, Container ID
	for _, service := range sp.SupervisorServices {
		err := db.UpdateServiceContainerID(service.Name, service.DockerService.Container.ID, sp.Db)
		if err != nil {

		}
		err = db.UpdateServiceImageID(service.Name, service.DockerService.Image.ID, sp.Db)
		if err != nil {

		}
	}
}

func (sp *RosSupervisor) HandleUserCmd(
	ctx *context.Context,
	cmd *handler.SupervisorCommand,
	logger *zap.Logger) {

}

func (sp *RosSupervisor) NormalStart(
	ctx *context.Context,
	cmd *handler.SupervisorCommand,
	logger *zap.Logger) {

	sp.UpdateServicesImageContainerID(ctx, cmd, logger)
}

func (sp *RosSupervisor) StopAllRunningServices(
	ctx *context.Context,
	cmd *handler.SupervisorCommand,
	logger *zap.Logger) {
	localCtx, cancel := context.WithCancel(*ctx)
	defer cancel()

	sp.StopRunningCntOfType(localCtx, CONSUMER, logger)
	sp.StopRunningCntOfType(localCtx, DISTRIBUTOR, logger)
	sp.StopRunningCntOfType(localCtx, PRODUCER, logger)

	allRunningCntInfo, err := docker.ListAllContainers(localCtx, sp.DockerCli, logger)
	if err != nil {
		logger.Error("Unable to list containers")
	}
	allRuningCnt := docker.MakeContainersFromInfo(allRunningCntInfo)

	for _, cnt := range allRuningCnt {
		if strings.Contains(cnt.Name, "core") && cmd.UpdateCore {
			logger.Info(fmt.Sprintf("Stopping service %s", cnt.Name))
			cnt.Stop(localCtx, sp.DockerCli, logger)
			cnt.Remove(localCtx, sp.DockerCli, logger)
			break
		}
	}
}

func (sp *RosSupervisor) UpdateServicesImageContainerID(
	ctx *context.Context,
	cmd *handler.SupervisorCommand,
	logger *zap.Logger) {

	localCtx, cancel := context.WithCancel(*ctx)
	defer cancel()

	// Get all containers
	allCntInfo, err := docker.ListAllContainers(localCtx, sp.DockerCli, logger)
	allCnt := docker.MakeContainersFromInfo(allCntInfo)
	// TODO: Handle errors
	if err != nil {

	}

	// Update services with existing container IDs
	for _, cnt := range allCnt {
		for idx, service := range sp.DockerProject.Services {
			if sp.DockerProject.Name+"_"+service.Name == cnt.Name {
				sp.DockerProject.Services[idx].Container = cnt
			}
		}
	}

	// Get all images
	allImagesInfo, err := docker.ListAllImages(localCtx, sp.DockerCli, logger)
	allImages := docker.MakeImagesFromInfo(allImagesInfo)
	// TODO: Handle errors
	if err != nil {

	}

	// Update services with existing image IDs
	for _, img := range allImages {
		for idx, service := range sp.DockerProject.Services {
			name := sp.DockerProject.Name + "_" + service.Name
			if img.Name == name {
				sp.DockerProject.Services[idx].Image = img
			}
		}
	}

	sp.StartServicesByType(localCtx, logger)
}

func (sp *RosSupervisor) RebuildAndUpdateServices(
	ctx *context.Context,
	cmd *handler.SupervisorCommand,
	logger *zap.Logger) {
	localCtx, cancel := context.WithCancel(*ctx)
	defer cancel()
	if cmd.UpdateCore {
		logger.Info("Building core and services")
		sp.DockerProject.BuildProjectImages(localCtx, sp.DockerCli, false, logger)
		sp.DockerProject.CreateProjectContainers(localCtx, sp.DockerCli, false, sp.Env, logger)

		// Start Core first
		err := sp.DockerProject.Core.Container.Start(localCtx, sp.DockerCli, logger)
		if err != nil {
			// TODO: Resolve error here
		}

		// Then start by dependencies
		sp.StartServicesByType(localCtx, logger)

	} else if cmd.UpdateServices {
		logger.Info("Building services")
		sp.DockerProject.BuildProjectImages(localCtx, sp.DockerCli, true, logger)
		sp.DockerProject.CreateProjectContainers(localCtx, sp.DockerCli, true, sp.Env, logger)

		// Then start by dependencies
		sp.StartServicesByType(localCtx, logger)
	}

	// Once done, update database with imageID, Container ID
	for _, service := range sp.SupervisorServices {
		err := db.UpdateServiceContainerID(service.Name, service.DockerService.Container.ID, sp.Db)
		if err != nil {

		}
		err = db.UpdateServiceImageID(service.Name, service.DockerService.Image.ID, sp.Db)
		if err != nil {

		}
	}
}

func (sp *RosSupervisor) Supervise(
	ctx *context.Context,
	cmd *handler.SupervisorCommand,
	logger *zap.Logger) {

	localCtx, cancel := context.WithCancel(*ctx)
	defer cancel()

	// Resolve host - should only be done if the env is not prod
	if sp.Env != utils.PRODUCTION {
		allHosts := make(map[string]resolvable.Host)
		for _, srv := range sp.DockerProject.Services {
			host := resolvable.Host{
				Ip:       "127.0.0.1",
				Hostname: srv.Hostname,
			}
			allHosts[srv.Name] = host
		}
		hostPath := resolvable.HostFile{
			Path: "/tmp/etc/hosts",
		}
		hostPath.PrepareFile()
		hostPath.UpdateHostFile(allHosts)
	}

	for {
		triggerUpdate := false
		// We need a better way of mapping compose services
		// TODO: Remove the trigger update flag
		// Monitor each service, if an update is available, only update that image and keep
		// other intact
		for idx := range sp.SupervisorServices {
			if sp.SupervisorServices[idx].IsUpdateReady(localCtx, sp.GitCli, logger) {
				logger.Info(fmt.Sprintf("Update for service %s is ready", sp.SupervisorServices[idx].Name))
				triggerUpdate = true
				sp.UpdateService(localCtx, &sp.SupervisorServices[idx], logger)
			}
			if triggerUpdate {
				data, _ := yaml.Marshal(&sp.SupervisorServices)
				ioutil.WriteFile("supervisor_services.yml", data, 0777)
			} else {
				if cmd.UpdateCore || cmd.UpdateServices {
					logger.Info("Update signal received")
					return
				}
			}
			// Check once every 5s
		}
		if !triggerUpdate {
			logger.Info("Update is not ready.")
		}
		time.Sleep(10 * time.Second)
	}
}

func (sp *RosSupervisor) UpdateService(ctx context.Context, service *Service, logger *zap.Logger) {
	cnt := service.DockerService.Container
	err := cnt.Stop(ctx, sp.DockerCli, logger)
	if err != nil {

	}

	err = cnt.Remove(ctx, sp.DockerCli, logger)
	name := sp.DockerProject.Name + "_" + service.DockerService.Name
	img := docker.MakeImage(name, "latest")
	err = img.Build(ctx, sp.DockerCli, service.DockerService, logger)
	if err != nil {
		// TODO: Resolve error here
	}
	service.DockerService.Image = img

	cnt = docker.MakeContainer(name)
	net := sp.DockerProject.Networks[0]
	err = cnt.Create(ctx, sp.DockerCli, service.DockerService, &net, sp.Env, logger)
	if err != nil {
		// TODO: Resolve error here
	}
	err = cnt.Start(ctx, sp.DockerCli, logger)
	if err != nil {
		// TODO: Resolve error here
	}

	for repoIdx := range service.Repos {
		_, err := service.Repos[repoIdx].GetUpstreamCommitUrl(ctx, sp.GitCli, "", logger)
		if err != nil {

		}
	}
}

func (sp *RosSupervisor) StartServicesByType(ctx context.Context, logger *zap.Logger) {
	// Start producers
	for idx := range sp.Producers {
		if sp.Producers[idx].DockerService.Restart == "always" ||
			sp.Producers[idx].DockerService.Restart == "unless-stopped" {
			sp.Producers[idx].DockerService.Container.Start(ctx, sp.DockerCli, logger)
		}

	}

	// TODO: Wait for all producers to start
	// TODO: Either handle the start time using a timeout parameter in the config file
	// or introduce a custom api for the service to notify the supervisor when it thinks
	// that it is running stably

	// Start distributor
	for idx := range sp.Distributors {
		if sp.Distributors[idx].DockerService.Restart == "always" ||
			sp.Distributors[idx].DockerService.Restart == "unless-stopped" {
			sp.Distributors[idx].DockerService.Container.Start(ctx, sp.DockerCli, logger)
		}
	}

	// Same wait as mentioned above

	// Start consumer
	for idx := range sp.Consumers {
		if sp.Consumers[idx].DockerService.Restart == "always" ||
			sp.Consumers[idx].DockerService.Restart == "unless-stopped" {
			sp.Consumers[idx].DockerService.Container.Start(ctx, sp.DockerCli, logger)
		}
	}
}

func (sp *RosSupervisor) StopServicesByType(ctx context.Context, logger *zap.Logger) {

	// Stop consumers first
	for idx := range sp.Producers {
		if sp.Producers[idx].DockerService.Restart == "always" ||
			sp.Producers[idx].DockerService.Restart == "unless-stopped" {
			sp.Producers[idx].DockerService.Container.Stop(ctx, sp.DockerCli, logger)
		}

	}
}

func (sp *RosSupervisor) StopRunningCntOfType(ctx context.Context, srvType string, logger *zap.Logger) {
	allRunningCntInfo, err := docker.ListAllContainers(ctx, sp.DockerCli, logger)
	allRuningCnt := docker.MakeContainersFromInfo(allRunningCntInfo)
	// TODO: Handles error here
	if err != nil {

	}
	// We stop all consumers first
	for _, cnt := range allRuningCnt {
		// This is a bit inefficient, but once we have a working local db
		// this should be resolved
		info, _ := cnt.Inspect(ctx, sp.DockerCli, logger)
		allEnv := info.Config.Env
		for _, env := range allEnv {
			if strings.Contains(env, srvType) {
				logger.Info(fmt.Sprintf("Stopping service %s", cnt.Name))
				cnt.Stop(ctx, sp.DockerCli, logger)
				cnt.Remove(ctx, sp.DockerCli, logger)
				// We move on to the next container
				break
			}
		}
	}

}

func (sp *RosSupervisor) DisplayProject() {
	// fmt.Printf("DOCKER PROJECT \n")
	// sp.DockerProject.DisplayProject()
	fmt.Printf("SUPERVISOR CONFIG \n")
	fmt.Printf("PRODUCER\n")
	for idx := range sp.Producers {
		sp.Producers[idx].Print()
		fmt.Printf("==================== \n")
	}
	fmt.Printf("DISTRIBUTOR\n")
	for idx := range sp.Distributors {
		sp.Distributors[idx].Print()
		fmt.Printf("==================== \n")
	}
	fmt.Printf("CONSUMER\n")
	for idx := range sp.Consumers {
		sp.Consumers[idx].Print()
		fmt.Printf("==================== \n")
	}
}

func (sp *RosSupervisor) OrganiseServices() {

	// Categorise by type
	// Producers
	sp.Producers = make([]*Service, 0)
	for idx, service := range sp.SupervisorServices {
		if service.Type == PRODUCER {
			envType := "SERVICE_TYPE=" + PRODUCER
			sp.SupervisorServices[idx].DockerService.Environment = append(
				sp.SupervisorServices[idx].DockerService.Environment, envType)
			sp.Producers = append(sp.Producers, &sp.SupervisorServices[idx])
		}
	}
	sp.Producers = OrganiseByDependencies(sp.Producers)

	// Distributors
	sp.Distributors = make([]*Service, 0)
	for idx, service := range sp.SupervisorServices {
		if service.Type == DISTRIBUTOR {
			envType := "SERVICE_TYPE=" + DISTRIBUTOR
			sp.SupervisorServices[idx].DockerService.Environment = append(
				sp.SupervisorServices[idx].DockerService.Environment, envType)
			sp.Distributors = append(sp.Distributors, &sp.SupervisorServices[idx])
		}
	}
	sp.Distributors = OrganiseByDependencies(sp.Distributors)

	// Consumers
	sp.Consumers = make([]*Service, 0)
	for idx, service := range sp.SupervisorServices {
		if service.Type == CONSUMER {
			envType := "SERVICE_TYPE=" + CONSUMER
			sp.SupervisorServices[idx].DockerService.Environment = append(
				sp.SupervisorServices[idx].DockerService.Environment, envType)
			sp.Consumers = append(sp.Consumers, &sp.SupervisorServices[idx])
		}
	}
	sp.Consumers = OrganiseByDependencies(sp.Consumers)
}

func OrganiseByDependencies(services []*Service) []*Service {
	output := make([]*Service, 0)
	numDepends := make([]int, 0)
	numDepends = append(numDepends, 0)

	find := func(element int, arr []int) bool {
		for _, d := range arr {
			if element == d {
				return true
			}
		}
		return false
	}

	for idx := range services {
		if !find(len(services[idx].DependsOn), numDepends) {
			numDepends = append(numDepends, len(services[idx].DependsOn))
		}
	}

	sort.Ints(numDepends)

	for _, nd := range numDepends {
		for idx := range services {
			if len(services[idx].DependsOn) == nd {
				output = append(output, services[idx])
			}
		}
	}

	return output
}

func MakeRosSupervisor(
	ctx context.Context,
	envConfig *env.Config,
	logger *zap.Logger) RosSupervisor {

	gitAccessToken := envConfig.GitAccessToken
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: gitAccessToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	// Github client
	gitCli := gh.NewClient(tc)

	// We should verify github client here first
	// Docker client
	dockerCli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		logger.Error(fmt.Sprintf("Unable to create docker client: %v", err))
	}

	supervisor := RosSupervisor{
		DockerCli: dockerCli,
		GitCli:    gitCli,
	}

	return supervisor
}
