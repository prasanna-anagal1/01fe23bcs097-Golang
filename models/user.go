package models

import (
	"database/sql"
	"errors"
	"time"

	"event-ticketing/db"
	"golang.org/x/crypto/bcrypt"
)

// User represents a registered user (organizer or attendee).
type User struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"` // "organizer" | "attendee"
	CreatedAt time.Time `json:"created_at"`
}

// CreateUser hashes the password and inserts a new user record.
// Returns the created User (without the hashed password).
func CreateUser(name, email, plainPassword, role string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	res, err := db.DB.Exec(
		`INSERT INTO users (name, email, password, role) VALUES (?, ?, ?, ?)`,
		name, email, string(hash), role,
	)
	if err != nil {
		return nil, err
	}

	id, _ := res.LastInsertId()
	return &User{
		ID:        id,
		Name:      name,
		Email:     email,
		Role:      role,
		CreatedAt: time.Now(),
	}, nil
}

// GetUserByEmail fetches a user row including the hashed password.
// Returns (user, hashedPassword, error).
func GetUserByEmail(email string) (*User, string, error) {
	row := db.DB.QueryRow(
		`SELECT id, name, email, password, role, created_at FROM users WHERE email = ?`, email,
	)
	u := &User{}
	var hashedPw string
	err := row.Scan(&u.ID, &u.Name, &u.Email, &hashedPw, &u.Role, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", errors.New("user not found")
	}
	return u, hashedPw, err
}

// GetUserByID fetches a user by their primary key.
func GetUserByID(id int64) (*User, error) {
	row := db.DB.QueryRow(
		`SELECT id, name, email, role, created_at FROM users WHERE id = ?`, id,
	)
	u := &User{}
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("user not found")
	}
	return u, err
}

// CheckPassword compares a plain-text password against the stored bcrypt hash.
func CheckPassword(plain, hashed string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}
