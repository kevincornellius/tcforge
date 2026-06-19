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

	jsonMW := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			next.ServeHTTP(w, r)
		})
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Public
	r.With(jsonMW).Get("/health", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"ok":true}`)) })
	r.With(jsonMW).Post("/api/auth/login", handler.Login)

	// Authenticated
	r.Group(func(r chi.Router) {
		r.Use(handler.RequireAuth)
		r.Use(jsonMW)

		r.Post("/api/auth/logout", handler.Logout)
		r.Get("/api/auth/me", handler.Me)

		r.Get("/api/problems", handler.ListProblems)
		r.Get("/api/problems/{slug}", handler.GetProblem)

		r.Get("/api/submissions", handler.ListSubmissions)
		r.Post("/api/submissions", handler.Submit)
		r.Get("/api/submissions/{id}", handler.GetSubmission)

		r.Get("/api/scoreboard", handler.GetScoreboard)
	})

	// Public asset serving (images referenced in problem statements)
	r.Get("/api/problems/{slug}/assets/*", handler.ServeAsset)

	// Admin routes (auth + admin required)
	r.Group(func(r chi.Router) {
		r.Use(handler.RequireAuth)
		r.Use(handler.RequireAdmin)
		r.Use(jsonMW)

		r.Get("/api/admin/users", handler.ListUsers)
		r.Post("/api/admin/users", handler.CreateUser)
		r.Delete("/api/admin/users/{id}", handler.DeleteUser)
		r.Put("/api/admin/users/{id}/password", handler.ResetPassword)
	})

	// Serve pre-built React frontend (SPA fallback to index.html)
	distDir := "/app/web/dist"
	r.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		path := distDir + req.URL.Path
		if _, err := os.Stat(path); os.IsNotExist(err) {
			http.ServeFile(w, req, distDir+"/index.html")
			return
		}
		http.FileServer(http.Dir(distDir)).ServeHTTP(w, req)
	}))

	log.Println("api listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
