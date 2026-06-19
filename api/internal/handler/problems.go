package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/kevincornellius/tcforge/api/internal/db"
)

type Problem struct {
	ID          int    `json:"id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	TimeLimit   int    `json:"time_limit"`
	MemoryLimit int    `json:"memory_limit"`
	Position    int    `json:"position"`
}

var contestDir string

func SetContestDir(dir string) {
	contestDir = dir
}

func ListProblems(w http.ResponseWriter, r *http.Request) {
	rows, err := db.DB.Query(
		"SELECT id, slug, title, time_limit, memory_limit, position FROM problems ORDER BY position",
	)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	problems := []Problem{}
	for rows.Next() {
		var p Problem
		rows.Scan(&p.ID, &p.Slug, &p.Title, &p.TimeLimit, &p.MemoryLimit, &p.Position)
		problems = append(problems, p)
	}
	json.NewEncoder(w).Encode(problems)
}

func GetProblem(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	var p Problem
	var path string
	err := db.DB.QueryRow(
		"SELECT id, slug, path, title, time_limit, memory_limit, position FROM problems WHERE slug = ?", slug,
	).Scan(&p.ID, &p.Slug, &path, &p.Title, &p.TimeLimit, &p.MemoryLimit, &p.Position)
	if err != nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}

	statement := readStatement(path)
	json.NewEncoder(w).Encode(map[string]any{
		"problem":   p,
		"statement": statement,
	})
}

// readStatement tries description-en.html then statement.md.
func readStatement(problemPath string) string {
	for _, name := range []string{"description-en.html", "statement.md", "description-id.html"} {
		data, err := os.ReadFile(filepath.Join(contestDir, problemPath, name))
		if err == nil {
			return string(data)
		}
	}
	return ""
}
