package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kevincornellius/tcforge/cli/internal/config"
	"github.com/kevincornellius/tcforge/cli/internal/docker"
	"github.com/spf13/cobra"
)

var deployBaseTag string
var deployFly bool
var deployLocal bool
var deployPush bool
var deployImageFlag string

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Generate deploy files for cloud deployment",
	Long: `Writes Dockerfile, entrypoint.sh, fly.toml, and .dockerignore to the current directory.

  tcforge deploy           # emit files + print instructions
  tcforge deploy --push    # emit + build + push to GHCR (auto-detects image from git remote)
  tcforge deploy --fly     # emit + deploy to Fly.io automatically
  tcforge deploy --local   # build image locally for testing

Docs: https://github.com/kevincornellius/tcforge/blob/main/docs/deploy.md`,
	RunE: runDeploy,
}

func init() {
	deployCmd.Flags().StringVar(&deployBaseTag, "base", "latest", "tcforge base image tag to source binaries from")
	deployCmd.Flags().BoolVar(&deployFly, "fly", false, "Deploy to Fly.io after emitting files")
	deployCmd.Flags().BoolVar(&deployLocal, "local", false, "Build image locally with Docker")
	deployCmd.Flags().BoolVar(&deployPush, "push", false, "Build and push image to GHCR (auto-detects from git remote)")
	deployCmd.Flags().StringVar(&deployImageFlag, "image", "", "Override image name for --push or --local (default: auto-detected)")
}

func runDeploy(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	cfg, err := config.Load(filepath.Join(cwd, yamlFilename))
	if err != nil {
		return fmt.Errorf("could not load tcforge.yaml: %w\nRun 'tcforge init' first", err)
	}

	if deployLocal {
		return localDockerBuild(cfg, cwd)
	}

	if deployPush {
		return ghcrPush(cfg, cwd)
	}

	if err := emitRepoMode(cfg, cwd, deployBaseTag); err != nil {
		return err
	}
	if deployFly {
		return flyDeploy(cwd, cfg)
	}
	return nil
}

func ghcrPush(cfg *config.Config, cwd string) error {
	image := deployImageFlag
	if image == "" {
		user := githubUserFromRemote(cwd)
		if user == "" {
			return fmt.Errorf("could not detect GitHub username from git remote\nSet it explicitly: tcforge deploy --push --image ghcr.io/you/%s:latest", sanitizeImageName(cfg.Contest.Name))
		}
		image = fmt.Sprintf("ghcr.io/%s/%s:latest", user, sanitizeImageName(cfg.Contest.Name))
	}

	if err := emitRepoMode(cfg, cwd, deployBaseTag); err != nil {
		return err
	}

	if err := docker.CheckRunning(); err != nil {
		return err
	}

	fmt.Printf("→ Building and pushing %s (linux/amd64)...\n", image)
	buildCmd := exec.Command("docker", "buildx", "build",
		"--platform", "linux/amd64",
		"--push",
		"-t", image,
		cwd,
	)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("docker build failed (run 'docker login ghcr.io' first if pushing to GHCR): %w", err)
	}

	fmt.Printf("\n✓ Image pushed: %s\n", image)
	fmt.Printf("\nDeploy on any platform using image: %s\n", image)
	fmt.Printf("Port: 8080 | Add persistent disk at /data for data persistence\n")
	fmt.Printf("\nDocs: https://github.com/kevincornellius/tcforge/blob/main/docs/deploy.md\n")
	return nil
}

