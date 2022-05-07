package supervisor

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/dkhoanguyen/ros-supervisor/internal/env"
	"github.com/dkhoanguyen/ros-supervisor/internal/utils"
	"github.com/dkhoanguyen/ros-supervisor/pkg/compose"
	"github.com/dkhoanguyen/ros-supervisor/pkg/handlers/health"
	handler "github.com/dkhoanguyen/ros-supervisor/pkg/handlers/v1/supervisor"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
	gh "github.com/google/go-github/github"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"
)

type RosSupervisor struct {
	DockerCli          *client.Client
	GitCli             *gh.Client
	ProjectCtx         ProjectContext
	DockerProject      *compose.DockerProject
	SupervisorServices SupervisorServices
	ProjectPath        string
	MonitorTimeout     time.Duration
	ConfigFile         []byte
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

	router.GET("/health/liveness", health.LivenessGet)
	router.POST("/cmd", handler.MakeCommand(*ctx, &cmd))
	go router.Run("172.21.0.2:8080")

	for {
		// If the supervisor_services does not exist, wait for update signal (which should only trigger once the file is received and placed appropriately)
		if _, err := os.Stat("/supervisor/project/"); err != nil {
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

	rawData, err := utils.ReadYaml(configFile)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to read yaml file %s due to error: %s", configFile, err))
	}

	sp.ProjectCtx = MakeProjectCtx(rawData, logger)
	sp.SupervisorServices = MakeServices(rawData, *ctx, sp.GitCli, logger)

	// If use_git_context then get the latest commit and use it as the build context
	projectPath := sp.ProjectCtx.PrepareContextFromGit(projectDir, logger)
	sp.ProjectPath = projectPath
	sp.DockerProject = compose.MakeDockerProject(composeFile, projectPath, logger)
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
		allRunningContainers, err := compose.ListAllContainers(localCtx, sp.DockerCli, logger)
		// TODO: Handles error here
		if err != nil {

		}

		for _, cnt := range allRunningContainers {
			// If service only then stop them
			if strings.Contains(cnt.Names[0], sp.DockerProject.Name) && !strings.Contains(cnt.Names[0], "core") {
				logger.Info(fmt.Sprintf("Stopping service %s", cnt.Names[0]))
				ID := cnt.ID
				compose.StopServiceByID(localCtx, sp.DockerCli, ID, logger)
				compose.RemoveServiceByID(localCtx, sp.DockerCli, ID, logger)
			} else {
				// Only stop core if the request flag is true
				if strings.Contains(cnt.Names[0], "core") && cmd.UpdateCore {
					logger.Info(fmt.Sprintf("Stopping service %s", cnt.Names[0]))
					ID := cnt.ID
					compose.StopServiceByID(localCtx, sp.DockerCli, ID, logger)
					compose.RemoveServiceByID(localCtx, sp.DockerCli, ID, logger)
				}
			}
		}
		if _, err = os.Stat("/supervisor/supervisor_services.yml"); err != nil {
			// If this is the first run - build all services including core
			logger.Info("Building core and services")
			compose.BuildAll(localCtx, sp.DockerCli, sp.DockerProject, logger)
			compose.CreateContainers(localCtx, sp.DockerCli, sp.DockerProject, logger)
			compose.StartAll(localCtx, sp.DockerCli, sp.DockerProject, logger)
		} else {
			// If we receive update request from user, build and update services based on the received request
			// Note only build if valid cmd is received
			if cmd.UpdateCore {
				logger.Info("Building core and services")
				compose.BuildAll(localCtx, sp.DockerCli, sp.DockerProject, logger)
				compose.CreateContainers(localCtx, sp.DockerCli, sp.DockerProject, logger)
				compose.StartAll(localCtx, sp.DockerCli, sp.DockerProject, logger)
			} else if cmd.UpdateServices {
				logger.Info("Building services")
				compose.BuildServices(localCtx, sp.DockerCli, sp.DockerProject, logger)
				compose.CreateServiceContainers(localCtx, sp.DockerCli, sp.DockerProject, logger)
				compose.StartServices(localCtx, sp.DockerCli, sp.DockerProject, logger)
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
		allContainers, err := compose.ListAllContainers(localCtx, sp.DockerCli, logger)
		// TODO: Handle errors
		if err != nil {

		}

		for _, cnt := range allContainers {
			for idx, service := range sp.DockerProject.Services {
				if sp.DockerProject.Name+"_"+service.Name == cnt.Names[0][1:] {
					sp.DockerProject.Services[idx].Container.ID = cnt.ID
				}
			}
		}
		allImages, err := compose.ListAllImages(localCtx, sp.DockerCli, logger)
		// TODO: Handle errors
		if err != nil {

		}

		for _, img := range allImages {
			splitString := strings.Split(img.RepoTags[0], ":")
			imageName := splitString[0]

			for idx, service := range sp.DockerProject.Services {
				name := sp.DockerProject.Name + "_" + service.Name
				if imageName == name {
					sp.DockerProject.Services[idx].Image.ID = img.ID
				}
			}
		}
		serviceData := make([]SupervisorService, len(sp.SupervisorServices))
		yfile, _ := ioutil.ReadFile("/supervisor/supervisor_services.yml")
		yaml.Unmarshal(yfile, &serviceData)
		sp.SupervisorServices = serviceData
	}

}

func (sp *RosSupervisor) Supervise(
	ctx *context.Context,
	cmd *handler.SupervisorCommand,
	logger *zap.Logger) {

	localCtx, cancel := context.WithCancel(*ctx)
	defer cancel()
	for {
		triggerUpdate := false
		for idx := range sp.SupervisorServices {
			for _, repo := range sp.SupervisorServices[idx].Repos {
				upStreamCommit, err := repo.UpdateUpStreamCommit(localCtx, sp.GitCli, logger)
				if err != nil {

				}
				if repo.IsUpdateReady() {
					sp.SupervisorServices[idx].UpdateReady = true
					triggerUpdate = true
					fmt.Printf("Update for service %s is ready. Upstream commit: %s\n", sp.SupervisorServices[idx].ContainerName, upStreamCommit)
				}
			}
		}
		// We need a better way of mapping compose services
		if triggerUpdate {
			logger.Info("Update is ready. Performing updates")
			for idx := range sp.SupervisorServices {
				if sp.SupervisorServices[idx].UpdateReady {
					for srvIdx := range sp.DockerProject.Services {
						if sp.DockerProject.Services[srvIdx].Name == sp.SupervisorServices[idx].ServiceName {
							compose.StopService(localCtx, sp.DockerCli, &sp.DockerProject.Services[srvIdx])
							compose.RemoveService(localCtx, sp.DockerCli, &sp.DockerProject.Services[srvIdx], logger)

							compose.BuildSingle(localCtx, sp.DockerCli, sp.DockerProject.Name, &sp.DockerProject.Services[srvIdx], logger)
							compose.CreateNetwork(localCtx, sp.DockerProject, sp.DockerCli, false, logger)
							compose.CreateSingleContainer(localCtx, sp.DockerProject.Name, &sp.DockerProject.Services[srvIdx], &sp.DockerProject.Networks[0], sp.DockerCli, logger)
							compose.StartSingleServiceContainer(localCtx, sp.DockerCli, &sp.DockerProject.Services[srvIdx], logger)
							sp.SupervisorServices[idx].UpdateReady = false
						}
					}

					for repoIdx := range sp.SupervisorServices[idx].Repos {
						_, err := sp.SupervisorServices[idx].Repos[repoIdx].GetUpstreamCommitUrl(localCtx, sp.GitCli, "", logger)
						if err != nil {

						}
					}
				}
			}

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
			if sp.SupervisorServices[idx].ServiceName == service.Name {
				sp.SupervisorServices[idx].ContainerName = service.Container.Name
				sp.SupervisorServices[idx].ContainerID = service.Container.ID
			}
		}
	}
}

func (sp *RosSupervisor) DisplayProject() {
	fmt.Printf("DOCKER PROJECT \n")
	sp.DockerProject.DisplayProject()
	fmt.Printf("SUPERVISOR CONFIG \n")
	for _, service := range sp.SupervisorServices {
		fmt.Printf("Service Name: %s\n", service.ServiceName)
		fmt.Printf("Container Name: %s\n", service.ContainerName)
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
