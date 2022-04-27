package supervisor

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/dkhoanguyen/ros-supervisor/internal/env"
	"github.com/dkhoanguyen/ros-supervisor/internal/logging"
	"github.com/dkhoanguyen/ros-supervisor/internal/utils"
	"github.com/dkhoanguyen/ros-supervisor/pkg/compose"
	"github.com/dkhoanguyen/ros-supervisor/pkg/handlers/health"
	"github.com/dkhoanguyen/ros-supervisor/pkg/handlers/v1/supervisor"
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
	DockerProject      *compose.Project
	SupervisorServices SupervisorServices
	ProjectDir         string
	MonitorTimeout     time.Duration
	ConfigFile         []byte
}

type SupervisorCommand struct {
	Update bool `json:"update"`
}

func MakeSupervisor(
	ctx context.Context,
	githubClient *gh.Client,
	configPath string,
	projectDir string,
	logger *zap.Logger) (
	RosSupervisor, string) {

	supervisor := RosSupervisor{}
	projectPath := ""
	rawData, err := utils.ReadYaml(configPath)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to read yaml file %s due to error: %s", configPath, err))
	}
	supervisor.ProjectCtx = MakeProject(rawData, logger)
	supervisor.SupervisorServices = MakeServices(rawData, ctx, githubClient, logger)

	// If use_git_context then get the latest commit and use it as the build context
	projectPath = supervisor.ProjectCtx.PrepareContextFromGit(projectDir, logger)
	return supervisor, projectPath
}

func StartProcess(c *gin.Context) {
	var supCommand SupervisorCommand
	if err := c.BindJSON(&supCommand); err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(supCommand)
}

func Execute() {
	ctx := context.Background()

	envConfig, err := env.LoadConfig(ctx)
	if err != nil {
		log.Fatal(err)
	}

	logger := logging.Make(envConfig)

	gitAccessToken := envConfig.GitAccessToken
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: gitAccessToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	gitClient := gh.NewClient(tc)

	// We should verify github client here first

	dockerCli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		logger.Fatal(fmt.Sprintf("%s", err))
	}

	rs := RosSupervisor{
		GitCli:     gitClient,
		DockerCli:  dockerCli,
		ProjectDir: envConfig.SupervisorProjectPath,
	}

	// Router and handlers
	cmd := supervisor.SupervisorCommand{
		UpdateCore:     false,
		UpdateServices: false,
	}
	router := gin.Default()

	router.GET("/health/liveness", health.LivenessGet)
	router.POST("/cmd", supervisor.MakeCommand(ctx, &cmd))
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
			if err != nil {
				logger.Fatal(fmt.Sprintf("%s", err))
			}

			PrepareSupervisor(ctx, &rs, &cmd)
			StartSupervisor(ctx, &rs, dockerCli, gitClient, &cmd, logger)
			time.Sleep(2 * time.Second)
			break

		} else {
			time.Sleep(2 * time.Second)
		}
	}
}

func PrepareSupervisor(ctx context.Context, supervisor *RosSupervisor, cmd *supervisor.SupervisorCommand) RosSupervisor {

	localCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	envConfig, err := env.LoadConfig(localCtx)
	if err != nil {
		log.Fatal(err)
	}

	logger := logging.Make(envConfig)
	projectDir := envConfig.SupervisorProjectPath
	composeFile := envConfig.SupervisorComposeFile
	configFile := envConfig.SupervisorConfigFile

	dockerCli := supervisor.DockerCli
	gitClient := supervisor.GitCli

	rs, projectPath := MakeSupervisor(localCtx, gitClient, configFile, projectDir, logger)

	composeProject := compose.CreateProject(composeFile, projectPath, logger)
	_, err = os.Stat("/supervisor/supervisor_services.yml")

	if err != nil || cmd.UpdateServices || cmd.UpdateCore {
		// File does not exist
		// Check to see if there is any container with the given name is running
		// If yes then stop and remove all of them to rebuild the project
		// In the future, for ROS integration we should keep core running, as it is
		// rarely changed
		allRunningContainers, err := compose.ListAllContainers(localCtx, dockerCli, logger)
		if err != nil {

		}
		for _, cnt := range allRunningContainers {
			// If service only then stop them
			if strings.Contains(cnt.Names[0], composeProject.Name) && !strings.Contains(cnt.Names[0], "core") {
				logger.Info(fmt.Sprintf("Stopping service %s", cnt.Names[0]))
				ID := cnt.ID
				compose.StopServiceByID(ctx, dockerCli, ID, logger)
				compose.RemoveServiceByID(ctx, dockerCli, ID, logger)
			} else {
				// Only stop core if the request flag is true
				if strings.Contains(cnt.Names[0], "core") && cmd.UpdateCore {
					logger.Info(fmt.Sprintf("Stopping service %s", cnt.Names[0]))
					ID := cnt.ID
					compose.StopServiceByID(ctx, dockerCli, ID, logger)
					compose.RemoveServiceByID(ctx, dockerCli, ID, logger)
				}
			}
		}

		if _, err = os.Stat("/supervisor/supervisor_services.yml"); err != nil {
			// If this is the first run - build all services including core
			logger.Info("Building core and services")
			compose.BuildAll(localCtx, dockerCli, &composeProject, logger)
			compose.CreateContainers(localCtx, dockerCli, &composeProject, logger)
			compose.StartAll(localCtx, dockerCli, &composeProject, logger)
		} else {
			// If we receive update request from user, build and update services based on the received request
			// Note only build if valid cmd is received
			if cmd.UpdateCore {
				logger.Info("Building core and services")
				compose.BuildAll(localCtx, dockerCli, &composeProject, logger)
				compose.CreateContainers(localCtx, dockerCli, &composeProject, logger)
				compose.StartAll(localCtx, dockerCli, &composeProject, logger)
			} else if cmd.UpdateServices {
				logger.Info("Building services")
				compose.BuildServices(localCtx, dockerCli, &composeProject, logger)
				compose.CreateServiceContainers(localCtx, dockerCli, &composeProject, logger)
				compose.StartServices(localCtx, dockerCli, &composeProject, logger)
			}
		}

		// Update supervisor
		rs.DockerProject = &composeProject
		rs.AttachContainers()
		data, _ := yaml.Marshal(&rs.SupervisorServices)
		ioutil.WriteFile("/supervisor/supervisor_services.yml", data, 0777)

		// Reset update flag
		cmd.UpdateCore = false
		cmd.UpdateServices = false

	} else {
		// File exist or no update request receives -> Start the process normally
		// Extract existing info
		logger.Info("Extracting running services")
		allContainers, err := compose.ListAllContainers(localCtx, dockerCli, logger)
		if err != nil {

		}

		for _, cnt := range allContainers {
			for idx, service := range composeProject.Services {
				if composeProject.Name+"_"+service.Name == cnt.Names[0][1:] {
					composeProject.Services[idx].Container.ID = cnt.ID
				}
			}
		}
		allImages, err := compose.ListAllImages(localCtx, dockerCli, logger)
		if err != nil {

		}

		for _, img := range allImages {
			splitString := strings.Split(img.RepoTags[0], ":")
			imageName := splitString[0]

			for idx, service := range composeProject.Services {
				name := composeProject.Name + "_" + service.Name
				if imageName == name {
					composeProject.Services[idx].Image.ID = img.ID
				}
			}
		}

		serviceData := make([]SupervisorService, len(rs.SupervisorServices))
		yfile, _ := ioutil.ReadFile("/supervisor/supervisor_services.yml")
		yaml.Unmarshal(yfile, &serviceData)
		rs.SupervisorServices = serviceData
	}
	return rs
}

