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

var deployImageFlag string
var deployPushFlag bool
var deployBaseTag string
var deployEmit bool

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Bake the contest into a single Docker image for cloud deployment",
	Long: `Packages the contest with the tcforge API and judge into a deployable Docker image.

Build locally (requires 'tcforge build' first):
  tcforge deploy                  # builds tcforge-<contest>:latest
  tcforge deploy --push           # build + push to registry
  tcforge deploy --image ghcr.io/you/c:v1 --push

Emit a Dockerfile for repo-based deployment (Koyeb/Railway "deploy from GitHub"):
  tcforge deploy --emit           # writes Dockerfile + entrypoint.sh
  Commit those files and link your repo on Koyeb/Railway/Render.
  Docker will run tcforge build automatically during image build.`,
	RunE: runDeploy,
}

func init() {
	deployCmd.Flags().StringVar(&deployImageFlag, "image", "", "Image name:tag to build (default: tcforge-<contest>:latest)")
	deployCmd.Flags().BoolVar(&deployPushFlag, "push", false, "Push image to registry after building")
	deployCmd.Flags().StringVar(&deployBaseTag, "base", "latest", "tcforge base image tag to source binaries from")
	deployCmd.Flags().BoolVar(&deployEmit, "emit", false, "Write Dockerfile + entrypoint.sh to the current directory for repo-based deployment")
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

	if deployEmit {
		return emitRepoMode(cfg, cwd, deployBaseTag)
	}

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
		name := sanitizeImageName(cfg.Contest.Name)
		image = "tcforge-" + name + ":latest"
	}

	tmpDir, err := os.MkdirTemp("", "tcforge-deploy-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	fmt.Println("→ Preparing build context...")

	contestDst := filepath.Join(tmpDir, "contest")
	if err := copyContest(cwd, contestDst); err != nil {
		return fmt.Errorf("copying contest: %w", err)
	}

	entrypoint := "#!/bin/sh\n/bin/judge &\nexec /bin/api\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "entrypoint.sh"), []byte(entrypoint), 0755); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "Dockerfile"), []byte(buildDeployDockerfile(deployBaseTag)), 0644); err != nil {
		return err
	}

	fmt.Printf("→ Building image %s (linux/amd64)...\n", image)
	buildArgs := []string{
		"build",
		"--platform", "linux/amd64",
		"-t", image,
		"--build-arg", "BASE_TAG=" + deployBaseTag,
		tmpDir,
	}
	buildCmd := exec.Command("docker", buildArgs...)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}

	fmt.Printf("\n✓ Image built: %s\n", image)

	if deployPushFlag {
		fmt.Printf("→ Pushing %s...\n", image)
		pushCmd := exec.Command("docker", "push", image)
		pushCmd.Stdout = os.Stdout
		pushCmd.Stderr = os.Stderr
		if err := pushCmd.Run(); err != nil {
			return fmt.Errorf("docker push failed: %w", err)
		}
		fmt.Printf("✓ Pushed: %s\n", image)
	}

	printDeployInstructions(image, deployPushFlag)
	return nil
}

func buildDeployDockerfile(baseTag string) string {
	registry := "ghcr.io/kevincornellius"
	return fmt.Sprintf(`# syntax=docker/dockerfile:1
ARG BASE_TAG=%s
FROM %s/tcforge-api:${BASE_TAG} AS api-src
FROM %s/tcforge-judge:${BASE_TAG}

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
EXPOSE 8080
ENTRYPOINT ["/entrypoint.sh"]
`, baseTag, registry, registry)
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

// emitRepoMode writes Dockerfile + entrypoint.sh + .dockerignore into cwd so
// the user can commit them and link the repo on Koyeb/Railway/Render.
// Docker will run tcframe build automatically during the image build.
func emitRepoMode(cfg *config.Config, cwd, baseTag string) error {
	dockerfilePath := filepath.Join(cwd, "Dockerfile")
	entrypointPath := filepath.Join(cwd, "entrypoint.sh")
	ignorePath := filepath.Join(cwd, ".dockerignore")

	// Warn if Dockerfile already exists
	if _, err := os.Stat(dockerfilePath); err == nil {
		fmt.Println("warning: Dockerfile already exists — overwriting")
	}

	df := buildEmitDockerfile(cfg, baseTag)
	if err := os.WriteFile(dockerfilePath, []byte(df), 0644); err != nil {
		return fmt.Errorf("writing Dockerfile: %w", err)
	}

	entrypoint := "#!/bin/sh\n/bin/judge &\nexec /bin/api\n"
	if err := os.WriteFile(entrypointPath, []byte(entrypoint), 0755); err != nil {
		return fmt.Errorf("writing entrypoint.sh: %w", err)
	}

	dockerignore := ".git\n.tcforge\n"
	if err := os.WriteFile(ignorePath, []byte(dockerignore), 0644); err != nil {
		return fmt.Errorf("writing .dockerignore: %w", err)
	}

	fmt.Println("✓ Dockerfile")
	fmt.Println("✓ entrypoint.sh")
	fmt.Println("✓ .dockerignore")
	fmt.Printf(`
Next steps:
  git add Dockerfile entrypoint.sh .dockerignore
  git commit -m "add tcforge deploy files"
  git push

Then on Koyeb (koyeb.com → New Service → GitHub):
  • Repo: your-repo   Branch: main
  • Builder: Dockerfile
  • Port: 8080
  • Deploy → get your .koyeb.app URL

On Railway (railway.app → New → GitHub Repo):
  • Port: 8080  (set PORT=8080 in env or railway detects EXPOSE)

On Render (render.com → New Web Service → GitHub):
  • Runtime: Docker   Port: 8080

Note: submissions are not persisted across restarts.
`)
	return nil
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
	fmt.Fprintf(&b, "FROM %s/tcforge-builder:%s AS tc-builder\n\n", registry, baseTag)
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
	fmt.Fprintf(&b, "FROM %s/tcforge-api:%s AS api-src\n\n", registry, baseTag)

	// Stage 3: final image (judge base has ubuntu 22.04 + g++ + isolate)
	b.WriteString("# ── Stage 3: final deploy image ─────────────────────────────────────────────\n")
	fmt.Fprintf(&b, "FROM %s/tcforge-judge:%s\n\n", registry, baseTag)
	b.WriteString("COPY --from=api-src /bin/api /bin/api\n")
	b.WriteString("COPY --from=api-src /app/web/dist /app/web/dist\n\n")
	b.WriteString("COPY --from=tc-builder /contest /contest\n\n")
	b.WriteString("COPY entrypoint.sh /entrypoint.sh\n")
	b.WriteString("RUN chmod +x /entrypoint.sh\n\n")
	b.WriteString("ENV TCFORGE_CONTEST_DIR=/contest\n")
	b.WriteString("EXPOSE 8080\n")
	b.WriteString("ENTRYPOINT [\"/entrypoint.sh\"]\n")

	return b.String()
}

func printDeployInstructions(image string, pushed bool) {
	pushNote := ""
	if !pushed {
		pushNote = fmt.Sprintf("\nPush first:  docker push %s\n", image)
	}

	fmt.Printf(`
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Test locally:
  docker run -p 6174:8080 %s
  open http://localhost:6174
%s
Deploy to Koyeb (koyeb.com → New Service → Docker):
  Image: %s  |  Port: 8080

Deploy to Railway (railway.app):
  railway up --image %s

Deploy to Render (render.com → New Web Service):
  Deploy an existing image: %s  |  Port: 8080

Note: submissions are not persisted across restarts.
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
`, image, pushNote, image, image, image)
}
