package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/kevincornellius/tcforge/api/internal/db"
)

type ContestState struct {
	Name            string  `json:"name"`
	Duration        string  `json:"duration"`
	Scoring         string  `json:"scoring"`
	AlwaysOpen      bool    `json:"always_open"`
	AllowSubmission bool    `json:"allow_submission"`
	StartAt         *string `json:"start_at"`
	EndAt           *string `json:"end_at"`
}

func GetContest(w http.ResponseWriter, r *http.Request) {
	var cs ContestState
	var alwaysOpen, allowSubmission int
	var startAt, endAt sql.NullString
	db.DB.QueryRow("SELECT name, duration, scoring, always_open, allow_submission, start_at, end_at FROM contest_state WHERE id=1").
		Scan(&cs.Name, &cs.Duration, &cs.Scoring, &alwaysOpen, &allowSubmission, &startAt, &endAt)
	cs.AlwaysOpen = alwaysOpen == 1
	cs.AllowSubmission = allowSubmission == 1
	if startAt.Valid {
		cs.StartAt = &startAt.String
	}
	if endAt.Valid {
		cs.EndAt = &endAt.String
	}
	json.NewEncoder(w).Encode(cs)
}

func UpdateContest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name            string `json:"name"`
		Duration        string `json:"duration"`
		Scoring         string `json:"scoring"`
		AlwaysOpen      bool   `json:"always_open"`
		AllowSubmission bool   `json:"allow_submission"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.Scoring != "ioi" && req.Scoring != "icpc" {
		req.Scoring = "ioi"
	}
	alwaysOpen, allowSubmission := 0, 0
	if req.AlwaysOpen {
		alwaysOpen = 1
	}
	if req.AllowSubmission {
		allowSubmission = 1
	}
	db.DB.Exec("UPDATE contest_state SET name=?, duration=?, scoring=?, always_open=?, allow_submission=? WHERE id=1",
		req.Name, req.Duration, req.Scoring, alwaysOpen, allowSubmission)
	w.WriteHeader(http.StatusNoContent)
}

// IsContestOpen returns true if submissions are currently allowed.
// Admins always bypass. Non-admins: always_open=true → open;
// otherwise open only if start_at is set and now is within [start_at, end_at].
func IsContestOpen(user *User) (open bool, reason string) {
	if user != nil && user.IsAdmin {
		return true, ""
	}
	var alwaysOpen int
	var startAt, endAt sql.NullString
	db.DB.QueryRow("SELECT always_open, start_at, end_at FROM contest_state WHERE id=1").
		Scan(&alwaysOpen, &startAt, &endAt)

	if alwaysOpen == 1 {
		return true, ""
	}
	if !startAt.Valid {
		return false, "contest has not been published yet"
	}

	now := time.Now()
	start, _ := time.Parse(time.RFC3339, startAt.String)
	if now.Before(start) {
		return false, "contest has not started yet"
	}
	if endAt.Valid {
		end, _ := time.Parse(time.RFC3339, endAt.String)
		if now.After(end) {
			return false, "contest has ended"
		}
	}
	return true, ""
}

func StartContest(w http.ResponseWriter, r *http.Request) {
	var dur string
	db.DB.QueryRow("SELECT duration FROM contest_state WHERE id=1").Scan(&dur)

	// Optional body: { "start_at": "2006-01-02T15:04" } for scheduled start
	var body struct {
		StartAt string `json:"start_at"` // local datetime-local value, e.g. "2006-01-02T15:04"
	}
	json.NewDecoder(r.Body).Decode(&body)

	var start time.Time
	if body.StartAt != "" {
		var err error
		// Try RFC3339 first, then datetime-local format
		start, err = time.ParseInLocation("2006-01-02T15:04", body.StartAt, time.Local)
		if err != nil {
			start, err = time.Parse(time.RFC3339, body.StartAt)
		}
		if err != nil {
			http.Error(w, "invalid start_at format", http.StatusBadRequest)
			return
		}
		start = start.UTC()
	} else {
		start = time.Now().UTC()
	}

	var end *time.Time
	if dur != "" {
		d, err := time.ParseDuration(dur)
		if err == nil && d > 0 {
			t := start.Add(d)
			end = &t
		}
	}

	if end != nil {
		db.DB.Exec("UPDATE contest_state SET start_at=?, end_at=? WHERE id=1",
			start.Format(time.RFC3339), end.Format(time.RFC3339))
	} else {
		db.DB.Exec("UPDATE contest_state SET start_at=?, end_at=NULL WHERE id=1",
			start.Format(time.RFC3339))
	}
	w.WriteHeader(http.StatusNoContent)
}

func StopContest(w http.ResponseWriter, r *http.Request) {
	db.DB.Exec("UPDATE contest_state SET end_at=? WHERE id=1", time.Now().UTC().Format(time.RFC3339))
	w.WriteHeader(http.StatusNoContent)
}

func ResetContest(w http.ResponseWriter, r *http.Request) {
	db.DB.Exec("UPDATE contest_state SET start_at=NULL, end_at=NULL WHERE id=1")
	w.WriteHeader(http.StatusNoContent)
}

// Announcements

type Announcement struct {
	ID        int    `json:"id"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

func ListAnnouncements(w http.ResponseWriter, r *http.Request) {
	rows, err := db.DB.Query("SELECT id, message, created_at FROM announcements ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	result := []Announcement{}
	for rows.Next() {
		var a Announcement
		rows.Scan(&a.ID, &a.Message, &a.CreatedAt)
		result = append(result, a)
	}
	json.NewEncoder(w).Encode(result)
}

func CreateAnnouncement(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Message == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	res, err := db.DB.Exec("INSERT INTO announcements (message) VALUES (?)", req.Message)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	id, _ := res.LastInsertId()
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"id": id})
}

func DeleteAnnouncement(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	db.DB.Exec("DELETE FROM announcements WHERE id=?", id)
	w.WriteHeader(http.StatusNoContent)
}

// Admin: list all submissions for rejudge view

type AdminSubmission struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	ProblemSlug  string  `json:"problem_slug"`
	ProblemTitle string  `json:"problem_title"`
	Language     string  `json:"language"`
	Status       string  `json:"status"`
	Verdict      string  `json:"verdict"`
	Score        int     `json:"score"`
	SubmittedAt  string  `json:"submitted_at"`
	GradedAt     *string `json:"graded_at"`
}

func ListAllSubmissions(w http.ResponseWriter, r *http.Request) {
	rows, err := db.DB.Query(`
		SELECT s.id, u.username, p.slug, p.title, s.language, s.status, s.verdict, s.score, s.submitted_at, s.graded_at
		FROM submissions s
		JOIN users u ON s.user_id = u.id
		JOIN problems p ON s.problem_id = p.id
		ORDER BY s.submitted_at DESC
		LIMIT 200
	`)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	result := []AdminSubmission{}
	for rows.Next() {
		var s AdminSubmission
		rows.Scan(&s.ID, &s.Username, &s.ProblemSlug, &s.ProblemTitle, &s.Language, &s.Status, &s.Verdict, &s.Score, &s.SubmittedAt, &s.GradedAt)
		result = append(result, s)
	}
	json.NewEncoder(w).Encode(result)
}

func RejudgeSubmission(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	db.DB.Exec("DELETE FROM verdicts WHERE submission_id=?", id)
	db.DB.Exec("DELETE FROM subtask_scores WHERE submission_id=?", id)
	db.DB.Exec("UPDATE submissions SET status='queued', verdict='', score=0, time_ms=0 WHERE id=?", id)
	w.WriteHeader(http.StatusNoContent)
}
