package supervisor

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/dkhoanguyen/ros-supervisor/internal/env"
	"github.com/dkhoanguyen/ros-supervisor/internal/resolvable"
	"github.com/dkhoanguyen/ros-supervisor/internal/utils"
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
	ORCHESTRATOR string = "orchestrator"
	PERFORMER    string = "performer"
)

type RosSupervisor struct {
	DockerCli          *client.Client
	GitCli             *gh.Client
	ProjectCtx         ProjectContext
	DockerProject      *docker.DockerProject
	SupervisorServices []Service
	ProjectPath        string
	MonitorTimeout     time.Duration
	ConfigFile         []byte
	Env                string
	Role               string
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
	composeFile := envConfig.SupervisorComposeFile
	configFile := envConfig.SupervisorConfigFile
	hostmachineName := envConfig.HostMachineName

	rawData, err := utils.ReadYaml(configFile)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to read yaml file %s due to error: %s", configFile, err))
	}

	// If use_git_context then get the latest commit and use it as the build context
	sp.ProjectCtx = MakeProjectCtx(rawData, logger)
	projectPath := sp.ProjectCtx.PrepareContextFromGit(projectDir, logger)
	sp.ProjectPath = projectPath
	sp.DockerProject = docker.MakeDockerProject(composeFile, projectPath, hostmachineName, envConfig.DevEnv, logger)
	sp.Env = envConfig.DevEnv

	sp.SupervisorServices = MakeServices(rawData, sp.DockerProject, *ctx, sp.GitCli, logger)
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

	localCtx, cancel := context.WithCancel(*ctx)
	defer cancel()
	_, err := os.Stat("/supervisor/supervisor_services.yml")
	if err != nil || cmd.UpdateServices || cmd.UpdateCore {
		// File does not exist
		// Check to see if there is any container with the given name is running
		// If yes then stop and remove all of them to rebuild the project
		// In the future, for ROS integration we should keep core running, as it is
		// rarely changed
		allRunningCntInfo, err := docker.ListAllContainers(localCtx, sp.DockerCli, logger)
		allRuningCnt := docker.MakeContainersFromInfo(allRunningCntInfo)
		// TODO: Handles error here
		if err != nil {

		}

		for _, cnt := range allRuningCnt {
			// If service only then stop them
			if strings.Contains(cnt.Name, sp.DockerProject.Name) && !strings.Contains(cnt.Name, "core") {
				logger.Info(fmt.Sprintf("Stopping service %s", cnt.Name))
				cnt.Stop(localCtx, sp.DockerCli, logger)
				cnt.Remove(localCtx, sp.DockerCli, logger)
			} else {
				// Only stop core if the request flag is true
				if strings.Contains(cnt.Name, "core") && cmd.UpdateCore {
					logger.Info(fmt.Sprintf("Stopping service %s", cnt.Name))
					cnt.Stop(localCtx, sp.DockerCli, logger)
					cnt.Remove(localCtx, sp.DockerCli, logger)
				}
			}
		}
		if _, err = os.Stat("/supervisor/supervisor_services.yml"); err != nil {
			// If this is the first run - build all services including core
			logger.Info("Building core and services")
			sp.DockerProject.BuildProjectImages(localCtx, sp.DockerCli, false, logger)
			sp.DockerProject.CreateProjectContainers(localCtx, sp.DockerCli, false, sp.Env, logger)
			sp.DockerProject.StartProjectContainers(localCtx, sp.DockerCli, false, logger)
		} else {
			// If we receive update request from user, build and update services based on the received request
			// Note only build if valid cmd is received
			if cmd.UpdateCore {
				logger.Info("Building core and services")
				sp.DockerProject.BuildProjectImages(localCtx, sp.DockerCli, false, logger)
				sp.DockerProject.CreateProjectContainers(localCtx, sp.DockerCli, false, sp.Env, logger)
				sp.DockerProject.StartProjectContainers(localCtx, sp.DockerCli, false, logger)
			} else if cmd.UpdateServices {
				logger.Info("Building services")
				sp.DockerProject.BuildProjectImages(localCtx, sp.DockerCli, true, logger)
				sp.DockerProject.CreateProjectContainers(localCtx, sp.DockerCli, true, sp.Env, logger)
				sp.DockerProject.StartProjectContainers(localCtx, sp.DockerCli, true, logger)
			}
		}
		// Update supervisor
		// TODO: Write to db
		sp.AttachContainers()
		data, _ := yaml.Marshal(sp.SupervisorServices)
		ioutil.WriteFile("/supervisor/supervisor_services.yml", data, 0777)

		// Reset update flag
		cmd.UpdateCore = false
		cmd.UpdateServices = false

	} else {
		// File exist or no update request receives -> Start the process normally
		// Extract existing info
		logger.Info("Extracting running services")
		allRunningCntInfo, err := docker.ListAllContainers(localCtx, sp.DockerCli, logger)
		allRuningCnt := docker.MakeContainersFromInfo(allRunningCntInfo)
		// TODO: Handle errors
		if err != nil {

		}

		for _, cnt := range allRuningCnt {
			for idx, service := range sp.DockerProject.Services {
				if sp.DockerProject.Name+"_"+service.Name == cnt.Name {
					sp.DockerProject.Services[idx].Container = cnt
				}
			}
		}
		allImagesInfo, err := docker.ListAllImages(localCtx, sp.DockerCli, logger)
		allImages := docker.MakeImagesFromInfo(allImagesInfo)
		// TODO: Handle errors
		if err != nil {

		}

		for _, img := range allImages {
			for idx, service := range sp.DockerProject.Services {
				name := sp.DockerProject.Name + "_" + service.Name
				if img.Name == name {
					sp.DockerProject.Services[idx].Image = img
				}
			}
		}
		serviceData := make([]Service, len(sp.SupervisorServices))
		yfile, _ := ioutil.ReadFile("/supervisor/supervisor_services.yml")
		yaml.Unmarshal(yfile, &serviceData)
		sp.SupervisorServices = serviceData
	}

	for _, srv := range sp.SupervisorServices {
		srv.AttachDockerService(sp.DockerProject)
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
				cnt := sp.SupervisorServices[idx].DockerService.Container
				err := cnt.Stop(localCtx, sp.DockerCli, logger)
				if err != nil {

				}

				err = cnt.Remove(localCtx, sp.DockerCli, logger)
				name := sp.DockerProject.Name + "_" + sp.SupervisorServices[idx].DockerService.Name
				img := docker.MakeImage(name, "latest")
				err = img.Build(localCtx, sp.DockerCli, sp.SupervisorServices[idx].DockerService, logger)
				if err != nil {
					// TODO: Resolve error here
				}
				sp.SupervisorServices[idx].DockerService.Image = img

				cnt = docker.MakeContainer(name)
				net := sp.DockerProject.Networks[0]
				err = cnt.Create(localCtx, sp.DockerCli, sp.SupervisorServices[idx].DockerService, &net, sp.Env, logger)
				if err != nil {
					// TODO: Resolve error here
				}
				err = cnt.Start(localCtx, sp.DockerCli, logger)
				if err != nil {
					// TODO: Resolve error here
				}

				for repoIdx := range sp.SupervisorServices[idx].Repos {
					_, err := sp.SupervisorServices[idx].Repos[repoIdx].GetUpstreamCommitUrl(localCtx, sp.GitCli, "", logger)
					if err != nil {

					}
				}
			}
		}
		if triggerUpdate {
			data, _ := yaml.Marshal(&sp.SupervisorServices)
			ioutil.WriteFile("supervisor_services.yml", data, 0777)
		} else {
			logger.Info("Update is not ready.")

			if cmd.UpdateCore || cmd.UpdateServices {
				break
			}
		}
		// Check once every 5s
		time.Sleep(10 * time.Second)
	}
}

func (sp *RosSupervisor) AttachContainers() {
	for idx := range sp.SupervisorServices {
		for _, service := range sp.DockerProject.Services {
			if sp.SupervisorServices[idx].Name == service.Name {
				// sp.SupervisorServices[idx].ContainerName = service.Container.Name
				// sp.SupervisorServices[idx].ContainerID = service.Container.ID
			}
		}
	}
}

func (sp *RosSupervisor) DisplayProject() {
	fmt.Printf("DOCKER PROJECT \n")
	sp.DockerProject.DisplayProject()
	fmt.Printf("SUPERVISOR CONFIG \n")
	for _, service := range sp.SupervisorServices {
		// fmt.Printf("Service Name: %s\n", service.ServiceName)
		// fmt.Printf("Container Name: %s\n", service.ContainerName)
		for _, repo := range service.Repos {
			fmt.Printf("Repo name: %s\n", repo.Name)
			fmt.Printf("Owner name: %s\n", repo.Owner)
			fmt.Printf("URL: %s\n", repo.Url)
			fmt.Printf("Branch name: %s\n", repo.Branch)
			fmt.Printf("Local Commit: %s\n", repo.CurrentCommit)
		}
		fmt.Printf("===== \n")
	}
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
