package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kevincornellius/tcforge/cli/internal/compose"
	"github.com/kevincornellius/tcforge/cli/internal/config"
	"github.com/kevincornellius/tcforge/cli/internal/docker"
	"github.com/spf13/cobra"
)

var tunnel bool
var imageTag string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Boot the full contest stack (web + API + judge)",
	RunE:  runServe,
}

func init() {
	serveCmd.Flags().BoolVar(&tunnel, "tunnel", false, "Print cloudflared tunnel instructions after starting")
	serveCmd.Flags().StringVar(&imageTag, "tag", "", "Docker image tag to use (default: latest)")
}

func runServe(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	cfg, err := config.Load(filepath.Join(cwd, yamlFilename))
	if err != nil {
		return fmt.Errorf("could not load tcforge.yaml: %w\nRun 'tcforge init' first", err)
	}

	if err := docker.CheckRunning(); err != nil {
		return err
	}

	composePath, err := compose.Generate(cwd, imageTag)
	if err != nil {
		return fmt.Errorf("failed to generate compose file: %w", err)
	}

	fmt.Printf("Starting contest: %s\n", cfg.Contest.Name)
	if err := compose.Up(composePath); err != nil {
		return fmt.Errorf("failed to start services: %w", err)
	}

	fmt.Println()
	fmt.Println("Contest is live at: http://localhost:6174")

	if tunnel {
		fmt.Println()
		fmt.Println("To share publicly, run in a separate terminal:")
		fmt.Println("  cloudflared tunnel --url http://localhost:6174")
	}

	return nil
}
