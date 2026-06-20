package handler

import (
	"encoding/json"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	mdparser "github.com/gomarkdown/markdown"
	mdhtml "github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
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

type StatementMeta struct {
	Language string `json:"language"`
	Label    string `json:"label"`
	Format   string `json:"format"`
}

var contestDir string

func SetContestDir(dir string) {
	contestDir = dir
}

var langLabels = map[string]string{
	"en": "English",
	"id": "Bahasa Indonesia",
	"ja": "日本語",
	"zh": "中文",
}

func langLabel(code string) string {
	if l, ok := langLabels[code]; ok {
		return l
	}
	return strings.ToUpper(code)
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
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = "en"
	}

	var p Problem
	var path string
	err := db.DB.QueryRow(
		"SELECT id, slug, path, title, time_limit, memory_limit, position FROM problems WHERE slug = ?", slug,
	).Scan(&p.ID, &p.Slug, &path, &p.Title, &p.TimeLimit, &p.MemoryLimit, &p.Position)
	if err != nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}

	availLangs := availableLanguages(p.ID, path)

	// Try requested lang, fall back to first available
	statement := loadStatement(p.ID, path, slug, lang)
	if statement == "" && len(availLangs) > 0 {
		statement = loadStatement(p.ID, path, slug, availLangs[0].Language)
	}

	json.NewEncoder(w).Encode(map[string]any{
		"problem":         p,
		"statement":       statement,
		"available_langs": availLangs,
	})
}

// GetSubtasks returns the subtask structure from config.json if present.
func GetSubtasks(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	var problemPath string
	if err := db.DB.QueryRow("SELECT path FROM problems WHERE slug = ?", slug).Scan(&problemPath); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	type subtaskConfig struct {
		TestGroups [][]int `json:"test_groups"`
		Points     []int   `json:"points"`
	}

	data, err := os.ReadFile(filepath.Join(contestDir, problemPath, "config.json"))
	if err != nil {
		json.NewEncoder(w).Encode(subtaskConfig{})
		return
	}

	var cfg subtaskConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		json.NewEncoder(w).Encode(subtaskConfig{})
		return
	}
	json.NewEncoder(w).Encode(cfg)
}

// ServeAsset serves static files (images, PDFs, etc.) from a problem's directory.
func ServeAsset(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	var problemPath string
	err := db.DB.QueryRow("SELECT path FROM problems WHERE slug = ?", slug).Scan(&problemPath)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	asset := chi.URLParam(r, "*")
	if strings.Contains(asset, "..") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	http.ServeFile(w, r, filepath.Join(contestDir, problemPath, asset))
}

