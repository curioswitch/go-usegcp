package firebaseauth

import (
	"context"

	"firebase.google.com/go/v4/auth"
)

type firebaseUserContextKeyType struct{}

var firebaseUserContextKey firebaseUserContextKeyType = struct{}{}

type firebaseUserHolder struct {
	user  *auth.Token
	token string
}

// TokenFromContext returns the auth.Token contained in ctx, if any.
// Note, this is not the string JWT token but the decoded token. Use
// RawTokenFromContext to get the JWT token.
func TokenFromContext(ctx context.Context) *auth.Token {
	if h, ok := ctx.Value(firebaseUserContextKey).(*firebaseUserHolder); ok {
		return h.user
	}
	return nil
}

// RawTokenFromContext returns the raw JWT token string contained in ctx, if any.
// To get the decoded token, use TokenFromContext.
func RawTokenFromContext(ctx context.Context) string {
	if h, ok := ctx.Value(firebaseUserContextKey).(*firebaseUserHolder); ok {
		return h.token
	}
	return ""
}
