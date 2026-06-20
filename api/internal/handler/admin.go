package handler

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

// RebuildProblem streams the builder Docker container output via SSE.
// It deletes config.json first so subtask assignments always regenerate from spec.cpp.
func RebuildProblem(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var problemPath string
	if err := db.DB.QueryRow("SELECT path FROM problems WHERE id=?", id).Scan(&problemPath); err != nil {
		http.Error(w, "problem not found", http.StatusNotFound)
		return
	}

	// Host path is needed for the Docker volume mount — the daemon sees host paths.
	hostContestDir := os.Getenv("TCFORGE_HOST_CONTEST_DIR")
	if hostContestDir == "" {
		hostContestDir = contestDir
	}

	tag := os.Getenv("TCFORGE_VERSION")
	if tag == "" {
		tag = "latest"
	}

	// Stream output line-by-line via SSE.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flush, canFlush := w.(http.Flusher)

	send := func(line string) {
		fmt.Fprintf(w, "data: %s\n\n", line)
		if canFlush {
			flush.Flush()
		}
	}

	// Delete config.json so subtask assignment regenerates from spec.cpp.
	cfgPath := filepath.Join(contestDir, problemPath, "config.json")
	if err := os.Remove(cfgPath); err == nil {
		send("[rebuild] Removed existing config.json — will regenerate from spec.cpp")
	}

	builderImage := "ghcr.io/kevincornellius/tcforge-builder:" + tag
	send(fmt.Sprintf("[rebuild] Running %s ...", builderImage))

	cmd := exec.CommandContext(r.Context(), "docker", "run", "--rm",
		"-v", hostContestDir+":/contest",
		builderImage,
		"/contest/"+problemPath,
	)

	// Merge stdout+stderr through an io.Pipe so we can scan lines while the
	// process runs. The goroutine closes pw when the process finishes, which
	// signals EOF to the scanner.
	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		send("[rebuild] ERROR: " + err.Error())
		send("DONE:error")
		return
	}

	var cmdErr error
	done := make(chan struct{})
	go func() {
		cmdErr = cmd.Wait()
		pw.Close()
		close(done)
	}()

	scanner := bufio.NewScanner(pr)
	for scanner.Scan() {
		send(scanner.Text())
	}
	<-done

	if cmdErr != nil {
		send("[rebuild] FAILED: " + cmdErr.Error())
		send("DONE:error")
		return
	}
	send("DONE:ok")
}
