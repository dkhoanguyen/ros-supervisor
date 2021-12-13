package supervisor

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	dc "github.com/dkhoanguyen/ros-supervisor/cmd/compose"
	"github.com/dkhoanguyen/ros-supervisor/models/compose"
	"github.com/dkhoanguyen/ros-supervisor/pkg/github"
	"github.com/docker/docker/client"
	gh "github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"
)

type SupervisorServices []SupervisorService

type SupervisorService struct {
	ServiceName   string
	ContainerName string
	Repos         []github.Repo
	UpdateReady   bool
}

type RosSupervisor struct {
	DockerProject      *compose.Project
	SupervisorServices SupervisorServices
	ProjectDir         string
	MonitorTimeout     time.Duration
	ConfigFile         []byte
}

func CreateRosSupervisor(ctx context.Context, githubClient *gh.Client, configPath string, targetProject *compose.Project) RosSupervisor {
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
	supServices := extractServices(rawData, ctx, githubClient)
	supProject.SupervisorServices = supServices
	supProject.AttachContainers()
	return supProject
}

func extractServices(rawData map[interface{}]interface{}, ctx context.Context, githubClient *gh.Client) SupervisorServices {
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
				repo.GetCurrentLocalCommit(ctx, githubClient, "")
				supService.Repos = append(supService.Repos, repo)
			}
		}
		supServices = append(supServices, supService)
	}

	return supServices
}

func PrepareSupervisor(supervisor *RosSupervisor) (*client.Client, *gh.Client) {
	projectPath := os.Getenv("SUPERVISOR_DOCKER_PROJECT_PATH")
	composeFile := os.Getenv("SUPERVISOR_DOCKER_COMPOSE_FILE")
	configFile := os.Getenv("SUPERVISOR_CONFIG_FILE")
	gitAccessToken := os.Getenv("GITHUB_ACCESS_TOKEN")

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: gitAccessToken},
	)
	tc := oauth2.NewClient(ctx, ts)
	gitClient := gh.NewClient(tc)

	dockerCli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}

	composeProject := compose.CreateProject(composeFile, projectPath)

	if _, err := os.Stat("supervisor_services.yml"); err != nil {
		// File does not exist
		// Start full build
		dc.Build(ctx, dockerCli, &composeProject)
		dc.CreateContainers(ctx, &composeProject, dockerCli)
		dc.StartAllServiceContainer(ctx, dockerCli, &composeProject)
		// Update supervisor
		rs := CreateRosSupervisor(ctx, gitClient, configFile, &composeProject)
		supervisor = &rs

		data, _ := yaml.Marshal(&supervisor.SupervisorServices)
		ioutil.WriteFile("supervisor_services.yml", data, 0777)
	} else {
		// File exist
		// Extract existing info
		fmt.Println("Extracting info")
		allContainers := dc.ListAllContainers(ctx, dockerCli)

		for _, cnt := range allContainers {
			for idx, service := range composeProject.Services {
				if composeProject.Name+"_"+service.Name == cnt.Names[0][1:] {
					composeProject.Services[idx].Container.ID = cnt.ID
				}
			}
		}
		allImages := dc.ListAllImages(ctx, dockerCli)

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

		rs := CreateRosSupervisor(ctx, gitClient, configFile, &composeProject)
		serviceData := make([]SupervisorService, len(rs.SupervisorServices))
		yfile, _ := ioutil.ReadFile("supervisor_services.yml")
		yaml.Unmarshal(yfile, &serviceData)
		rs.SupervisorServices = serviceData
		supervisor = &rs
	}
	supervisor.DisplayProject()

	for {
		ctx := context.Background()
		for idx := range supervisor.SupervisorServices {
			for _, repo := range supervisor.SupervisorServices[idx].Repos {
				upStreamCommit := repo.UpdateUpStreamCommit(ctx, gitClient)
				fmt.Printf("Upstream: %s\n", upStreamCommit)
				if repo.IsUpdateReady() {
					fmt.Println("Update ready")
					supervisor.SupervisorServices[idx].UpdateReady = true
				}
			}
		}

		// Check once every 5s
		time.Sleep(5 * time.Second)

	}
	// Update supervisor settings
	return dockerCli, gitClient
}

func StartSupervisor(supervisor *RosSupervisor, dockeClient *client.Client, gitClient *gh.Client) {
	for {
		ctx := context.Background()
		for idx := range supervisor.SupervisorServices {
			for _, repo := range supervisor.SupervisorServices[idx].Repos {
				upStreamCommit := repo.UpdateUpStreamCommit(ctx, gitClient)
				fmt.Printf("Upstream: %s\n", upStreamCommit)
				if repo.IsUpdateReady() {
					fmt.Println("Update ready")
					supervisor.SupervisorServices[idx].UpdateReady = true
				}
			}
		}

		// Check once every 5s
		time.Sleep(5 * time.Second)

	}
}

func (s *RosSupervisor) StartDockerProject() {

}

func (s *RosSupervisor) CollectDockerProject() {

}

func (s *RosSupervisor) AttachContainers() {
	for idx := range s.SupervisorServices {
		for _, service := range s.DockerProject.Services {
			if s.SupervisorServices[idx].ServiceName == service.Name {
				s.SupervisorServices[idx].ContainerName = service.Container.Name
			}
		}
	}
}

func (s *RosSupervisor) MonitorServiceRepo() {
	for _, supService := range s.SupervisorServices {
		for _, repo := range supService.Repos {
			if repo.IsUpdateReady() {
				supService.UpdateReady = true
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
