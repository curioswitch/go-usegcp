package firebaseauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"firebase.google.com/go/v4/auth"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestMiddleware(t *testing.T) {
	tests := []struct {
		name          string
		authorization string

		status     int
		body       string
		nextCalled bool
		tokenUID   string
		rawToken   string
	}{
		{
			name:          "success",
			authorization: "Bearer valid-token",
			status:        http.StatusOK,
			body:          "success",
			nextCalled:    true,
			tokenUID:      "userid",
			rawToken:      "valid-token",
		},
		{
			name:   "missing header",
			status: http.StatusForbidden,
			body:   "missing authorization header\n",
		},
		{
			name:          "malformed header",
			authorization: "valid-token",
			status:        http.StatusForbidden,
			body:          "malformed authorization header\n",
		},
		{
			name:          "invalid",
			authorization: "Bearer invalid-token",
			status:        http.StatusForbidden,
			body:          "invalid token\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nextCalled := false
			nextCtx := t.Context()

			next := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				nextCalled = true
				nextCtx = req.Context() //nolint:fatcontext // not creating a new context

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("success"))
			})

			fbAuth := NewMockfirebaseAuth(t)

			fbAuth.EXPECT().
				VerifyIDToken(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, s string) (*auth.Token, error) {
					switch s {
					case "valid-token":
						return &auth.Token{UID: tc.tokenUID}, nil
					default:
						return nil, errors.New("invalid signature")
					}
				}).
				Maybe()

			h := &handler{next: next, fbAuth: fbAuth}

			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.Header.Set("Authorization", tc.authorization)

			res := httptest.NewRecorder()

			h.ServeHTTP(res, req)

			require.Equal(t, tc.status, res.Code)
			require.Equal(t, tc.body, res.Body.String())
			require.Equal(t, tc.nextCalled, nextCalled)

			if tc.tokenUID != "" {
				require.Equal(t, tc.tokenUID, TokenFromContext(nextCtx).UID)
			} else {
				require.Nil(t, TokenFromContext(nextCtx))
			}

			if tc.rawToken != "" {
				require.Equal(t, tc.rawToken, RawTokenFromContext(nextCtx))
			} else {
				require.Empty(t, RawTokenFromContext(nextCtx))
			}
		})
	}
}
