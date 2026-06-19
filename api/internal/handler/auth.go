package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/kevincornellius/tcforge/api/internal/db"
	"golang.org/x/crypto/bcrypt"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	var userID int
	var hash, displayName string
	var isAdmin int
	err := db.DB.QueryRow(
		"SELECT id, password_hash, display_name, is_admin FROM users WHERE username = ?",
		req.Username,
	).Scan(&userID, &hash, &displayName, &isAdmin)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	token := randomToken()
	db.DB.Exec("INSERT INTO sessions (token, user_id) VALUES (?, ?)", token, userID)

	json.NewEncoder(w).Encode(map[string]any{
		"token":        token,
		"username":     req.Username,
		"display_name": displayName,
		"is_admin":     isAdmin == 1,
	})
}

func Logout(w http.ResponseWriter, r *http.Request) {
	token := tokenFromRequest(r)
	if token != "" {
		db.DB.Exec("DELETE FROM sessions WHERE token = ?", token)
	}
	w.WriteHeader(http.StatusNoContent)
}

func Me(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	json.NewEncoder(w).Encode(user)
}

func randomToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return hex.EncodeToString(b)
}
