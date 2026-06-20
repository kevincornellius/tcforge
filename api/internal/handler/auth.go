package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/kevincornellius/tcforge/api/internal/db"
	"golang.org/x/crypto/bcrypt"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type claims struct {
	UserID      int    `json:"uid"`
	DisplayName string `json:"name"`
	IsAdmin     bool   `json:"admin"`
	jwt.RegisteredClaims
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

	c := claims{
		UserID:      userID,
		DisplayName: displayName,
		IsAdmin:     isAdmin == 1,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   req.Username,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(jwtSecret)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]any{
		"token":        token,
		"username":     req.Username,
		"display_name": displayName,
		"is_admin":     isAdmin == 1,
	})
}

func Logout(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func Me(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r.Context())
	json.NewEncoder(w).Encode(user)
}
