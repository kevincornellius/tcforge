package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/kevincornellius/tcforge/api/internal/db"
)

type submitRequest struct {
	ProblemSlug string `json:"problem_slug"`
	Language    string `json:"language"`
	Code        string `json:"code"`
}

func Submit(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())

	var req submitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	var problemID int
	err := db.DB.QueryRow("SELECT id FROM problems WHERE slug = ?", req.ProblemSlug).Scan(&problemID)
	if err != nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}

	res, err := db.DB.Exec(
		"INSERT INTO submissions (user_id, problem_id, language, code) VALUES (?, ?, ?, ?)",
		user.ID, problemID, req.Language, req.Code,
	)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	id, _ := res.LastInsertId()
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"id": id})
}

func ListSubmissions(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())

	rows, err := db.DB.Query(`
		SELECT s.id, p.slug, p.title, s.language, s.status, s.verdict, s.score, s.time_ms, s.submitted_at
		FROM submissions s JOIN problems p ON s.problem_id = p.id
		WHERE s.user_id = ?
		ORDER BY s.submitted_at DESC`, user.ID,
	)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type row struct {
		ID          int    `json:"id"`
		ProblemSlug string `json:"problem_slug"`
		ProblemTitle string `json:"problem_title"`
		Language    string `json:"language"`
		Status      string `json:"status"`
		Verdict     string `json:"verdict"`
		Score       int    `json:"score"`
		TimeMs      int    `json:"time_ms"`
		SubmittedAt string `json:"submitted_at"`
	}

	results := []row{}
	for rows.Next() {
		var r row
		rows.Scan(&r.ID, &r.ProblemSlug, &r.ProblemTitle, &r.Language, &r.Status, &r.Verdict, &r.Score, &r.TimeMs, &r.SubmittedAt)
		results = append(results, r)
	}
	json.NewEncoder(w).Encode(results)
}

func GetSubmission(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var sub struct {
		ID          int    `json:"id"`
		ProblemSlug string `json:"problem_slug"`
		Language    string `json:"language"`
		Code        string `json:"code"`
		Status      string `json:"status"`
		Verdict     string `json:"verdict"`
		Score       int    `json:"score"`
		TimeMs      int    `json:"time_ms"`
		SubmittedAt string `json:"submitted_at"`
	}
	err := db.DB.QueryRow(`
		SELECT s.id, p.slug, s.language, s.code, s.status, s.verdict, s.score, s.time_ms, s.submitted_at
		FROM submissions s JOIN problems p ON s.problem_id = p.id
		WHERE s.id = ?`, id,
	).Scan(&sub.ID, &sub.ProblemSlug, &sub.Language, &sub.Code, &sub.Status, &sub.Verdict, &sub.Score, &sub.TimeMs, &sub.SubmittedAt)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	rows, _ := db.DB.Query(
		"SELECT test_case, verdict, time_ms, memory_kb FROM verdicts WHERE submission_id = ? ORDER BY test_case",
		id,
	)
	defer rows.Close()

	type v struct {
		TestCase string `json:"test_case"`
		Verdict  string `json:"verdict"`
		TimeMs   int    `json:"time_ms"`
		MemoryKB int    `json:"memory_kb"`
	}
	var verdicts []v
	for rows.Next() {
		var vv v
		rows.Scan(&vv.TestCase, &vv.Verdict, &vv.TimeMs, &vv.MemoryKB)
		verdicts = append(verdicts, vv)
	}

	json.NewEncoder(w).Encode(map[string]any{
		"submission": sub,
		"verdicts":   verdicts,
	})
}
