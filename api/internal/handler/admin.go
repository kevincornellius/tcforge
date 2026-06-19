package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/kevincornellius/tcforge/api/internal/db"
	"golang.org/x/crypto/bcrypt"
)

func ListUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := db.DB.Query(
		"SELECT id, username, display_name, is_admin FROM users ORDER BY id",
	)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type row struct {
		ID          int    `json:"id"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		IsAdmin     bool   `json:"is_admin"`
	}
	users := []row{}
	for rows.Next() {
		var u row
		var isAdmin int
		rows.Scan(&u.ID, &u.Username, &u.DisplayName, &isAdmin)
		u.IsAdmin = isAdmin == 1
		users = append(users, u)
	}
	json.NewEncoder(w).Encode(users)
}

func CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
		IsAdmin     bool   `json:"is_admin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" || req.Password == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Username
	}
	isAdmin := 0
	if req.IsAdmin {
		isAdmin = 1
	}

	res, err := db.DB.Exec(
		"INSERT INTO users (username, password_hash, display_name, is_admin) VALUES (?,?,?,?)",
		req.Username, string(hash), displayName, isAdmin,
	)
	if err != nil {
		http.Error(w, "username already exists", http.StatusConflict)
		return
	}
	id, _ := res.LastInsertId()
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"id": id})
}

func DeleteUser(w http.ResponseWriter, r *http.Request) {
	me := userFromContext(r.Context())
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	if me.ID == id {
		http.Error(w, "cannot delete yourself", http.StatusBadRequest)
		return
	}
	db.DB.Exec("DELETE FROM users WHERE id=?", id)
	w.WriteHeader(http.StatusNoContent)
}

func ResetPassword(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Password == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	db.DB.Exec("UPDATE users SET password_hash=? WHERE id=?", string(hash), id)
	w.WriteHeader(http.StatusNoContent)
}