func githubUserFromRemote(cwd string) string {
	out, err := exec.Command("git", "-C", cwd, "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	url := strings.TrimSpace(string(out))
	// https://github.com/user/repo or git@github.com:user/repo
	for _, prefix := range []string{"https://github.com/", "git@github.com:"} {
		if strings.HasPrefix(url, prefix) {
			parts := strings.SplitN(strings.TrimPrefix(url, prefix), "/", 2)
			if len(parts) >= 1 {
				return strings.ToLower(parts[0])
			}
		}
	}
	return ""
}

func flyDeploy(cwd string, cfg *config.Config) error {
	if _, err := exec.LookPath("fly"); err != nil {
		fmt.Println()
		fmt.Println("fly CLI not found. Install it and authenticate, then run 'tcforge deploy' again:")
		fmt.Println("  curl -L https://fly.io/install.sh | sh")
		fmt.Println("  fly auth login")
		return nil
	}

	appName := sanitizeImageName(cfg.Contest.Name)

	// Check if app exists; if not, run fly launch to create it + provision volume
	check := exec.Command("fly", "status", "-a", appName)
	check.Dir = cwd
	if err := check.Run(); err != nil {
		fmt.Printf("→ Creating Fly app %q...\n", appName)
		launch := exec.Command("fly", "launch", "--no-deploy", "--name", appName, "--copy-config")
		launch.Dir = cwd
		launch.Stdout = os.Stdout
		launch.Stderr = os.Stderr
		if err := launch.Run(); err != nil {
			return fmt.Errorf("fly launch failed: %w", err)
		}
	}

	fmt.Println("→ Deploying to Fly.io...")
	deploy := exec.Command("fly", "deploy", "-a", appName)
	deploy.Dir = cwd
	deploy.Stdout = os.Stdout
	deploy.Stderr = os.Stderr
	if err := deploy.Run(); err != nil {
		return fmt.Errorf("fly deploy failed: %w", err)
	}

	fmt.Printf("\n✓ Live at https://%s.fly.dev\n", appName)
	return nil
}

func localDockerBuild(cfg *config.Config, cwd string) error {
	fmt.Println("→ Checking test cases...")
	for _, p := range cfg.Problems {
		tcDir := filepath.Join(cwd, p.Path, "tc")
		n, err := countFiles(tcDir, ".in")
		if err != nil || n == 0 {
			return fmt.Errorf("problem %q has no test cases in tc/ — run 'tcforge build' first", p.ID)
		}
		fmt.Printf("  ✓ %s (%d test cases)\n", p.ID, n)
	}

	if err := docker.CheckRunning(); err != nil {
		return err
	}

	image := deployImageFlag
	if image == "" {
		image = "tcforge-" + sanitizeImageName(cfg.Contest.Name) + ":latest"
	}

	tmpDir, err := os.MkdirTemp("", "tcforge-deploy-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	fmt.Println("→ Preparing build context...")
	if err := copyContest(cwd, filepath.Join(tmpDir, "contest")); err != nil {
		return fmt.Errorf("copying contest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "entrypoint.sh"), []byte(deployEntrypoint()), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "Dockerfile"), []byte(buildDeployDockerfile(deployBaseTag)), 0644); err != nil {
		return err
	}

	fmt.Printf("→ Building image %s (linux/amd64)...\n", image)
	buildCmd := exec.Command("docker", "build", "--platform", "linux/amd64", "-t", image, "--build-arg", "BASE_TAG="+deployBaseTag, tmpDir)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}
	fmt.Printf("✓ Image built: %s\n", image)

	printDeployInstructions(image)
	return nil
}

func buildDeployDockerfile(baseTag string) string {
	registry := "ghcr.io/kevincornellius"
	return fmt.Sprintf(`# syntax=docker/dockerfile:1
ARG BASE_TAG=%s
FROM --platform=linux/amd64 %s/tcforge-api:${BASE_TAG} AS api-src
FROM --platform=linux/amd64 %s/tcforge-judge:${BASE_TAG}

COPY --from=api-src /bin/api /bin/api
COPY --from=api-src /app/web/dist /app/web/dist

COPY contest/ /contest/

# Recompile scorer/communicator for linux/amd64
RUN find /contest -name "scorer.cpp" \
      -exec sh -c 'g++ -O2 -std=c++20 -o "${1%%.cpp}" "$1"' _ {} \; && \
    find /contest -name "communicator.cpp" \
      -exec sh -c 'g++ -O2 -std=c++20 -o "${1%%.cpp}" "$1"' _ {} \;

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENV TCFORGE_CONTEST_DIR=/contest
ENV TCFORGE_VERSION=%s
EXPOSE 8080
ENTRYPOINT ["/entrypoint.sh"]
`, baseTag, registry, registry, Version)
}

func sanitizeImageName(name string) string {
	s := strings.ToLower(name)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "contest"
	}
	return result
}

// copyContest recursively copies contest files into dst, skipping files not
// needed at runtime (build artifacts, source binaries, docker config).
func copyContest(src, dst string) error {
	skipDirs := map[string]bool{
		".tcforge": true,
	}
	skipFiles := map[string]bool{
		"spec.cpp":           true,
		"solution.cpp":       true,
		"runner":             true,
		"solution":           true,
		"docker-compose.yml": true,
	}

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		base := info.Name()
		if info.IsDir() {
			if skipDirs[base] {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), info.Mode())
		}

		if skipFiles[base] {
			return nil
		}

		return copyFile(path, filepath.Join(dst, rel))
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func deployEntrypoint() string {
	return `#!/bin/sh
# If a persistent volume is mounted at /data, store the DB there so sessions
# and submissions survive container restarts. Otherwise use the baked-in path.
if [ -d /data ] && [ -w /data ]; then
    export TCFORGE_DB_PATH=/data/db.sqlite
fi
/bin/judge &
exec /bin/api
`
}

// emitRepoMode writes Dockerfile, entrypoint.sh, .dockerignore, and fly.toml
// into cwd so the user can commit and deploy with a single `fly deploy`.
func emitRepoMode(cfg *config.Config, cwd, baseTag string) error {
	files := map[string]struct {
		content []byte
		mode    os.FileMode
	}{
		"Dockerfile":    {[]byte(buildEmitDockerfile(cfg, baseTag)), 0644},
		"entrypoint.sh": {[]byte(deployEntrypoint()), 0755},
		".dockerignore": {[]byte(".git\n.tcforge\n"), 0644},
		"fly.toml":      {[]byte(buildFlyToml(cfg)), 0644},
	}

	for name, f := range files {
		path := filepath.Join(cwd, name)
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("warning: %s already exists — overwriting\n", name)
		}
		if err := os.WriteFile(path, f.content, f.mode); err != nil {
			return fmt.Errorf("writing %s: %w", name, err)
		}
		fmt.Printf("✓ %s\n", name)
	}

	fmt.Printf(`
Next steps — pick one:

  Push image to GHCR (deploy on any platform):
    tcforge deploy --push

  Deploy to Fly.io:
    tcforge deploy --fly

  Deploy from GitHub repo (Koyeb/Railway/Render):
    git add Dockerfile entrypoint.sh .dockerignore fly.toml
    git commit -m "deploy" && git push
    Link repo on platform → Dockerfile | Port 8080

Full guide: https://github.com/kevincornellius/tcforge/blob/main/docs/deploy.md
`)
	return nil
}

