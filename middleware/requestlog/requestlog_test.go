package requestlog

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type gcpRequest struct {
	RequestMethod string `json:"requestMethod,omitempty"`
	RequestURL    string `json:"requestUrl,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
	Status        int    `json:"status,omitempty"`
	UserAgent     string `json:"userAgent,omitempty"`
	RemoteIP      string `json:"remoteIp,omitempty"`
	ResponseSize  int64  `json:"responseSize,omitempty"`
	Latency       string `json:"latency,omitempty"`
}

type gcpRecord struct {
	HTTPRequest *gcpRequest `json:"httpRequest,omitempty"`
	StackTrace  string      `json:"stack_trace,omitempty"`

	// Extra attributes for testing.
	Animal string `json:"animal,omitempty"`
}

func TestMiddleware(t *testing.T) {
	tests := []struct {
		name string
		req  *http.Request
		next http.Handler

		record   *gcpRecord
		panicErr error
	}{
		{
			name: "normal",
			req:  httptest.NewRequest(http.MethodGet, "/", nil),
			next: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),

			record: &gcpRecord{
				HTTPRequest: &gcpRequest{
					RequestMethod: http.MethodGet,
					RequestURL:    "/",
					Protocol:      "HTTP/1.1",
					Status:        http.StatusOK,
					RemoteIP:      "192.0.2.1:1234",
					ResponseSize:  0,
				},
			},
		},
		{
			name: "user agent",
			req: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				req.Header.Set("User-Agent", "curioswitch")
				return req
			}(),
			next: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),

			record: &gcpRecord{
				HTTPRequest: &gcpRequest{
					RequestMethod: http.MethodGet,
					RequestURL:    "/",
					Protocol:      "HTTP/1.1",
					UserAgent:     "curioswitch",
					Status:        http.StatusOK,
					RemoteIP:      "192.0.2.1:1234",
					ResponseSize:  0,
				},
			},
		},
		{
			name: "response body",
			req:  httptest.NewRequest(http.MethodPost, "/bear", strings.NewReader("who")),
			next: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("kuma"))
			}),

			record: &gcpRecord{
				HTTPRequest: &gcpRequest{
					RequestMethod: http.MethodPost,
					RequestURL:    "/bear",
					Protocol:      "HTTP/1.1",
					Status:        http.StatusOK,
					RemoteIP:      "192.0.2.1:1234",
					ResponseSize:  4,
				},
			},
		},
		{
			name: "extra attrs",
			req:  httptest.NewRequest(http.MethodGet, "/", nil),
			next: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.WriteHeader(http.StatusOK)
				AddExtraAttr(req.Context(), slog.String("animal", "bear"))
			}),

			record: &gcpRecord{
				HTTPRequest: &gcpRequest{
					RequestMethod: http.MethodGet,
					RequestURL:    "/",
					Protocol:      "HTTP/1.1",
					Status:        http.StatusOK,
					RemoteIP:      "192.0.2.1:1234",
					ResponseSize:  0,
				},
				Animal: "bear",
			},
		},
		{
			name: "error status",
			req:  httptest.NewRequest(http.MethodPut, "/error", nil),
			next: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}),

			record: &gcpRecord{
				HTTPRequest: &gcpRequest{
					RequestMethod: http.MethodPut,
					RequestURL:    "/error",
					Protocol:      "HTTP/1.1",
					Status:        http.StatusInternalServerError,
					RemoteIP:      "192.0.2.1:1234",
					ResponseSize:  0,
				},
			},
		},
		{
			name: "panic",
			req:  httptest.NewRequest(http.MethodGet, "/", nil),
			next: http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				panic(errors.New("failure"))
			}),

			record: &gcpRecord{
				HTTPRequest: &gcpRequest{
					RequestMethod: http.MethodGet,
					RequestURL:    "/",
					Status:        500,
					Protocol:      "HTTP/1.1",
					RemoteIP:      "192.0.2.1:1234",
					ResponseSize:  0,
				},
				StackTrace: "github.com/curioswitch/go-usegcp/middleware/requestlog.TestMiddleware",
			},
			panicErr: errors.New("failure"),
		},
		{
			name: "panic with non-error status",
			req:  httptest.NewRequest(http.MethodGet, "/", nil),
			next: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusFound)
				panic(errors.New("failure"))
			}),

			record: &gcpRecord{
				HTTPRequest: &gcpRequest{
					RequestMethod: http.MethodGet,
					RequestURL:    "/",
					Status:        500,
					Protocol:      "HTTP/1.1",
					RemoteIP:      "192.0.2.1:1234",
					ResponseSize:  0,
				},
				StackTrace: "github.com/curioswitch/go-usegcp/middleware/requestlog.TestMiddleware",
			},
			panicErr: errors.New("failure"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var output bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{}))
			w := httptest.NewRecorder()

			mw := NewMiddleware(Logger(logger))
			h := mw(tc.next)

			if tc.panicErr != nil {
				require.PanicsWithError(t, tc.panicErr.Error(), func() {
					h.ServeHTTP(w, tc.req)
				})
			} else {
				mw(tc.next).ServeHTTP(w, tc.req)
			}

			require.NotEmpty(t, output.String())

			rec := &gcpRecord{}
			require.NoError(t, json.Unmarshal(output.Bytes(), rec))
			latencyStr := rec.HTTPRequest.Latency
			require.True(t, strings.HasSuffix(latencyStr, "s"))
			latency, err := strconv.ParseFloat(strings.TrimSuffix(latencyStr, "s"), 64)
			require.NoError(t, err)
			// Generally would use Greater but with low precision clock like on Windows,
			// can be equal and it's better than slowing down the test with a sleep.
			require.GreaterOrEqual(t, latency, 0.0)
			rec.HTTPRequest.Latency = ""

			if tc.record.StackTrace == "" {
				require.Empty(t, rec.StackTrace)
			} else {
				require.Contains(t, rec.StackTrace, tc.record.StackTrace)
				tc.record.StackTrace = ""
				rec.StackTrace = ""
			}

			require.Equal(t, tc.record, rec)
		})
	}
}
