package firebaseauth

import (
	"context"
	"net/http"
	"strings"

	"firebase.google.com/go/v4/auth"
)

// NewMiddleware returns an http.Handler middleware that authenticates requests
// using Firebase Authentication ID tokens. Requests with a valid ID token as the
// bearer token in the Authorization header will proceed, with the token content
// accessible from context.Context via TokenFromContext or RawTokenFromContext.
// Requests without a valid ID token will be rejected with a 403 Forbidden
// response.
func NewMiddleware(fbAuth *auth.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return &handler{
			next:   next,
			fbAuth: fbAuth,
		}
	}
}

type firebaseAuth interface {
	VerifyIDToken(ctx context.Context, idToken string) (*auth.Token, error)
}

type handler struct {
	next http.Handler

	fbAuth firebaseAuth
}

// ServeHTTP implements http.Handler.
func (m *handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	hdr := req.Header.Get("Authorization")
	if hdr == "" {
		http.Error(w, "missing authorization header", http.StatusForbidden)
		return
	}

	token, ok := strings.CutPrefix(hdr, "Bearer ")
	if !ok {
		http.Error(w, "malformed authorization header", http.StatusForbidden)
		return
	}

	decoded, err := m.fbAuth.VerifyIDToken(req.Context(), token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusForbidden)
		return
	}

	req = req.WithContext(context.WithValue(req.Context(), firebaseUserContextKey, &firebaseUserHolder{
		user:  decoded,
		token: token,
	}))

	m.next.ServeHTTP(w, req)
}
