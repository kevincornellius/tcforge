package cmd

import (
	"fmt"
	"os"

	"github.com/kevincornellius/tcforge/cli/internal/compose"
	"github.com/kevincornellius/tcforge/cli/internal/docker"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the contest stack",
	RunE:  runStop,
}

func runStop(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := docker.CheckRunning(); err != nil {
		return err
	}

	composePath := compose.ComposePath(cwd)
	if _, err := os.Stat(composePath); err != nil {
		return fmt.Errorf("no running contest found in this directory (missing .tcforge/docker-compose.yml)")
	}

	fmt.Println("Stopping contest stack...")
	return compose.Down(composePath)
}
