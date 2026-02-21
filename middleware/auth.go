package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type contextKey string

const UserIDKey contextKey = "userID"
const UserRoleKey contextKey = "userRole"

// tokenSecret is used to sign/verify tokens.
// In production load this from an environment variable.
var tokenSecret = func() string {
	if s := os.Getenv("TOKEN_SECRET"); s != "" {
		return s
	}
	return "event-ticketing-secret-key-2026"
}()

// tokenPayload is the JSON claims embedded in the token.
type tokenPayload struct {
	UserID    int64  `json:"user_id"`
	UserRole  string `json:"role"`
	ExpiresAt int64  `json:"exp"`
}

// GenerateToken creates a signed HMAC-SHA256 token for a user.
func GenerateToken(userID int64, role string) (string, error) {
	payload := tokenPayload{
		UserID:    userID,
		UserRole:  role,
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encoded := base64.URLEncoding.EncodeToString(data)
	sig := sign(encoded)
	return encoded + "." + sig, nil
}

// verifyToken validates the token signature and expiry, returning the payload.
func verifyToken(token string) (*tokenPayload, bool) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return nil, false
	}
	if sign(parts[0]) != parts[1] {
		return nil, false
	}
	data, err := base64.URLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, false
	}
	var p tokenPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, false
	}
	if time.Now().Unix() > p.ExpiresAt {
		return nil, false
	}
	return &p, true
}

// sign creates an HMAC-SHA256 signature for a string.
func sign(data string) string {
	h := hmac.New(sha256.New, []byte(tokenSecret))
	h.Write([]byte(data))
	return base64.URLEncoding.EncodeToString(h.Sum(nil))
}

// Authenticate is an HTTP middleware that validates the Bearer token and
// injects userID and role into the request context.
func Authenticate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, `{"error":"missing or invalid Authorization header"}`, http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		payload, ok := verifyToken(token)
		if !ok {
			http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, payload.UserID)
		ctx = context.WithValue(ctx, UserRoleKey, payload.UserRole)
		next(w, r.WithContext(ctx))
	}
}

// GetUserID extracts the authenticated user's ID from context.
func GetUserID(r *http.Request) int64 {
	v := r.Context().Value(UserIDKey)
	if v == nil {
		return 0
	}
	return v.(int64)
}

// GetUserRole extracts the authenticated user's role from context.
func GetUserRole(r *http.Request) string {
	v := r.Context().Value(UserRoleKey)
	if v == nil {
		return ""
	}
	return v.(string)
}

// GetPathID parses the last segment of a URL path as an int64.
// e.g. "/api/events/42" → 42
func GetPathID(r *http.Request) (int64, bool) {
	parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	if len(parts) == 0 {
		return 0, false
	}
	last := parts[len(parts)-1]
	// skip non-numeric segments (e.g. "register")
	id, err := strconv.ParseInt(last, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

// GetEventIDFromPath parses the event ID from paths like /api/events/42/register
func GetEventIDFromPath(r *http.Request) (int64, bool) {
	parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	// path: ["", "api", "events", "42", "register"]
	for i, p := range parts {
		if p == "events" && i+1 < len(parts) {
			id, err := strconv.ParseInt(parts[i+1], 10, 64)
			if err != nil {
				return 0, false
			}
			return id, true
		}
	}
	return 0, false
}
