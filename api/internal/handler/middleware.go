package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/kevincornellius/tcforge/api/internal/db"
)

type contextKey string

const userKey contextKey = "user"

type User struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	IsAdmin     bool   `json:"is_admin"`
}

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := tokenFromRequest(r)
		if token == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := userByToken(token)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := userFromContext(r.Context())
		if user == nil || !user.IsAdmin {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func userFromContext(ctx context.Context) *User {
	u, _ := ctx.Value(userKey).(*User)
	return u
}

func tokenFromRequest(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func userByToken(token string) (*User, error) {
	u := &User{}
	var isAdmin int
	err := db.DB.QueryRow(`
		SELECT u.id, u.username, u.display_name, u.is_admin
		FROM sessions s JOIN users u ON s.user_id = u.id
		WHERE s.token = ?`, token,
	).Scan(&u.ID, &u.Username, &u.DisplayName, &isAdmin)
	if err != nil {
		return nil, err
	}
	u.IsAdmin = isAdmin == 1
	return u, nil
}
