package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kevincornellius/tcforge/cli/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold tcforge.yaml in the current contest directory",
	RunE:  runInit,
}

const yamlFilename = "tcforge.yaml"

func runInit(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	yamlPath := filepath.Join(cwd, yamlFilename)
	if _, err := os.Stat(yamlPath); err == nil {
		return errors.New("tcforge.yaml already exists — delete it first if you want to reinitialise")
	}

	problems, err := detectProblems(cwd)
	if err != nil {
		return err
	}
	if len(problems) == 0 {
		return errors.New("no problem directories found (looking for subdirectories containing spec.cpp)")
	}

	cfg := &config.Config{
		Contest: config.Contest{
			Name:     filepath.Base(cwd),
			Duration: "5h",
		},
		Problems: problems,
		Accounts: []config.Account{
			{Username: "admin", Password: "admin", DisplayName: "Admin"},
		},
		Judge: config.Judge{
			Languages: []string{"cpp17", "python3"},
		},
	}

	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	header := "# tcforge contest configuration\n# Edit contest name, duration, accounts, and problem titles as needed.\n\n"
	if err := os.WriteFile(yamlPath, append([]byte(header), raw...), 0644); err != nil {
		return err
	}

	fmt.Printf("Created %s with %d problem(s):\n", yamlFilename, len(problems))
	for _, p := range problems {
		fmt.Printf("  • %s\n", p.Path)
	}
	fmt.Println("\nNext: edit tcforge.yaml to set contest name, accounts, and problem titles.")
	fmt.Println("Then run: tcforge build")

	return nil
}

// detectProblems walks the current directory one level deep and returns
// subdirectories that contain a spec.cpp file.
func detectProblems(dir string) ([]config.Problem, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var problems []config.Problem
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name[0] == '.' {
			continue
		}
		specPath := filepath.Join(dir, name, "spec.cpp")
		if _, err := os.Stat(specPath); err != nil {
			continue
		}
		problems = append(problems, config.Problem{
			Path:  name,
			ID:    name,
			Title: name,
		})
	}
	return problems, nil
}
