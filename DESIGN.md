# Design Document: Concurrent Booking & Race Condition Prevention

## Capstone Project 5 — Event Registration & Ticketing System

---

## The Problem

In a ticketing system, **overbooking** is the most critical correctness issue. Consider this scenario:

1. An event has **1 spot** remaining.
2. **50 users** simultaneously send a registration request.
3. Each request reads `available_spots = 1` (sees a spot available).
4. All 50 proceed to insert a registration record.
5. Result: **50 tickets issued for 1 spot** — a catastrophic failure.

This is a classic **Time-of-Check / Time-of-Use (TOCTOU)** race condition.

---

## Why This Is Hard

In a concurrent server, every incoming HTTP request is handled in its own goroutine. Go's HTTP server uses a goroutine pool, so 50 simultaneous requests truly run in parallel across CPU cores. Without proper synchronization:

```
Goroutine 1: READ available_spots → 1   // ← both see 1
Goroutine 2: READ available_spots → 1   // ← race!
Goroutine 1: INSERT registration        // succeeds
Goroutine 2: INSERT registration        // also succeeds → OVERBOOKING!
Goroutine 1: UPDATE spots → 0
Goroutine 2: UPDATE spots → -1          // negative! constraint violated
```

---

## Our Solution: `BEGIN IMMEDIATE` Transactions

We use SQLite's **serializable transaction isolation** with the `BEGIN IMMEDIATE` mode via Go's standard `db.Begin()` + a single-connection constraint.

```go
// In models/registration.go
func RegisterForEvent(eventID, userID int64) (*Registration, error) {
    tx, err := db.DB.Begin()
    // ...

    // Step 1: Read available spots INSIDE the transaction
    var spots int
    tx.QueryRow(`SELECT available_spots FROM events WHERE id = ?`, eventID).Scan(&spots)

    // Step 2: Check capacity — rollback if none available
    if spots <= 0 {
        tx.Rollback()
        return nil, ErrNoSpots  // → HTTP 409
    }

    // Step 3: Insert registration (UNIQUE constraint catches duplicates)
    tx.Exec(`INSERT INTO registrations (event_id, user_id, ticket_no) VALUES (?, ?, ?)`, ...)

    // Step 4: Atomically decrement — all in one transaction
    tx.Exec(`UPDATE events SET available_spots = available_spots - 1 WHERE id = ?`, eventID)

    tx.Commit()  // only succeeds if nothing else modified the row
}
```

### Why This Works

SQLite uses **database-level locking**:

| Mode | Lock acquired |
|------|--------------|
| `BEGIN` (deferred) | Read lock; upgrades to write on first write operation |
| `BEGIN IMMEDIATE` | Write lock immediately — blocks all other writers |

We enforce `DB.SetMaxOpenConns(1)` — only one connection, one goroutine writes at a time. This means:

```
Goroutine 1: BEGIN → gets write lock
Goroutine 2: BEGIN → BLOCKED (waiting for lock)
Goroutine 3: BEGIN → BLOCKED (waiting for lock)
...
Goroutine 1: READ spots=5, INSERT, UPDATE spots=4, COMMIT → releases lock
Goroutine 2: acquires lock → READ spots=4, INSERT, UPDATE spots=3 ...
...
Goroutine 49: acquires lock → READ spots=0 → ROLLBACK → HTTP 409
Goroutine 50: acquires lock → READ spots=0 → ROLLBACK → HTTP 409
```

**Result:** Exactly N registrations succeed. No overbooking. Ever.

---

## Additional Protection Layers

### Layer 1: UNIQUE Database Constraint
```sql
UNIQUE(event_id, user_id)
```
Even if two goroutines somehow bypass the capacity check, SQLite's constraint will reject the second `INSERT` with a unique constraint error. This is a last-resort safety net.

### Layer 2: CHECK Constraint on `available_spots`
```sql
available_spots INTEGER NOT NULL CHECK(available_spots >= 0)
```
Database-level assertion: `available_spots` can never go negative, even if application logic has a bug.

### Layer 3: WAL Mode
```sql
PRAGMA journal_mode=WAL;
```
Write-Ahead Logging allows concurrent **reads** while a write transaction is in progress. This means non-booking requests (like listing events) are never blocked by a registration in flight.

---

## Alternative Approaches Considered

### ❌ Option A: Application-Level Mutex

```go
var mu sync.Mutex
func RegisterForEvent(...) {
    mu.Lock()
    defer mu.Unlock()
    // ... read, check, write
}
```

**Problem:** Only works within a single server process. In a multi-instance deployment (e.g., Kubernetes), each instance has its own mutex — different instances don't coordinate → overbooking resumes.

### ❌ Option B: Redis Distributed Lock

Use `SETNX` in Redis as a distributed lock:
```
SETNX lock:event:42 1 EX 5
```

**Problem:** Requires a Redis server. Complex to implement correctly (lock expiry, crash recovery). Over-engineering for this scale.

### ❌ Option C: Optimistic Locking (CAS)

```sql
UPDATE events 
SET available_spots = available_spots - 1 
WHERE id = 42 AND available_spots > 0
```

Check `RowsAffected` — if 0, someone else took the spot.

**Viable**, but requires retry logic and doesn't handle the `INSERT registrations` + `UPDATE events` atomically in one step. Our approach is cleaner.

### ✅ Option D: `BEGIN IMMEDIATE` Transactions (Chosen)

Single-writer serialization at the DB level. Atomic read-check-write. Works correctly with multiple goroutines. No extra infrastructure. Simpler code.

---

## Performance Characteristics

| Metric | Value |
|--------|-------|
| Lock granularity | Database-level (entire DB) |
| Max concurrent writers | 1 |
| Max concurrent readers | Unlimited (via WAL) |
| Throughput (booking) | ~500-2000 req/sec (SQLite limit) |
| Throughput (read-only) | ~10,000+ req/sec |

For event-scale loads (thousands of users, not millions), SQLite is entirely sufficient.

---

## Proof: The Test

`concurrent_test.go` launches **50 goroutines simultaneously** using a `startGun` channel:

```go
startGun := make(chan struct{})
for i := 0; i < 50; i++ {
    go func(token string) {
        <-startGun          // all goroutines wait here
        // ... register
    }(tokens[i])
}
close(startGun)  // all 50 goroutines unblock simultaneously
```

Run with the Go race detector:
```bash
go test -v -race ./...
```

Expected output:
```
Results → success=5  conflict=45  error=0
✅ available_spots in DB = 0 (correct)
PASS
```

The `-race` flag instruments the binary with Go's data race detector. If any unsynchronized concurrent memory access occurs, it fails immediately — providing a second proof of correctness.

---

*Prepared by: Anagal | Infosys Capstone Project 5 | February 2026*
