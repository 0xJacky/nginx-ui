package cmd

import (
	"context"

	"github.com/0xJacky/Nginx-UI/internal/docker"
	"github.com/gin-gonic/gin"
	"github.com/uozi-tech/cosy/logger"
	"github.com/urfave/cli/v3"
)

// Command to be executed in the temporary container
var UpgradeDockerStep2Command = &cli.Command{
	Name:   "upgrade-docker-step2",
	Usage:  "Execute the second step of Docker container upgrade (to be run inside the temp container)",
	Action: UpgradeDockerStep2,
}

// UpgradeDockerStep2 executes the second step in the temporary container
func UpgradeDockerStep2(ctx context.Context, command *cli.Command) error {
	logger.Init(gin.DebugMode)
	logger.Info("Starting Docker OTA upgrade step 2 from CLI...")

	return docker.UpgradeStepTwo(ctx)
}
