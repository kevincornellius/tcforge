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

	json.NewEncoder(w).Encode(result)
}
