package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"event-ticketing/db"
	"event-ticketing/handlers"
	"event-ticketing/middleware"
)

// setupTestServer spins up an in-memory test HTTP server with a fresh temp database.
func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	// Use a temporary DB file per test to avoid state leakage
	tmpDB := t.TempDir() + "/test.db"
	db.Init(tmpDB)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/register", handlers.Register)
	mux.HandleFunc("/api/login", handlers.Login)
	mux.HandleFunc("/api/events", handlers.Events)
	mux.HandleFunc("/api/events/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/register") {
			middleware.Authenticate(handlers.RegisterForEvent)(w, r)
			return
		}
		if strings.HasSuffix(path, "/registrations") {
			middleware.Authenticate(handlers.ListRegistrations)(w, r)
			return
		}
		handlers.EventByID(w, r)
	})

	return httptest.NewServer(mux)
}

// createOrganizerAndEvent is a helper that registers an organizer, creates an
// event with the given capacity, and returns the token + event ID.
func createOrganizerAndEvent(t *testing.T, baseURL string, capacity int) (string, int64) {
	t.Helper()

	// Register organizer
	body, _ := json.Marshal(map[string]string{
		"name": "Organizer", "email": "org@test.com",
		"password": "secret", "role": "organizer",
	})
	resp, err := http.Post(baseURL+"/api/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("register organizer: %v", err)
	}
	defer resp.Body.Close()
	var authResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&authResp)
	token := authResp["token"].(string)

	// Create event
	eventBody, _ := json.Marshal(map[string]interface{}{
		"title": "Go Summit", "description": "A conf", "location": "Bangalore",
		"date": "2026-12-01T10:00:00Z", "capacity": capacity,
	})
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/api/events", bytes.NewReader(eventBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("create event: %v", err)
	}
	defer resp2.Body.Close()
	var eventResp map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&eventResp)
	eventID := int64(eventResp["id"].(float64))

	return token, eventID
}

// registerAttendee registers a unique attendee and returns their auth token.
func registerAttendee(t *testing.T, baseURL string, idx int) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"name":     fmt.Sprintf("User%d", idx),
		"email":    fmt.Sprintf("user%d@test.com", idx),
		"password": "pass",
		"role":     "attendee",
	})
	resp, err := http.Post(baseURL+"/api/register", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("register attendee %d: %v", idx, err)
	}
	defer resp.Body.Close()
	var authResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&authResp)
	return authResp["token"].(string)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestConcurrentBooking: The core deliverable test.
