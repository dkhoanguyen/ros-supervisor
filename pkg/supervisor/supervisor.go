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
	"github.com/dkhoanguyen/ros-supervisor/pkg/github"
	"github.com/dkhoanguyen/ros-supervisor/pkg/handlers/health"
	"github.com/dkhoanguyen/ros-supervisor/pkg/handlers/v1/supervisor"
	"github.com/docker/docker/client"
	"github.com/gin-gonic/gin"
	gh "github.com/google/go-github/github"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"
)

type SupervisorServices []SupervisorService

type SupervisorService struct {
	ServiceName   string
	ContainerName string
	ContainerID   string
	Repos         []github.Repo
	UpdateReady   bool
}

type RosSupervisor struct {
	DockerCli          *client.Client
	GitCli             *gh.Client
	DockerProject      *compose.Project
	SupervisorServices SupervisorServices
	ProjectDir         string
	MonitorTimeout     time.Duration
	ConfigFile         []byte
}

type SupervisorCommand struct {
	Update bool `json:"update"`
}

func CreateRosSupervisor(ctx context.Context, githubClient *gh.Client, configPath string, targetProject *compose.Project, logger *zap.Logger) RosSupervisor {
	supProject := RosSupervisor{
		DockerProject: targetProject,
	}
	configFile, err := ioutil.ReadFile(configPath)
	if err != nil {
		panic(err)
	}
	rawData := make(map[interface{}]interface{})
	err2 := yaml.Unmarshal(configFile, &rawData)
	if err2 != nil {
		log.Fatal(err2)
	}
	supServices := extractServices(rawData, ctx, githubClient, logger)
	supProject.SupervisorServices = supServices
	supProject.AttachContainers()
	return supProject
}

func extractServices(rawData map[interface{}]interface{}, ctx context.Context, githubClient *gh.Client, logger *zap.Logger) SupervisorServices {
	supServices := SupervisorServices{}
	services := rawData["services"].(map[string]interface{})

	for serviceName, serviceConfig := range services {
		supService := SupervisorService{}
		supService.ServiceName = serviceName
		repoLists := serviceConfig.([]interface{})
		for _, repoData := range repoLists {
			branch := repoData.(map[string]interface{})["branch"].(string)
			url := repoData.(map[string]interface{})["url"].(string)

			if commit, ok := repoData.(map[string]interface{})["current_commit"].(string); ok {
				repo := github.MakeRepository(url, branch, commit)
				supService.Repos = append(supService.Repos, repo)
			} else {
				repo := github.MakeRepository(url, branch, "")
				repo.GetCurrentLocalCommit(ctx, githubClient, "", logger)
				supService.Repos = append(supService.Repos, repo)
			}
		}
		supServices = append(supServices, supService)
	}

	return supServices
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
		Update: false,
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
		if utils.FileExists("/supervisor/project/docker-compose.yml") &&
			utils.FileExists("/supervisor/project/ros-supervisor.yml") {
			if err != nil {
				logger.Fatal(fmt.Sprintf("%s", err))
			}
			if cmd.Update {
				rs := PrepareSupervisor(ctx, &rs)
				cmd.Update = false
				StartSupervisor(ctx, &rs, dockerCli, gitClient, logger)
			} else {
				time.Sleep(2 * time.Second)
			}

		} else {
			time.Sleep(2 * time.Second)
		}
	}
}

func PrepareSupervisor(ctx context.Context, supervisor *RosSupervisor) RosSupervisor {
	localCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	envConfig, err := env.LoadConfig(localCtx)
	if err != nil {
		log.Fatal(err)
	}

	logger := logging.Make(envConfig)
	projectPath := envConfig.SupervisorProjectPath
	composeFile := envConfig.SupervisorComposeFile
	configFile := envConfig.SupervisorConfigFile

	dockerCli := supervisor.DockerCli
	gitClient := supervisor.GitCli

	composeProject := compose.CreateProject(composeFile, projectPath, logger)

	rs := RosSupervisor{}

	if _, err := os.Stat("/supervisor/supervisor_services.yml"); err != nil {
		// File does not exist
		// Check to see if there is any container with the given name is running
		// If yes then stop and remove all of them to rebuild the project
		// In the future, for ROS integration we should keep core running, as it is
		// rarely changed
		allRunningContainers, err := compose.ListAllContainers(localCtx, dockerCli, logger)
		if err != nil {

		}

		for _, cnt := range allRunningContainers {
			if strings.Contains(cnt.Names[0], composeProject.Name) {
				ID := cnt.ID
				compose.StopServiceByID(ctx, dockerCli, ID, logger)
				compose.RemoveServiceByID(ctx, dockerCli, ID, logger)
			}
		}

		// Start full build
		compose.Build(localCtx, dockerCli, &composeProject, logger)
		compose.CreateContainers(localCtx, dockerCli, &composeProject, logger)
		compose.StartAllServiceContainer(localCtx, dockerCli, &composeProject, logger)
		// Update supervisor
		rs = CreateRosSupervisor(localCtx, gitClient, configFile, &composeProject, logger)
		data, _ := yaml.Marshal(&rs.SupervisorServices)
		ioutil.WriteFile("/supervisor/supervisor_services.yml", data, 0777)
	} else {
		// File exist
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

		rs = CreateRosSupervisor(localCtx, gitClient, configFile, &composeProject, logger)
		serviceData := make([]SupervisorService, len(rs.SupervisorServices))
		yfile, _ := ioutil.ReadFile("/supervisor/supervisor_services.yml")
		yaml.Unmarshal(yfile, &serviceData)
		rs.SupervisorServices = serviceData
	}
	return rs
}

func StartSupervisor(ctx context.Context, supervisor *RosSupervisor, dockeClient *client.Client, gitClient *gh.Client, logger *zap.Logger) {
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
						_, err := supervisor.SupervisorServices[idx].Repos[repoIdx].GetCurrentLocalCommit(localCtx, gitClient, "", logger)
						if err != nil {

						}
					}
				}
			}

			data, _ := yaml.Marshal(&supervisor.SupervisorServices)
			ioutil.WriteFile("supervisor_services.yml", data, 0777)
		} else {
			logger.Info("Update is not ready.")
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
