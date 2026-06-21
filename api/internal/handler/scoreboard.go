package handler

import (
	"encoding/json"
	"net/http"

	"github.com/kevincornellius/tcforge/api/internal/db"
)

type ScoreboardEntry struct {
	UserID      int            `json:"user_id"`
	DisplayName string         `json:"display_name"`
	TotalScore  int            `json:"total_score"`
	Problems    map[string]int `json:"problems"` // slug → best score
}

type ScoreboardData struct {
	Entries         []*ScoreboardEntry `json:"entries"`
	FirstSolvers    map[string]int     `json:"first_solvers"`     // slug → user_id of earliest full-score submission
	ProblemMaxScore map[string]int     `json:"problem_max_score"` // slug → max possible score
}

func GetScoreboard(w http.ResponseWriter, r *http.Request) {
	// Best score per user per problem
	rows, err := db.DB.Query(`
		SELECT u.id, u.display_name, p.slug, MAX(s.score)
		FROM submissions s
		JOIN users u ON s.user_id = u.id
		JOIN problems p ON s.problem_id = p.id
		WHERE s.verdict != ''
		GROUP BY u.id, p.id
		ORDER BY u.display_name
	`)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	entries := map[int]*ScoreboardEntry{}
	for rows.Next() {
		var uid, score int
		var displayName, slug string
		rows.Scan(&uid, &displayName, &slug, &score)

		if _, ok := entries[uid]; !ok {
			entries[uid] = &ScoreboardEntry{
				UserID:      uid,
				DisplayName: displayName,
				Problems:    map[string]int{},
			}
		}
		entries[uid].Problems[slug] = score
		entries[uid].TotalScore += score
	}

	result := make([]*ScoreboardEntry, 0, len(entries))
	for _, e := range entries {
		result = append(result, e)
	}

	// Max possible score per problem — sum of subtask max_scores from the earliest
	// completed submission. Falls back to 100 for problems with no subtask rows.
	maxRows, err := db.DB.Query(`
		SELECT p.slug, SUM(ss.max_score)
		FROM subtask_scores ss
		JOIN submissions s ON ss.submission_id = s.id
		JOIN problems p ON s.problem_id = p.id
		WHERE s.id IN (
			SELECT MIN(id) FROM submissions WHERE status = 'done' GROUP BY problem_id
		)
		GROUP BY p.id
	`)
	problemMaxScore := map[string]int{}
	if err == nil {
		defer maxRows.Close()
		for maxRows.Next() {
			var slug string
			var total int
			maxRows.Scan(&slug, &total)
			if total > 0 {
				problemMaxScore[slug] = total
			}
		}
	}

	// First solver: earliest submission that achieved the full score for each problem.
	fsRows, err := db.DB.Query(`
		SELECT p.slug, s.user_id
		FROM submissions s
		JOIN problems p ON s.problem_id = p.id
		JOIN (
			SELECT problem_id, MAX(score) AS top
			FROM submissions
			WHERE status = 'done' AND score > 0
			GROUP BY problem_id
		) ms ON s.problem_id = ms.problem_id AND s.score = ms.top
		WHERE s.status = 'done'
		GROUP BY s.problem_id
		ORDER BY MIN(s.id)
	`)
	firstSolvers := map[string]int{}
	if err == nil {
		defer fsRows.Close()
		for fsRows.Next() {
			var slug string
			var uid int
			fsRows.Scan(&slug, &uid)
			firstSolvers[slug] = uid
		}
	}

	json.NewEncoder(w).Encode(ScoreboardData{
		Entries:         result,
		FirstSolvers:    firstSolvers,
		ProblemMaxScore: problemMaxScore,
	})
}
