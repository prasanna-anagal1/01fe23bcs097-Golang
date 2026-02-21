package handlers

import (
	"encoding/json"
	"net/http"

	"event-ticketing/middleware"
	"event-ticketing/models"
)

// registerRequest is the expected JSON body for user registration.
type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"` // "organizer" or "attendee"
}

// loginRequest is the expected JSON body for login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string      `json:"token"`
	User  *models.User `json:"user"`
}

// Register handles POST /api/register
func Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Name == "" || req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "name, email, and password are required")
		return
	}
	if req.Role != "organizer" && req.Role != "attendee" {
		req.Role = "attendee" // default role
	}

	user, err := models.CreateUser(req.Name, req.Email, req.Password, req.Role)
	if err != nil {
		writeError(w, http.StatusConflict, "email already registered: "+err.Error())
		return
	}

	token, err := middleware.GenerateToken(user.ID, user.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusCreated, authResponse{Token: token, User: user})
}

// Login handles POST /api/login
func Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	user, hashedPw, err := models.GetUserByEmail(req.Email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !models.CheckPassword(req.Password, hashedPw) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := middleware.GenerateToken(user.ID, user.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, authResponse{Token: token, User: user})
}
