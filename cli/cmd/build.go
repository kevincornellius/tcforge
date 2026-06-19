package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kevincornellius/tcforge/cli/internal/config"
	"github.com/kevincornellius/tcforge/cli/internal/docker"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build [problem...]",
	Short: "Compile tcframe specs and generate test cases into tc/",
	Long: `Compiles spec.cpp and solution.cpp for each problem using a Docker builder
container, then runs the tcframe runner to generate test cases into tc/.

Optionally pass problem IDs to build only specific problems:
  tcforge build A B`,
	RunE: runBuild,
}

func runBuild(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	cfg, err := config.Load(filepath.Join(cwd, yamlFilename))
	if err != nil {
		return fmt.Errorf("could not load tcforge.yaml: %w\nRun 'tcforge init' first", err)
	}

	problems := cfg.Problems
	if len(args) > 0 {
		problems = filterProblems(cfg.Problems, args)
		if len(problems) == 0 {
			return fmt.Errorf("no matching problems found for: %v", args)
		}
	}

	if err := docker.CheckRunning(); err != nil {
		return err
	}

	if err := docker.PullIfMissing(docker.BuilderImage); err != nil {
		return fmt.Errorf("failed to pull builder image: %w", err)
	}

	var failed []string
	for _, p := range problems {
		fmt.Printf("\n[%s] Building...\n", p.ID)
		problemDir := filepath.Join(cwd, p.Path)

		if err := docker.Run(docker.BuilderImage, cwd, "/contest/"+p.Path); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] FAILED: %v\n", p.ID, err)
			failed = append(failed, p.ID)
			continue
		}

		tcDir := filepath.Join(problemDir, "tc")
		count, _ := countFiles(tcDir, ".in")
		fmt.Printf("[%s] OK — %d test case(s) in tc/\n", p.ID, count)
	}

	if len(failed) > 0 {
		return fmt.Errorf("build failed for: %v", failed)
	}
	return nil
}

func filterProblems(problems []config.Problem, ids []string) []config.Problem {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	var out []config.Problem
	for _, p := range problems {
		if set[p.ID] {
			out = append(out, p)
		}
	}
	return out
}

func countFiles(dir, ext string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ext {
			count++
		}
	}
	return count, nil
}
