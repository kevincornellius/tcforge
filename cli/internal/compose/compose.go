package compose

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	APIImage   = "ghcr.io/kevincornellius/tcforge-api:latest"
	JudgeImage = "ghcr.io/kevincornellius/tcforge-judge:latest"
)

// Generate writes a docker-compose.yml into .tcforge/ for the given contest dir.
func Generate(contestDir string) (string, error) {
	tcforgeDir := filepath.Join(contestDir, ".tcforge")
	if err := os.MkdirAll(tcforgeDir, 0755); err != nil {
		return "", err
	}

	composePath := filepath.Join(tcforgeDir, "docker-compose.yml")

	content := fmt.Sprintf(`services:
  api:
    image: %s
    ports:
      - "3000:8080"
    volumes:
      - %s:/contest
    environment:
      - TCFORGE_CONTEST_DIR=/contest
    restart: unless-stopped

  judge:
    image: %s
    privileged: true
    volumes:
      - %s:/contest
    environment:
      - TCFORGE_CONTEST_DIR=/contest
    restart: unless-stopped
    depends_on:
      - api
`, APIImage, contestDir, JudgeImage, contestDir)

	if err := os.WriteFile(composePath, []byte(content), 0644); err != nil {
		return "", err
	}
	return composePath, nil
}

// Up runs docker compose up -d for the given compose file.
func Up(composePath string) error {
	cmd := exec.Command("docker", "compose", "-f", composePath, "up", "-d", "--pull", "always")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Down stops and removes containers for the given compose file.
func Down(composePath string) error {
	cmd := exec.Command("docker", "compose", "-f", composePath, "down")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ComposePath returns the expected path of the compose file for a contest dir.
func ComposePath(contestDir string) string {
	return filepath.Join(contestDir, ".tcforge", "docker-compose.yml")
}
