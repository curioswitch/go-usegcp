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

		status int
		body   string
	}{
		{
			name:          "success",
			authorization: "Bearer valid-token",
			status:        http.StatusOK,
			body:          "success",
		},
		{
			name:   "missing header",
			status: http.StatusUnauthorized,
			body:   "missing authorization header\n",
		},
		{
			name:          "malformed header",
			authorization: "valid-token",
			status:        http.StatusUnauthorized,
			body:          "malformed authorization header\n",
		},
		{
			name:          "invalid",
			authorization: "Bearer invalid-token",
			status:        http.StatusUnauthorized,
			body:          "invalid token\n",
		},
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	for _, tc := range tests {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			fbAuth := NewMockfirebaseAuth(t)

			fbAuth.EXPECT().
				VerifyIDToken(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, s string) (*auth.Token, error) {
					switch {
					case s == "valid-token":
						return &auth.Token{UID: "userid"}, nil
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
		})
	}
}