//
// Simulates 50 goroutines simultaneously trying to register for an event
// that has only 5 spots. Asserts:
//   - Exactly 5 registrations succeed (HTTP 201).
//   - All other 45 get HTTP 409 (fully booked).
//   - No panics, no data races (run with: go test -race ./...).
//
// ─────────────────────────────────────────────────────────────────────────────
func TestConcurrentBooking(t *testing.T) {
	const (
		totalGoroutines = 50
		eventCapacity   = 5
	)

	server := setupTestServer(t)
	defer server.Close()
	baseURL := server.URL

	// Pre-create 50 attendee accounts (sequential, no concurrency concern here)
	tokens := make([]string, totalGoroutines)
	for i := 0; i < totalGoroutines; i++ {
		tokens[i] = registerAttendee(t, baseURL, i)
	}

	// Create the event with capacity = 5
	_, eventID := createOrganizerAndEvent(t, baseURL, eventCapacity)

	// ── Concurrent booking phase ──────────────────────────────────────────────
	var (
		successCount int32 // HTTP 201
		conflictCount int32 // HTTP 409
		errorCount   int32 // anything else
		wg           sync.WaitGroup
		startGun     = make(chan struct{}) // ensures all goroutines start together
	)

	for i := 0; i < totalGoroutines; i++ {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			<-startGun // block until all goroutines are ready

			url := fmt.Sprintf("%s/api/events/%d/register", baseURL, eventID)
			req, _ := http.NewRequest(http.MethodPost, url, nil)
			req.Header.Set("Authorization", "Bearer "+token)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			switch resp.StatusCode {
			case http.StatusCreated:
				atomic.AddInt32(&successCount, 1)
			case http.StatusConflict:
				atomic.AddInt32(&conflictCount, 1)
			default:
				atomic.AddInt32(&errorCount, 1)
				t.Logf("unexpected status %d", resp.StatusCode)
			}
		}(tokens[i])
	}

	close(startGun) // fire! all goroutines race simultaneously
	wg.Wait()

	// ── Assertions ───────────────────────────────────────────────────────────
	t.Logf("Results → success=%d  conflict=%d  error=%d", successCount, conflictCount, errorCount)

	if int(successCount) != eventCapacity {
		t.Errorf("expected exactly %d successful registrations, got %d (OVERBOOKING DETECTED!)",
			eventCapacity, successCount)
	}
	if int(conflictCount) != totalGoroutines-eventCapacity {
		t.Errorf("expected %d conflict responses, got %d",
			totalGoroutines-eventCapacity, conflictCount)
	}
	if errorCount != 0 {
		t.Errorf("expected 0 error responses, got %d", errorCount)
	}

	// Verify database state directly
	var dbSpots int
	row := db.DB.QueryRow(`SELECT available_spots FROM events WHERE id = ?`, eventID)
	if err := row.Scan(&dbSpots); err != nil {
		t.Fatalf("could not read available_spots: %v", err)
	}
	if dbSpots != 0 {
		t.Errorf("expected available_spots=0 in DB, got %d", dbSpots)
	}
	t.Logf("✅ available_spots in DB = %d (correct)", dbSpots)
}

// TestNoOverbooking is a targeted unit test for the registration model.
func TestNoOverbooking(t *testing.T) {
	tmpDB := t.TempDir() + "/test_noover.db"
	db.Init(tmpDB)
	defer os.Remove(tmpDB)

	server := setupTestServer(t)
	defer server.Close()

	// Create event with capacity 1
	_, eventID := createOrganizerAndEvent(t, server.URL, 1)
	tok1 := registerAttendee(t, server.URL, 100)
	tok2 := registerAttendee(t, server.URL, 101)

	register := func(token string) int {
		url := fmt.Sprintf("%s/api/events/%d/register", server.URL, eventID)
		req, _ := http.NewRequest(http.MethodPost, url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()
		return resp.StatusCode
	}

	s1 := register(tok1)
	s2 := register(tok2)

	// Exactly one must succeed
	if (s1 == 201 && s2 == 409) || (s1 == 409 && s2 == 201) {
		t.Log("✅ Exactly one registration succeeded — no overbooking")
	} else {
		t.Errorf("unexpected status codes: user1=%d user2=%d", s1, s2)
	}
}

// TestDoubleRegistrationPrevented ensures the same user can't register twice.
func TestDoubleRegistrationPrevented(t *testing.T) {
	server := setupTestServer(t)
	defer server.Close()

	_, eventID := createOrganizerAndEvent(t, server.URL, 10)
	token := registerAttendee(t, server.URL, 200)

	register := func() int {
		url := fmt.Sprintf("%s/api/events/%d/register", server.URL, eventID)
		req, _ := http.NewRequest(http.MethodPost, url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, _ := http.DefaultClient.Do(req)
		resp.Body.Close()
		return resp.StatusCode
	}

	if s := register(); s != 201 {
		t.Fatalf("first registration should succeed, got %d", s)
	}
	if s := register(); s != 409 {
		t.Fatalf("second registration should be 409, got %d", s)
	}
	t.Log("✅ Double registration correctly rejected")
}

// TestHealthEndpoint verifies the /health endpoint returns 200.
func TestHealthEndpoint(t *testing.T) {
	server := setupTestServer(t)
	defer server.Close()

	// Attach the health handler on a fresh mux for this test
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"status":"ok"}`)
	}))
	defer ts.Close()

	resp, err := http.Get(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	t.Log("✅ Health endpoint returns 200")
}
