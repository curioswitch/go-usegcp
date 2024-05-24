package gcpslog

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

// logMsg is a helper function with a highly stable line number.
func logMsg(ctx context.Context, l *slog.Logger, level slog.Level, message string) {
	l.Log(ctx, level, message, slog.String("animal", "bear"))
}

type sourceRecord struct {
	File     string `json:"file"`
	Line     string `json:"line"`
	Function string `json:"function"`
}

type logRecord struct {
	Message   string       `json:"message"`
	Severity  string       `json:"severity"`
	Timestamp string       `json:"timestamp"`
	TraceID   string       `json:"logging.googleapis.com/trace"`
	SpanID    string       `json:"logging.googleapis.com/spanId"`
	Sampled   bool         `json:"logging.googleapis.com/trace_sampled"`
	Source    sourceRecord `json:"logging.googleapis.com/sourceLocation"`
	Animal    string       `json:"animal"`
}

func TestNewHandler(t *testing.T) {
	traceID, _ := trace.TraceIDFromHex("01020304010203040102030401020304")
	spanID, _ := trace.SpanIDFromHex("0102030401020304")

	tests := []struct {
		name        string
		ctx         context.Context
		message     string
		level       slog.Level
		source      bool
		replaceAttr func([]string, slog.Attr) slog.Attr

		logged logRecord
	}{
		{
			name:    "info no span",
			ctx:     context.Background(),
			message: "normal log",
			level:   slog.LevelInfo,

			logged: logRecord{
				Message:  "normal log",
				Severity: "INFO",
				Animal:   "bear",
			},
		},
		{
			name:    "error no span",
			ctx:     context.Background(),
			message: "bad log",
			level:   slog.LevelError,

			logged: logRecord{
				Message:  "bad log",
				Severity: "ERROR",
				Animal:   "bear",
			},
		},
		{
			name: "info with span",
			ctx: trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
				TraceID:    traceID,
				SpanID:     spanID,
				TraceFlags: trace.FlagsSampled,
			})),
			message: "normal log",
			level:   slog.LevelInfo,

			logged: logRecord{
				Message:  "normal log",
				Severity: "INFO",
				TraceID:  "01020304010203040102030401020304",
				SpanID:   "0102030401020304",
				Sampled:  true,
				Animal:   "bear",
			},
		},
		{
			name: "error with span and source",
			ctx: trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
				TraceID:    traceID,
				SpanID:     spanID,
				TraceFlags: trace.FlagsSampled,
			})),
			message: "error log",
			level:   slog.LevelError,
			source:  true,

			logged: logRecord{
				Message:  "error log",
				Severity: "ERROR",
				TraceID:  "01020304010203040102030401020304",
				SpanID:   "0102030401020304",
				Sampled:  true,
				Animal:   "bear",
				Source: sourceRecord{
					File:     "slog_test.go",
					Line:     "17",
					Function: "github.com/curioswitch/go-usegcp/gcpslog.logMsg",
				},
			},
		},
		{
			name:    "info replace attr",
			ctx:     context.Background(),
			message: "normal log",
			level:   slog.LevelInfo,
			replaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if groups == nil || a.Key != "animal" {
					return a
				}

				return slog.String("animal", "cat")
			},

			logged: logRecord{
				Message:  "normal log",
				Severity: "INFO",
				Animal:   "cat",
			},
		},
	}

	for _, tc := range tests {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer

			opts := []Option{}
			if tc.source {
				opts = append(opts, AddSource())
			}
			if tc.replaceAttr != nil {
				opts = append(opts, ReplaceAttr(tc.replaceAttr))
			}

			h := NewHandler(&out, opts...)
			l := slog.New(h)

			logMsg(tc.ctx, l, tc.level, tc.message)

			var rec logRecord
			require.NoError(t, json.Unmarshal(out.Bytes(), &rec))

			require.NotEmpty(t, rec.Timestamp)
			rec.Timestamp = ""
			if s := rec.Source.File; s != "" {
				rec.Source.File = filepath.Base(s)
			}

			require.Equal(t, tc.logged, rec)
		})
	}
}