func StartSupervisor(ctx context.Context, supervisor *RosSupervisor, dockeClient *client.Client, gitClient *gh.Client, cmd *supervisor.SupervisorCommand, logger *zap.Logger) {

	localCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	for {
		triggerUpdate := false
		for idx := range supervisor.SupervisorServices {
			for _, repo := range supervisor.SupervisorServices[idx].Repos {
				upStreamCommit, err := repo.UpdateUpStreamCommit(localCtx, gitClient, logger)
				if err != nil {

				}
				if repo.IsUpdateReady() {
					supervisor.SupervisorServices[idx].UpdateReady = true
					triggerUpdate = true
					fmt.Printf("Update for service %s is ready. Upstream commit: %s\n", supervisor.SupervisorServices[idx].ContainerName, upStreamCommit)
				}
			}
		}
		// We need a better way of mapping compose services
		if triggerUpdate {
			logger.Info("Update is ready. Performing updates")
			for idx := range supervisor.SupervisorServices {
				if supervisor.SupervisorServices[idx].UpdateReady {
					for srvIdx := range supervisor.DockerProject.Services {
						if supervisor.DockerProject.Services[srvIdx].Name == supervisor.SupervisorServices[idx].ServiceName {
							compose.StopService(localCtx, dockeClient, &supervisor.DockerProject.Services[srvIdx])
							compose.RemoveService(localCtx, dockeClient, &supervisor.DockerProject.Services[srvIdx], logger)

							compose.BuildSingle(localCtx, dockeClient, supervisor.DockerProject.Name, &supervisor.DockerProject.Services[srvIdx], logger)
							compose.CreateNetwork(localCtx, supervisor.DockerProject, dockeClient, false, logger)
							compose.CreateSingleContainer(localCtx, supervisor.DockerProject.Name, &supervisor.DockerProject.Services[srvIdx], &supervisor.DockerProject.Networks[0], dockeClient, logger)
							compose.StartSingleServiceContainer(localCtx, dockeClient, &supervisor.DockerProject.Services[srvIdx], logger)
							supervisor.SupervisorServices[idx].UpdateReady = false
						}
					}

					for repoIdx := range supervisor.SupervisorServices[idx].Repos {
						_, err := supervisor.SupervisorServices[idx].Repos[repoIdx].GetUpstreamCommitUrl(localCtx, gitClient, "", logger)
						if err != nil {

						}
					}
				}
			}

			data, _ := yaml.Marshal(&supervisor.SupervisorServices)
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

func (s *RosSupervisor) AttachContainers() {
	for idx := range s.SupervisorServices {
		for _, service := range s.DockerProject.Services {
			if s.SupervisorServices[idx].ServiceName == service.Name {
				s.SupervisorServices[idx].ContainerName = service.Container.Name
				s.SupervisorServices[idx].ContainerID = service.Container.ID
			}
		}
	}
}

func (s *RosSupervisor) DisplayProject() {
	fmt.Printf("DOCKER PROJECT \n")
	compose.DisplayProject(s.DockerProject)
	fmt.Printf("SUPERVISOR CONFIG \n")
	for _, service := range s.SupervisorServices {
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

func (s *RosSupervisor) MonitorService() {

}
