package handler

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/kevincornellius/tcforge/api/internal/db"
)

// Per-user submission rate limit: one submission per 15 seconds.
// Admins are exempt. State is in-memory; resets on API restart.
const submitCooldown = 15 * time.Second
const maxCodeBytes = 256 * 1024 // 256 KB

var (
	rlMu     sync.Mutex
	rlLastAt = map[int]time.Time{}
)

func submitRateLimit(userID int) bool {
	rlMu.Lock()
	defer rlMu.Unlock()
	if t, ok := rlLastAt[userID]; ok && time.Since(t) < submitCooldown {
		return false
	}
	rlLastAt[userID] = time.Now()
	return true
}

type submitRequest struct {
	ProblemSlug string `json:"problem_slug"`
	Language    string `json:"language"`
	Code        string `json:"code"`
}

func Submit(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())

	if user == nil || !user.IsAdmin {
		var alwaysOpen, allowSubmission int
		db.DB.QueryRow("SELECT always_open, allow_submission FROM contest_state WHERE id=1").
			Scan(&alwaysOpen, &allowSubmission)
		if alwaysOpen == 1 && allowSubmission == 0 {
			http.Error(w, "submissions are currently disabled", http.StatusForbidden)
			return
		}
		if !submitRateLimit(user.ID) {
			http.Error(w, "please wait before submitting again", http.StatusTooManyRequests)
			return
		}
	}

	var req submitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if len(req.Code) > maxCodeBytes {
		http.Error(w, "code exceeds 256 KB limit", http.StatusBadRequest)
		return
	}

	var problemID int
	err := db.DB.QueryRow("SELECT id FROM problems WHERE slug = ?", req.ProblemSlug).Scan(&problemID)
	if err != nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}

	id, err := db.DB.InsertReturningID(
		"INSERT INTO submissions (user_id, problem_id, language, code) VALUES (?, ?, ?, ?)",
		user.ID, problemID, req.Language, req.Code,
	)
	if err != nil {
		log.Printf("submit: insert error: %v", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"id": id})
}

func ListSubmissions(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())

	rows, err := db.DB.Query(`
		SELECT s.id, p.slug, p.title, s.language, s.status, s.verdict, s.score, s.time_ms, s.memory_kb, s.submitted_at, s.graded_at
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
		ID           int     `json:"id"`
		ProblemSlug  string  `json:"problem_slug"`
		ProblemTitle string  `json:"problem_title"`
		Language     string  `json:"language"`
		Status       string  `json:"status"`
		Verdict      string  `json:"verdict"`
		Score        int     `json:"score"`
		TimeMs       int     `json:"time_ms"`
		MemoryKb     int     `json:"memory_kb"`
		SubmittedAt  string  `json:"submitted_at"`
		GradedAt     *string `json:"graded_at"`
	}

	results := []row{}
	for rows.Next() {
		var r row
		rows.Scan(&r.ID, &r.ProblemSlug, &r.ProblemTitle, &r.Language, &r.Status, &r.Verdict, &r.Score, &r.TimeMs, &r.MemoryKb, &r.SubmittedAt, &r.GradedAt)
		results = append(results, r)
	}
	json.NewEncoder(w).Encode(results)
}

func GetSubmission(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(chi.URLParam(r, "id"))

	var sub struct {
		ID            int     `json:"id"`
		ProblemSlug   string  `json:"problem_slug"`
		ProblemTitle  string  `json:"problem_title"`
		Language      string  `json:"language"`
		Code          string  `json:"code"`
		Status        string  `json:"status"`
		Verdict       string  `json:"verdict"`
		Score         int     `json:"score"`
		TimeMs        int     `json:"time_ms"`
		MemoryKb      int     `json:"memory_kb"`
		CompileOutput string  `json:"compile_output"`
		SubmittedAt   string  `json:"submitted_at"`
		GradedAt      *string `json:"graded_at"`
	}
	err := db.DB.QueryRow(`
		SELECT s.id, p.slug, p.title, s.language, s.code, s.status, s.verdict, s.score, s.time_ms, s.memory_kb, s.compile_output, s.submitted_at, s.graded_at
		FROM submissions s JOIN problems p ON s.problem_id = p.id
		WHERE s.id = ?`, id,
	).Scan(&sub.ID, &sub.ProblemSlug, &sub.ProblemTitle, &sub.Language, &sub.Code, &sub.Status, &sub.Verdict, &sub.Score, &sub.TimeMs, &sub.MemoryKb, &sub.CompileOutput, &sub.SubmittedAt, &sub.GradedAt)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	vrows, _ := db.DB.Query(
		"SELECT test_case, verdict, time_ms, memory_kb, group_num, points_fraction FROM verdicts WHERE submission_id = ? ORDER BY group_num, test_case",
		id,
	)
	defer vrows.Close()

	type v struct {
		TestCase       string  `json:"test_case"`
		Verdict        string  `json:"verdict"`
		TimeMs         int     `json:"time_ms"`
		MemoryKB       int     `json:"memory_kb"`
		GroupNum       int     `json:"group_num"`
		PointsFraction float64 `json:"points_fraction"`
	}
	var verdicts []v
	for vrows.Next() {
		var vv v
		vrows.Scan(&vv.TestCase, &vv.Verdict, &vv.TimeMs, &vv.MemoryKB, &vv.GroupNum, &vv.PointsFraction)
		verdicts = append(verdicts, vv)
	}

	srows, _ := db.DB.Query(
		"SELECT subtask_num, verdict, score, max_score FROM subtask_scores WHERE submission_id = ? ORDER BY subtask_num",
		id,
	)
	defer srows.Close()

	type ss struct {
		SubtaskNum int    `json:"subtask_num"`
		Verdict    string `json:"verdict"`
		Score      int    `json:"score"`
		MaxScore   int    `json:"max_score"`
	}
	var subtaskScores []ss
	for srows.Next() {
		var s ss
		srows.Scan(&s.SubtaskNum, &s.Verdict, &s.Score, &s.MaxScore)
		subtaskScores = append(subtaskScores, s)
	}

	json.NewEncoder(w).Encode(map[string]any{
		"submission":     sub,
		"verdicts":       verdicts,
		"subtask_scores": subtaskScores,
	})
}
