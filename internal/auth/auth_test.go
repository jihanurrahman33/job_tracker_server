package auth_test

import (
	"context"
	"testing"
	"time"

	"job-tracker/internal/auth"
	"job-tracker/internal/user"
)

type mockUserRepo struct {
	users   map[string]*user.User
	byEmail map[string]*user.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		users:   make(map[string]*user.User),
		byEmail: make(map[string]*user.User),
	}
}

func (m *mockUserRepo) Create(ctx context.Context, u *user.User) error {
	u.ID = "test-user-id"
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()
	m.users[u.ID] = u
	m.byEmail[u.Email] = u
	return nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*user.User, error) {
	return m.users[id], nil
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	return m.byEmail[email], nil
}

func TestPasswordHashing(t *testing.T) {
	password := "SecretPass123!"

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if !auth.CheckPassword(password, hash) {
		t.Errorf("expected password to match hash")
	}

	if auth.CheckPassword("WrongPassword", hash) {
		t.Errorf("expected wrong password to fail verification")
	}
}

func TestMemorySessionStore(t *testing.T) {
	store := auth.NewMemorySessionStore(100 * time.Millisecond)
	ctx := context.Background()

	token, err := store.CreateSession(ctx, "user-123")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	userID, err := store.GetUserIDBySession(ctx, token)
	if err != nil || userID != "user-123" {
		t.Fatalf("expected userID 'user-123', got %q (err: %v)", userID, err)
	}

	// Test deletion
	_ = store.DeleteSession(ctx, token)
	_, err = store.GetUserIDBySession(ctx, token)
	if err == nil {
		t.Errorf("expected error after deleting session, got nil")
	}
}

func TestAuthService_RegisterAndLogin(t *testing.T) {
	repo := newMockUserRepo()
	sessions := auth.NewMemorySessionStore(1 * time.Hour)
	service := auth.NewService(repo, sessions)
	ctx := context.Background()

	// Register
	u, token, err := service.Register(ctx, "Alice", "alice@example.com", "securepassword")
	if err != nil {
		t.Fatalf("failed to register: %v", err)
	}
	if u.Email != "alice@example.com" || token == "" {
		t.Fatalf("unexpected register result: %+v, token: %s", u, token)
	}

	// Register duplicate email
	_, _, err = service.Register(ctx, "Alice 2", "alice@example.com", "securepassword")
	if err != auth.ErrUserAlreadyExists {
		t.Errorf("expected ErrUserAlreadyExists, got %v", err)
	}

	// Login with correct password
	loginUser, loginToken, err := service.Login(ctx, "alice@example.com", "securepassword")
	if err != nil {
		t.Fatalf("failed to login: %v", err)
	}
	if loginUser.ID != u.ID || loginToken == "" {
		t.Fatalf("unexpected login result: %+v", loginUser)
	}

	// Login with wrong password
	_, _, err = service.Login(ctx, "alice@example.com", "wrongpass")
	if err != auth.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}
