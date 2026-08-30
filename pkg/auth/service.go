package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"job-tracker/pkg/user"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserAlreadyExists  = errors.New("email is already registered")
)

const (
	pbkdf2Iterations = 100000
	saltLength       = 16
	keyLength        = 32
)

// Service provides authentication workflows.
type Service struct {
	userRepo user.Repository
	sessions SessionStore
}

// NewService creates a new Auth Service.
func NewService(userRepo user.Repository, sessions SessionStore) *Service {
	return &Service{
		userRepo: userRepo,
		sessions: sessions,
	}
}

// Register creates a new user account and returns a session token.
func (s *Service) Register(ctx context.Context, name, email, password string) (*user.User, string, error) {
	existing, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, "", err
	}
	if existing != nil {
		return nil, "", ErrUserAlreadyExists
	}

	hash, err := HashPassword(password)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash password: %w", err)
	}

	newUser := &user.User{
		Name:         name,
		Email:        email,
		PasswordHash: hash,
	}

	if err := s.userRepo.Create(ctx, newUser); err != nil {
		return nil, "", fmt.Errorf("failed to create user: %w", err)
	}

	token, err := s.sessions.CreateSession(ctx, newUser.ID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create session: %w", err)
	}

	return newUser, token, nil
}

// Login authenticates a user and returns a session token.
func (s *Service) Login(ctx context.Context, email, password string) (*user.User, string, error) {
	u, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, "", err
	}
	if u == nil {
		return nil, "", ErrInvalidCredentials
	}

	if !CheckPassword(password, u.PasswordHash) {
		return nil, "", ErrInvalidCredentials
	}

	token, err := s.sessions.CreateSession(ctx, u.ID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create session: %w", err)
	}

	return u, token, nil
}

// Logout deletes the active session token.
func (s *Service) Logout(ctx context.Context, token string) error {
	return s.sessions.DeleteSession(ctx, token)
}

// ValidateToken verifies a session token and returns the corresponding userID.
func (s *Service) ValidateToken(ctx context.Context, token string) (string, error) {
	return s.sessions.GetUserIDBySession(ctx, token)
}

// HashPassword generates a salted PBKDF2-SHA256 password hash.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := pbkdf2SHA256([]byte(password), salt, pbkdf2Iterations, keyLength)
	return fmt.Sprintf("pbkdf2_sha256$%d$%s$%s", pbkdf2Iterations, hex.EncodeToString(salt), hex.EncodeToString(hash)), nil
}

// CheckPassword securely verifies a password against an encoded hash.
func CheckPassword(password, encodedHash string) bool {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 4 || parts[0] != "pbkdf2_sha256" {
		return false
	}

	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false
	}

	salt, err := hex.DecodeString(parts[2])
	if err != nil {
		return false
	}

	expectedHash, err := hex.DecodeString(parts[3])
	if err != nil {
		return false
	}

	computedHash := pbkdf2SHA256([]byte(password), salt, iterations, len(expectedHash))
	return subtle.ConstantTimeCompare(computedHash, expectedHash) == 1
}

// Standard library PBKDF2 implementation using crypto/sha256
func pbkdf2SHA256(password, salt []byte, iter, keyLen int) []byte {
	prf := func(key, msg []byte) []byte {
		h := sha256.New()
		h.Write(key)
		h.Write(msg)
		return h.Sum(nil)
	}

	hLen := 32 // SHA-256 output length
	numBlocks := (keyLen + hLen - 1) / hLen
	var result []byte

	for block := 1; block <= numBlocks; block++ {
		// U_1 = PRF(Password, Salt || INT_32_BE(block))
		var blockBytes [4]byte
		blockBytes[0] = byte(block >> 24)
		blockBytes[1] = byte(block >> 16)
		blockBytes[2] = byte(block >> 8)
		blockBytes[3] = byte(block)

		u := prf(password, append(salt, blockBytes[:]...))
		xorSum := make([]byte, len(u))
		copy(xorSum, u)

		for i := 1; i < iter; i++ {
			u = prf(password, u)
			for j := 0; j < len(xorSum); j++ {
				xorSum[j] ^= u[j]
			}
		}

		result = append(result, xorSum...)
	}

	return result[:keyLen]
}
