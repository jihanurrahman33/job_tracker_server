package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"job-tracker/pkg/database"
)

var ErrInvalidSession = errors.New("invalid or expired session")

// SessionStore defines the interface for session storage.
type SessionStore interface {
	CreateSession(ctx context.Context, userID string) (string, error)
	GetUserIDBySession(ctx context.Context, sessionID string) (string, error)
	DeleteSession(ctx context.Context, sessionID string) error
}

// PostgresSessionStore implements SessionStore using PostgreSQL database.
type PostgresSessionStore struct {
	db  *database.DB
	ttl time.Duration
}

// NewPostgresSessionStore creates a new PostgreSQL-backed session manager.
func NewPostgresSessionStore(db *database.DB, ttl time.Duration) *PostgresSessionStore {
	return &PostgresSessionStore{
		db:  db,
		ttl: ttl,
	}
}

func (p *PostgresSessionStore) CreateSession(ctx context.Context, userID string) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)
	expiresAt := time.Now().Add(p.ttl)

	query := `
		INSERT INTO sessions (id, user_id, expires_at, created_at)
		VALUES ($1, $2, $3, NOW())`

	// Only a hash of the token is stored, so a copy of the database cannot be
	// used to sign in as anyone.
	_, err := p.db.ExecContext(ctx, query, hashToken(token), userID, expiresAt)
	if err != nil {
		return "", fmt.Errorf("failed to create session in database: %w", err)
	}

	return token, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (p *PostgresSessionStore) GetUserIDBySession(ctx context.Context, sessionID string) (string, error) {
	// A disabled account's sessions stop working immediately.
	query := `
		SELECT s.user_id, s.expires_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.id = $1 AND u.disabled = FALSE`

	var userID string
	var expiresAt time.Time

	err := p.db.QueryRowContext(ctx, query, hashToken(sessionID)).Scan(&userID, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrInvalidSession
		}
		return "", fmt.Errorf("failed to query session: %w", err)
	}

	if time.Now().After(expiresAt) {
		// Clean up expired session
		_ = p.DeleteSession(ctx, sessionID)
		return "", ErrInvalidSession
	}

	return userID, nil
}

func (p *PostgresSessionStore) DeleteSession(ctx context.Context, sessionID string) error {
	query := `DELETE FROM sessions WHERE id = $1`
	_, err := p.db.ExecContext(ctx, query, hashToken(sessionID))
	return err
}

// MemorySessionStore provides an in-memory thread-safe session store for local/testing use.
type MemorySessionStore struct {
	mu       sync.RWMutex
	sessions map[string]sessionData
	ttl      time.Duration
}

type sessionData struct {
	userID    string
	expiresAt time.Time
}

// NewMemorySessionStore creates a new in-memory session manager.
func NewMemorySessionStore(ttl time.Duration) *MemorySessionStore {
	return &MemorySessionStore{
		sessions: make(map[string]sessionData),
		ttl:      ttl,
	}
}

func (m *MemorySessionStore) CreateSession(ctx context.Context, userID string) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.sessions[token] = sessionData{
		userID:    userID,
		expiresAt: time.Now().Add(m.ttl),
	}

	return token, nil
}

func (m *MemorySessionStore) GetUserIDBySession(ctx context.Context, sessionID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, exists := m.sessions[sessionID]
	if !exists || time.Now().After(data.expiresAt) {
		return "", ErrInvalidSession
	}

	return data.userID, nil
}

func (m *MemorySessionStore) DeleteSession(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, sessionID)
	return nil
}
