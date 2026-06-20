package compose

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const registry = "ghcr.io/kevincornellius"

// Generate writes a docker-compose.yml into .tcforge/ for the given contest dir.
func Generate(contestDir, tag string) (string, error) {
	if tag == "" {
		tag = "latest"
	}
	apiImage := registry + "/tcforge-api:" + tag
	judgeImage := registry + "/tcforge-judge:" + tag
	tcforgeDir := filepath.Join(contestDir, ".tcforge")
	if err := os.MkdirAll(tcforgeDir, 0755); err != nil {
		return "", err
	}

	composePath := filepath.Join(tcforgeDir, "docker-compose.yml")

	// Forward optional env vars into containers if set in the host environment
	extraAPI := ""
	extraJudge := ""
	for _, key := range []string{"DB_TYPE", "DATABASE_URL", "JWT_SECRET"} {
		if val := os.Getenv(key); val != "" {
			extraAPI += fmt.Sprintf("      - %s=%s\n", key, val)
			extraJudge += fmt.Sprintf("      - %s=%s\n", key, val)
		}
	}

	content := fmt.Sprintf(`services:
  api:
    image: %s
    ports:
      - "6174:8080"
    volumes:
      - %s:/contest
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - TCFORGE_CONTEST_DIR=/contest
      - TCFORGE_HOST_CONTEST_DIR=%s
      - TCFORGE_VERSION=%s
%s    restart: unless-stopped
    healthcheck:
      test: ["CMD-SHELL", "wget -qO/dev/null http://localhost:8080/health || exit 1"]
      interval: 5s
      timeout: 3s
      retries: 10
      start_period: 5s
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"

  judge:
    image: %s
    privileged: true
    volumes:
      - %s:/contest
    environment:
      - TCFORGE_CONTEST_DIR=/contest
%s    restart: unless-stopped
    depends_on:
      api:
        condition: service_healthy
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
`, apiImage, contestDir, contestDir, tag, extraAPI, judgeImage, contestDir, extraJudge)

	if err := os.WriteFile(composePath, []byte(content), 0644); err != nil {
		return "", err
	}
	return composePath, nil
}

// Up runs docker compose up -d for the given compose file.
// pull=true pulls fresh images first (skipped for dev tag so local builds are used as-is).
func Up(composePath string, pull bool) error {
	if pull {
		fmt.Println("Pulling latest images...")
		pullCmd := exec.Command("docker", "compose", "-f", composePath, "pull", "--quiet")
		pullCmd.Stdout = os.Stdout
		pullCmd.Stderr = os.Stderr
		_ = pullCmd.Run() // non-fatal: offline or first run may skip gracefully
	}
	cmd := exec.Command("docker", "compose", "-f", composePath, "up", "-d", "--pull", "missing")
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
