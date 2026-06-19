package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/kevincornellius/tcforge/api/internal/db"
	"github.com/kevincornellius/tcforge/api/internal/handler"
)

func main() {
	contestDir := os.Getenv("TCFORGE_CONTEST_DIR")
	if contestDir == "" {
		contestDir = "/contest"
	}

	if err := db.Init(contestDir); err != nil {
		log.Fatalf("db init: %v", err)
	}
	if err := db.Seed(contestDir); err != nil {
		log.Fatalf("db seed: %v", err)
	}

	handler.SetContestDir(contestDir)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			next.ServeHTTP(w, r)
		})
	})

	// Public
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"ok":true}`)) })
	r.Post("/api/auth/login", handler.Login)

	// Authenticated
	r.Group(func(r chi.Router) {
		r.Use(handler.RequireAuth)

		r.Post("/api/auth/logout", handler.Logout)
		r.Get("/api/auth/me", handler.Me)

		r.Get("/api/problems", handler.ListProblems)
		r.Get("/api/problems/{slug}", handler.GetProblem)

		r.Get("/api/submissions", handler.ListSubmissions)
		r.Post("/api/submissions", handler.Submit)
		r.Get("/api/submissions/{id}", handler.GetSubmission)

		r.Get("/api/scoreboard", handler.GetScoreboard)
	})

	// Serve pre-built React frontend
	fs := http.FileServer(http.Dir("/app/web/dist"))
	r.Handle("/*", fs)

	log.Println("api listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
