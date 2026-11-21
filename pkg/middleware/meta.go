package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type metaKeyType struct{}

var metaKey metaKeyType

// RequestMeta carries facts learned while a request is handled (who made it,
// which sign-in subject it concerned) back out to the activity middleware,
// which wraps the handler and so cannot see values set deeper in the stack.
type RequestMeta struct {
	UserID  string
	Subject string
}

// WithRequestMeta attaches an empty RequestMeta to the context.
func WithRequestMeta(ctx context.Context) (context.Context, *RequestMeta) {
	m := &RequestMeta{}
	return context.WithValue(ctx, metaKey, m), m
}

// SetUserID records the authenticated user for the current request.
func SetUserID(ctx context.Context, id string) {
	if m, ok := ctx.Value(metaKey).(*RequestMeta); ok {
		m.UserID = id
	}
}

// SetSubject records the hashed sign-in subject for the current request.
func SetSubject(ctx context.Context, subject string) {
	if m, ok := ctx.Value(metaKey).(*RequestMeta); ok {
		m.Subject = subject
	}
}

// SubjectHash returns a short one-way hash of an email address, used to count
// repeated sign-in attempts without storing the address.
func SubjectHash(email string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(sum[:])[:32]
}
