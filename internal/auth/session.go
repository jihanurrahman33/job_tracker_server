package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"job-tracker/internal/database"
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

	_, err := p.db.ExecContext(ctx, query, token, userID, expiresAt)
	if err != nil {
		return "", fmt.Errorf("failed to create session in database: %w", err)
	}

	return token, nil
}

func (p *PostgresSessionStore) GetUserIDBySession(ctx context.Context, sessionID string) (string, error) {
	query := `
		SELECT user_id, expires_at
		FROM sessions
		WHERE id = $1`

	var userID string
	var expiresAt time.Time

	err := p.db.QueryRowContext(ctx, query, sessionID).Scan(&userID, &expiresAt)
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
	_, err := p.db.ExecContext(ctx, query, sessionID)
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
