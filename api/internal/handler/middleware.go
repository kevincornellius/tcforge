package handler

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
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

// RequireContestOpen blocks non-admins when the contest isn't running.
// Always-open contests and contests without a start_at set are never blocked.
func RequireContestOpen(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := userFromContext(r.Context())
		if open, reason := IsContestOpen(user); !open {
			http.Error(w, reason, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
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

func userByToken(tokenStr string) (*User, error) {
	c := &claims{}
	_, err := jwt.ParseWithClaims(tokenStr, c, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	// Verify the user still exists in this DB (guards against tokens from a different DB).
	var exists int
	if err := db.DB.QueryRow("SELECT 1 FROM users WHERE id = ?", c.UserID).Scan(&exists); err != nil {
		if err == sql.ErrNoRows {
			return nil, jwt.ErrTokenInvalidClaims
		}
		return nil, err
	}
	return &User{
		ID:          c.UserID,
		Username:    c.Subject,
		DisplayName: c.DisplayName,
		IsAdmin:     c.IsAdmin,
	}, nil
}
