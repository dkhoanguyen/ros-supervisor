package main

import (
	"fmt"

	yml "github.com/dkhoanguyen/ros-supervisor/handlers/yaml"
	"github.com/dkhoanguyen/ros-supervisor/models"
)

func main() {
	// cli, err := client.NewClientWithOpts(client.FromEnv)
	// if err != nil {
	// 	panic(err)
	// }

	// containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	// if err != nil {
	// 	panic(err)
	// }

	// for _, c := range containers {
	// 	// test, err := cli
	// 	// if err != nil {
	// 	// 	panic(err)
	// 	// }
	// 	container := models.MakeNodeContainer(c.Names[0], c.ID[:10], c.State)
	// 	fmt.Printf("Name: %s\n", container.GetName())
	// 	fmt.Printf("Status: %s\n", container.GetState())
	// }

	// images, err := cli.ImageList(context.Background(), types.ImageListOptions{})
	// if err != nil {
	// 	panic(err)
	// }

	// for _, image := range images {
	// 	fmt.Printf("Image: %d\n", image.Created)
	// }

	// client := github.NewClient(nil)
	// orgs, _, err := client.Organizations.List(context.Background(), "dkhoanguyen", nil)
	// ctx := context.Background()

	// psOpts := compose.psOptions{}
	// ts := oauth2.StaticTokenSource(
	// 	&oauth2.Token{AccessToken: "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"},
	// )

	// tc := oauth2.NewClient(ctx, ts)

	// client := github.NewClient(nil)

	// list all repositories for the authenticated user
	// repos, _, err := client.Repositories.List(ctx, "dkhoanguyen", nil)

	// for _, repo := range repos {
	// 	fmt.Printf("Repo Name: %s\n", *repo.Name)
	// 	fmt.Printf("Default branch : %s\n", *repo.DefaultBranch)
	// }
	// commits, _, err := client.Repositories.ListCommits(ctx, "gapaul", "dobot_magician_driver", nil)
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Printf("No commits : %d\n", len(commits))

	// // Get latest commits
	// latestCommits := commits[0]
	// fmt.Printf("Latest commit : %s\n", *latestCommits.SHA)
	// currentLocalCommits := commits[1]
	// fmt.Printf("Current commit : %s\n", *currentLocalCommits.SHA)

	yfile := yml.ReadYaml("ros-supervisor.yml")

	rsYaml := models.MakeRosSupervisorYaml(yfile)
	for _, service := range rsYaml.Services {
		fmt.Printf("Target: %s\n", service.Name)
	}

	// 	if env_vars_interface == nil {
	// 		continue
	// 	}
	// 	env_vars := env_vars_interface.([]interface{})
	// 	for _, env_var := range env_vars {
	// 		if strings.Contains(env_var.(string), "TARGET_REPO") {
	// 			fmt.Printf("%s - %s\n", service, env_var)
	// 			target_repo := env_var.(string)[len("TARGET_REPO=https://github.com/") : len(env_var.(string))-len(".git")]
	// 			user_repo := strings.Split(target_repo, "/")
	// 			fmt.Printf("User: %s\n", user_repo[0])
	// 			fmt.Printf("Repo: %s\n", user_repo[1])

	// 			commits, _, err := client.Repositories.ListCommits(ctx, user_repo[0], user_repo[1], nil)

	// 			if err != nil {
	// 				panic(err)
	// 			}
	// 			fmt.Printf("No commits : %d\n", len(commits))

	// 			// Get latest commits
	// 			latestCommits := commits[0]
	// 			fmt.Printf("Latest commit : %s\n", *latestCommits.SHA)
	// 			currentLocalCommits := commits[1]
	// 			fmt.Printf("Current commit : %s\n", *currentLocalCommits.SHA)
	// 		}
	// 	}
	// }

	// client.
	// project,err :=
}
