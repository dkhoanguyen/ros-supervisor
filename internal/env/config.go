package env

import (
	"context"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	SupervisorProjectPath string `env:"SUPERVISOR_DOCKER_PROJECT_PATH"`
	SupervisorComposeFile string `env:"SUPERVISOR_DOCKER_COMPOSE_FILE"`
	SupervisorConfigFile  string `env:"SUPERVISOR_CONFIG_FILE"`

	GitAccessToken string `env:"GITHUB_ACCESS_TOKEN"`
	UpdateFreq     string `env:"UPDATE_FREQUENCY"`

	LoggingLevel string `env:"LOGGING_LEVEL"`
	LoggingPath  string `env:"LOGGING_PATH"`
}

func LoadConfig(ctx context.Context) (*Config, error) {
	var err error
	config := Config{}

	if err = godotenv.Load(".env.local"); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Error loading .env.local %v \n", err)
		return nil, err
	}

	if err = envconfig.Process(ctx, &config); err != nil {
		return nil, err
	}
	fmt.Println(config)
	return &config, err
}
