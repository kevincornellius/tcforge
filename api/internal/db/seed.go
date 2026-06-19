package db

import (
	"fmt"
	"log"
	"path/filepath"

	"golang.org/x/crypto/bcrypt"
	"gopkg.in/yaml.v3"
	"os"
)

type contestConfig struct {
	Contest struct {
		Name     string `yaml:"name"`
		Duration string `yaml:"duration"`
	} `yaml:"contest"`
	Problems []struct {
		Path        string `yaml:"path"`
		ID          string `yaml:"id"`
		Title       string `yaml:"title"`
		TimeLimit   int    `yaml:"time_limit"`
		MemoryLimit int    `yaml:"memory_limit"`
	} `yaml:"problems"`
	Accounts []struct {
		Username    string `yaml:"username"`
		Password    string `yaml:"password"`
		DisplayName string `yaml:"display_name"`
		IsAdmin     bool   `yaml:"is_admin"`
	} `yaml:"accounts"`
}

func Seed(contestDir string) error {
	data, err := os.ReadFile(filepath.Join(contestDir, "tcforge.yaml"))
	if err != nil {
		return fmt.Errorf("could not read tcforge.yaml: %w", err)
	}

	var cfg contestConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("invalid tcforge.yaml: %w", err)
	}

	if err := seedUsers(cfg); err != nil {
		return err
	}
	if err := seedProblems(cfg); err != nil {
		return err
	}
	return nil
}

func seedUsers(cfg contestConfig) error {
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if count > 0 {
		return nil
	}

	for _, a := range cfg.Accounts {
		hash, err := bcrypt.GenerateFromPassword([]byte(a.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		isAdmin := 0
		if a.IsAdmin {
			isAdmin = 1
		}
		_, err = DB.Exec(
			"INSERT INTO users (username, password_hash, display_name, is_admin) VALUES (?, ?, ?, ?)",
			a.Username, string(hash), a.DisplayName, isAdmin,
		)
		if err != nil {
			return fmt.Errorf("seeding user %s: %w", a.Username, err)
		}
		log.Printf("seeded user: %s", a.Username)
	}
	return nil
}

func seedProblems(cfg contestConfig) error {
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM problems").Scan(&count)
	if count > 0 {
		return nil
	}

	for i, p := range cfg.Problems {
		tl := p.TimeLimit
		if tl == 0 {
			tl = 1
		}
		ml := p.MemoryLimit
		if ml == 0 {
			ml = 256
		}
		title := p.Title
		if title == "" {
			title = p.ID
		}
		_, err := DB.Exec(
			"INSERT INTO problems (slug, path, title, time_limit, memory_limit, position) VALUES (?, ?, ?, ?, ?, ?)",
			p.ID, p.Path, title, tl, ml, i,
		)
		if err != nil {
			return fmt.Errorf("seeding problem %s: %w", p.ID, err)
		}
		log.Printf("seeded problem: %s", p.ID)
	}
	return nil
}