func buildFlyToml(cfg *config.Config) string {
	appName := sanitizeImageName(cfg.Contest.Name)
	return fmt.Sprintf(`# Generated by tcforge deploy --emit
# Run: fly launch --no-deploy && fly deploy

app = "%s"
primary_region = "sin"

[build]

[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = "stop"
  auto_start_machines = true
  min_machines_running = 0

[[vm]]
  memory = "512mb"
  cpu_kind = "shared"
  cpus = 1

[[mounts]]
  source = "tcforge_data"
  destination = "/data"
`, appName)
}

// buildEmitDockerfile generates a Dockerfile that runs tcframe build for each
// problem during docker build — no local tcforge build step needed.
func buildEmitDockerfile(cfg *config.Config, baseTag string) string {
	registry := "ghcr.io/kevincornellius"
	var b strings.Builder

	b.WriteString("# Auto-generated by tcforge deploy --emit\n")
	b.WriteString("# Commit this file, entrypoint.sh, and .dockerignore to your repo.\n")
	b.WriteString("# Koyeb/Railway/Render will run tcframe build automatically.\n")
	b.WriteString("# syntax=docker/dockerfile:1\n\n")

	// Stage 1: build test cases
	fmt.Fprintf(&b, "# ── Stage 1: compile specs + generate test cases ────────────────────────────\n")
	fmt.Fprintf(&b, "FROM --platform=linux/amd64 %s/tcforge-builder:%s AS tc-builder\n\n", registry, baseTag)
	fmt.Fprintf(&b, "COPY %s /contest/%s\n\n", yamlFilename, yamlFilename)

	for _, p := range cfg.Problems {
		// Normalise path separator to forward slash for Dockerfile COPY
		problemPath := filepath.ToSlash(p.Path)
		fmt.Fprintf(&b, "# Problem: %s — %s\n", p.ID, p.Title)
		fmt.Fprintf(&b, "COPY %s/ /contest/%s/\n", problemPath, problemPath)
		fmt.Fprintf(&b, "RUN cd /contest/%s && \\\n", problemPath)
		fmt.Fprintf(&b, "    g++ -O2 -std=c++20 -o solution solution.cpp && \\\n")
		fmt.Fprintf(&b, "    { [ -f scorer.cpp ] && g++ -O2 -std=c++20 -o scorer scorer.cpp || true; } && \\\n")
		fmt.Fprintf(&b, "    { [ -f communicator.cpp ] && g++ -O2 -std=c++20 -o communicator communicator.cpp || true; } && \\\n")
		fmt.Fprintf(&b, "    tcframe build && \\\n")
		fmt.Fprintf(&b, "    ./runner --solution=./solution && \\\n")
		fmt.Fprintf(&b, "    { [ ! -f config.json ] && python3 /parse_spec.py spec.cpp > config.json 2>/dev/null || true; } && \\\n")
		fmt.Fprintf(&b, "    rm -f solution runner spec.cpp solution.cpp\n\n")
	}

	// Stage 2: API source
	b.WriteString("# ── Stage 2: tcforge API binary + web frontend ──────────────────────────────\n")
	fmt.Fprintf(&b, "FROM --platform=linux/amd64 %s/tcforge-api:%s AS api-src\n\n", registry, baseTag)

	// Stage 3: final image (judge base has ubuntu 22.04 + g++ + isolate)
	b.WriteString("# ── Stage 3: final deploy image ─────────────────────────────────────────────\n")
	fmt.Fprintf(&b, "FROM --platform=linux/amd64 %s/tcforge-judge:%s\n\n", registry, baseTag)
	b.WriteString("COPY --from=api-src /bin/api /bin/api\n")
	b.WriteString("COPY --from=api-src /app/web/dist /app/web/dist\n\n")
	b.WriteString("COPY --from=tc-builder /contest /contest\n\n")
	b.WriteString("COPY entrypoint.sh /entrypoint.sh\n")
	b.WriteString("RUN chmod +x /entrypoint.sh\n\n")
	fmt.Fprintf(&b, "ENV TCFORGE_CONTEST_DIR=/contest\nENV TCFORGE_VERSION=%s\n", Version)
	b.WriteString("EXPOSE 8080\n")
	b.WriteString("ENTRYPOINT [\"/entrypoint.sh\"]\n")

	return b.String()
}

func printDeployInstructions(image string) {
	fmt.Printf(`
Test locally:
  docker run -p 6174:8080 %s

Docs: https://github.com/kevincornellius/tcforge/blob/main/docs/deploy.md
`, image)
}
