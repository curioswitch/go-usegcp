package testutil

import (
	"context"

	"firebase.google.com/go/v4/auth"

	"github.com/curioswitch/go-usegcp/middleware/firebaseauth/internal/contextholder"
)

// ContextWithToken returns a new context with the given Firebase token set. This is only
// useful in unit tests, for example of handlers that use the Firebase token within a
// server using the firebaseauth middleware.
func ContextWithToken(ctx context.Context, token *auth.Token, rawToken string) context.Context {
	return context.WithValue(
		ctx,
		contextholder.FirebaseTokenContextKey,
		&contextholder.FirebaseTokenHolder{
			Token:    token,
			RawToken: rawToken,
		})
}
