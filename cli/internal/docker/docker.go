package docker

import (
	"fmt"
	"os"
	"os/exec"
)

const BuilderImage = "ghcr.io/kevincornellius/tcforge-builder:latest"

// CheckRunning returns an error if Docker is not running.
func CheckRunning() error {
	cmd := exec.Command("docker", "info")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Docker is not running — please start Docker Desktop (or Docker Engine) and try again")
	}
	return nil
}

// PullIfMissing pulls the image if it is not already present locally.
func PullIfMissing(image string) error {
	check := exec.Command("docker", "image", "inspect", image)
	check.Stdout = nil
	check.Stderr = nil
	if check.Run() == nil {
		return nil // already present
	}

	fmt.Printf("Pulling %s...\n", image)
	pull := exec.Command("docker", "pull", image)
	pull.Stdout = os.Stdout
	pull.Stderr = os.Stderr
	return pull.Run()
}

// Run runs a Docker container, streaming stdout/stderr to the terminal.
// hostDir is mounted as /contest inside the container.
// args are passed to the container entrypoint.
func Run(image, hostDir string, args ...string) error {
	dockerArgs := []string{
		"run", "--rm",
		"-v", hostDir + ":/contest",
		image,
	}
	dockerArgs = append(dockerArgs, args...)

	cmd := exec.Command("docker", dockerArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