// Admin: update problem title/limits
func UpdateProblem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var req struct {
		Title       string `json:"title"`
		TimeLimit   int    `json:"time_limit"`
		MemoryLimit int    `json:"memory_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.TimeLimit <= 0 {
		req.TimeLimit = 1
	}
	if req.MemoryLimit <= 0 {
		req.MemoryLimit = 256
	}
	db.DB.Exec("UPDATE problems SET title=?, time_limit=?, memory_limit=? WHERE id=?",
		req.Title, req.TimeLimit, req.MemoryLimit, id)
	w.WriteHeader(http.StatusNoContent)
}

// Admin: upload a statement file for a problem (multipart)
func UploadStatement(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "failed to parse form", http.StatusBadRequest)
		return
	}

	lang := strings.TrimSpace(r.FormValue("language"))
	if lang == "" {
		lang = "en"
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	format := strings.TrimPrefix(ext, ".")
	switch format {
	case "html", "pdf", "md", "tex":
	default:
		http.Error(w, "unsupported format: use html, pdf, md, or tex", http.StatusBadRequest)
		return
	}

	var problemPath string
	if err := db.DB.QueryRow("SELECT path FROM problems WHERE id=?", id).Scan(&problemPath); err != nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}

	stmtDir := filepath.Join(contestDir, problemPath, "statements")
	if err := os.MkdirAll(stmtDir, 0755); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	filename := lang + ext
	dst, err := os.Create(filepath.Join(stmtDir, filename))
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	db.DB.Exec("INSERT INTO problem_statements (problem_id, language, filename, format) VALUES (?,?,?,?) ON CONFLICT(problem_id, language) DO UPDATE SET filename=excluded.filename, format=excluded.format",
		id, lang, filename, format)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"language": lang, "format": format})
}

// ListStatements returns available statement languages for a problem.
func ListStatements(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	var problemID int
	var problemPath string
	if err := db.DB.QueryRow("SELECT id, path FROM problems WHERE slug=?", slug).Scan(&problemID, &problemPath); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(availableLanguages(problemID, problemPath))
}

// Admin: delete a statement
func DeleteStatement(w http.ResponseWriter, r *http.Request) {
	stmtID, err := strconv.Atoi(chi.URLParam(r, "stmtId"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var problemPath, filename string
	if err := db.DB.QueryRow(`SELECT p.path, ps.filename FROM problem_statements ps JOIN problems p ON ps.problem_id=p.id WHERE ps.id=?`, stmtID).Scan(&problemPath, &filename); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	os.Remove(filepath.Join(contestDir, problemPath, "statements", filename))
	db.DB.Exec("DELETE FROM problem_statements WHERE id=?", stmtID)
	w.WriteHeader(http.StatusNoContent)
}

// availableLanguages merges DB-tracked statements with legacy files in the problem dir.
func availableLanguages(problemID int, problemPath string) []StatementMeta {
	seen := map[string]bool{}
	var result []StatementMeta

	rows, _ := db.DB.Query("SELECT language, format FROM problem_statements WHERE problem_id=? ORDER BY language", problemID)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var lang, format string
			rows.Scan(&lang, &format)
			seen[lang] = true
			result = append(result, StatementMeta{Language: lang, Label: langLabel(lang), Format: format})
		}
	}

	// Legacy files
	legacyChecks := []struct{ lang, file, format string }{
		{"en", "description-en.html", "html"},
		{"en", "statement.md", "md"},
		{"id", "description-id.html", "html"},
	}
	for _, lc := range legacyChecks {
		if seen[lc.lang] {
			continue
		}
		if _, err := os.Stat(filepath.Join(contestDir, problemPath, lc.file)); err == nil {
			seen[lc.lang] = true
			result = append(result, StatementMeta{Language: lc.lang, Label: langLabel(lc.lang), Format: lc.format})
		}
	}

	return result
}

// loadStatement loads and renders a statement as HTML for the given language.
func loadStatement(problemID int, problemPath, slug, lang string) string {
	// Check DB-tracked statements first
	var filename, format string
	err := db.DB.QueryRow("SELECT filename, format FROM problem_statements WHERE problem_id=? AND language=?",
		problemID, lang).Scan(&filename, &format)
	if err == nil {
		data, err := os.ReadFile(filepath.Join(contestDir, problemPath, "statements", filename))
		if err == nil {
			return renderStatement(data, format, slug, filename)
		}
	}

	// Legacy files
	switch lang {
	case "en":
		if data, err := os.ReadFile(filepath.Join(contestDir, problemPath, "description-en.html")); err == nil {
			return renderStatement(data, "html", slug, "")
		}
		if data, err := os.ReadFile(filepath.Join(contestDir, problemPath, "statement.md")); err == nil {
			return renderStatement(data, "md", slug, "")
		}
	case "id":
		if data, err := os.ReadFile(filepath.Join(contestDir, problemPath, "description-id.html")); err == nil {
			return renderStatement(data, "html", slug, "")
		}
	}
	return ""
}

func renderStatement(data []byte, format, slug, filename string) string {
	switch format {
	case "html":
		return rewriteAssetPaths(string(data), slug)
	case "md":
		p := parser.NewWithExtensions(parser.CommonExtensions | parser.AutoHeadingIDs)
		renderer := mdhtml.NewRenderer(mdhtml.RendererOptions{Flags: mdhtml.CommonFlags})
		rendered := mdparser.ToHTML(data, p, renderer)
		return rewriteAssetPaths(string(rendered), slug)
	case "pdf":
		url := "/api/problems/" + slug + "/assets/statements/" + filename
		return `<div class="pdf-embed"><object data="` + url + `" type="application/pdf" width="100%" height="800px">` +
			`<p>PDF cannot be displayed. <a href="` + url + `" target="_blank">Download PDF</a></p></object></div>`
	case "tex":
		return `<div class="tex-source">` + html.EscapeString(string(data)) + `</div>`
	}
	return string(data)
}

var reSrc = regexp.MustCompile(`(?i)(src|href)="([^"]*)"`)

func rewriteAssetPaths(htmlStr, slug string) string {
	return reSrc.ReplaceAllStringFunc(htmlStr, func(m string) string {
		parts := reSrc.FindStringSubmatch(m)
		if len(parts) < 3 {
			return m
		}
		attr, val := parts[1], parts[2]
		if strings.HasPrefix(val, "http") || strings.HasPrefix(val, "/") ||
			strings.HasPrefix(val, "data:") || strings.HasPrefix(val, "#") {
			return m
		}
		return attr + `="/api/problems/` + slug + `/assets/` + val + `"`
	})
}
