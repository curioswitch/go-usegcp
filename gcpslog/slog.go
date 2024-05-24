package gcpslog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"

	"go.opentelemetry.io/otel/trace"
	"golang.org/x/oauth2/google"
)

// WrapHandler wraps a [slog.Handler], formatting default fields to follow
// GCP's format and adding trace context attributes to every logged record.
// This should be used with [slog.JSONHandler] to allow proper rendering of
// log messages and for traces and logs to be linked together in the GCP console.
func NewHandler(w io.Writer, opts ...Option) slog.Handler {
	var conf config
	for _, o := range opts {
		o.apply(&conf)
	}
	userRA := conf.options.ReplaceAttr
	conf.options.ReplaceAttr = func(groups []string, a slog.Attr) slog.Attr {
		if groups == nil {
			switch a.Key {
			case slog.LevelKey:
				return slog.Attr{Key: "severity", Value: a.Value}
			case slog.MessageKey:
				return slog.Attr{Key: "message", Value: a.Value}
			case slog.TimeKey:
				return slog.Attr{Key: "timestamp", Value: a.Value}
			case slog.SourceKey:
				val := a.Value.Any().(*slog.Source)
				return slog.Group("logging.googleapis.com/sourceLocation",
					slog.String("file", val.File),
					slog.String("line", strconv.Itoa(val.Line)),
					slog.String("function", val.Function),
				)
			}
		}

		if userRA != nil {
			return userRA(groups, a)
		}

		return a
	}

	delegate := slog.NewJSONHandler(w, &conf.options)

	creds, err := google.FindDefaultCredentials(context.Background())
	tracePrefix := "projects/unknown/traces/"
	if err == nil && creds.ProjectID != "" {
		tracePrefix = fmt.Sprintf("projects/%s/traces/", creds.ProjectID)
	}

	return otelLogHandler{
		delegate:    delegate,
		tracePrefix: tracePrefix,
	}
}

type otelLogHandler struct {
	delegate    slog.Handler
	tracePrefix string
}

var _ slog.Handler = otelLogHandler{}

// Enabled implements slog.Handler.
func (h otelLogHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.delegate.Enabled(ctx, l)
}

// Handle implements slog.Handler.
func (h otelLogHandler) Handle(ctx context.Context, r slog.Record) error {
	// We don't check existing attributes since it is extremely unlikely
	// a user would set them manually.
	if sctx := trace.SpanContextFromContext(ctx); sctx.IsValid() {
		r.AddAttrs(
			slog.String("logging.googleapis.com/trace", h.tracePrefix+sctx.TraceID().String()),
			slog.String("logging.googleapis.com/spanId", sctx.SpanID().String()),
			slog.Bool("logging.googleapis.com/trace_sampled", sctx.IsSampled()),
		)
	}

	return h.delegate.Handle(ctx, r)
}

// WithAttrs implements slog.Handler.
func (h otelLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return otelLogHandler{
		delegate: h.delegate.WithAttrs(attrs),
	}
}

// WithGroup implements slog.Handler.
func (h otelLogHandler) WithGroup(name string) slog.Handler {
	return otelLogHandler{
		delegate: h.delegate.WithGroup(name),
	}
}

type config struct {
	options slog.HandlerOptions
}

// Option is a configuration option for NewHandler.
type Option interface {
	apply(conf *config)
}

// Level returns an Option to set the minimum log level that will be logged.
// The handler discards records with lower levels.
// If Level is nil, the handler assumes LevelInfo.
// The handler calls Level.Level for each record processed;
// to adjust the minimum level dynamically, use a [slog.LevelVar].
func Level(l slog.Level) Option {
	return levelOption(l)
}

type levelOption slog.Level

func (o levelOption) apply(conf *config) {
	conf.options.Level = slog.Level(o)
}

// AddSource returns an Option to add source information to log records.
// It causes the handler to compute the source code position
// of the log statement and add a SourceKey attribute to the output.
func AddSource() Option {
	return addSourceOption{}
}

type addSourceOption struct{}

func (o addSourceOption) apply(conf *config) {
	conf.options.AddSource = true
}

// ReplaceAttr returns an Option to replace the value of an attribute.
// ReplaceAttr is called to rewrite each non-group attribute before it is logged.
// Unlike the similar option in [slog.HandlerOptions], the function will not be
// provided the built-in attributes.
//
// The attribute's value has been resolved (see [Value.Resolve]).
// If ReplaceAttr returns an Attr with Key == "", the attribute is discarded.
//
// The first argument is a list of currently open groups that contain the
// Attr. It must not be retained or modified. ReplaceAttr is never called
// for Group attributes, only their contents. For example, the attribute
// list
//
//	Int("a", 1), Group("g", Int("b", 2)), Int("c", 3)
//
// results in consecutive calls to ReplaceAttr with the following arguments:
//
//	nil, Int("a", 1)
//	[]string{"g"}, Int("b", 2)
//	nil, Int("c", 3)
//
// ReplaceAttr can be used to change the default keys of the built-in
// attributes, convert types (for example, to replace a `time.Time` with the
// integer seconds since the Unix epoch), sanitize personal information, or
// remove attributes from the output.
func ReplaceAttr(f func([]string, slog.Attr) slog.Attr) Option {
	return replaceAttrOption(f)
}

type replaceAttrOption func([]string, slog.Attr) slog.Attr

func (o replaceAttrOption) apply(conf *config) {
	conf.options.ReplaceAttr = o
}
