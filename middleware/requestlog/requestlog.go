package requestlog

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"sync"

	"github.com/felixge/httpsnoop"
)

// NewMiddleware returns an [http.Handler] middleware that logs requests in
// GCP structured format using [slog]. With [slog.JSONHandler] used as the
// handler, logs will be rendered on the GCP console with rich information
// about the HTTP request.
func NewMiddleware(opts ...Option) func(http.Handler) http.Handler {
	var conf config
	for _, o := range opts {
		o.apply(&conf)
	}

	return func(next http.Handler) http.Handler {
		return &handler{
			next:   next,
			logger: conf.logger,
		}
	}
}

type handler struct {
	next   http.Handler
	logger *slog.Logger
}

// ServeHTTP implements http.Handler.
func (h *handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	metrics := httpsnoop.Metrics{Code: http.StatusOK}

	var extraAttrs []any
	ctx := context.WithValue(req.Context(), extraAttrsContextKey, &extraAttrs)
	req = req.WithContext(ctx)
	defer func(ctx context.Context) {
		var stack []byte
		var servePanic any
		if err := recover(); err != nil {
			servePanic = err
			defer panic(servePanic)

			pooled := stacks.Get().(*[]byte)
			defer stacks.Put(pooled)
			n := runtime.Stack(*pooled, false)
			stack = (*pooled)[:n]
		}

		reqAttrs := []any{
			slog.String("requestMethod", req.Method),
			slog.String("requestUrl", req.URL.String()),
			slog.String("protocol", req.Proto),
			slog.String("remoteIp", req.RemoteAddr),
			slog.Int64("responseSize", metrics.Written),
			slog.String("latency", fmt.Sprintf("%.9fs", metrics.Duration.Seconds())),
		}
		if ua := req.Header.Get("User-Agent"); ua != "" {
			reqAttrs = append(reqAttrs, slog.String("userAgent", ua))
		}
		if servePanic != nil {
			// It is possible for a handler to flush a different status code before
			// panicking, but almost all cases will still cause the client side to
			// treat the response as an unknown error and the request is actually an
			// error. We go ahead and always use 500 for a panic. This is suspicious
			// but seems better in practice.
			reqAttrs = append(reqAttrs, slog.Int("status", 500))
		} else {
			reqAttrs = append(reqAttrs, slog.Int("status", metrics.Code))
		}

		logArgs := append([]any{
			slog.Group("httpRequest", reqAttrs...),
		}, extraAttrs...)
		if stack != nil {
			logArgs = append(logArgs, slog.String("stack_trace", string(stack)))
		}

		l := h.logger
		if l == nil {
			l = slog.Default()
		}
		l.InfoContext(ctx, "Server Request", logArgs...)
	}(ctx)

	metrics.CaptureMetrics(w, func(ww http.ResponseWriter) {
		h.next.ServeHTTP(ww, req)
	})
}

// Cap stack trace recording to 4KB.
var stacks = sync.Pool{New: func() any {
	buf := make([]byte, 4096)
	return &buf
}}

type config struct {
	logger *slog.Logger
}

// Option is a configuration option for NewMiddleware.
type Option interface {
	apply(conf *config)
}

// Logger returns an Option to set the [slog.Logger] used by the middleware.
// If not provided, the default logger is used.
func Logger(l *slog.Logger) Option {
	return &loggerOption{logger: l}
}

type loggerOption struct {
	logger *slog.Logger
}

func (o *loggerOption) apply(conf *config) {
	conf.logger = o.logger
}

type extraAttrsContextKeyType struct{}

var extraAttrsContextKey = extraAttrsContextKeyType{}

// AddExtraAttr adds an extra [slog.Attr] record with the request log for the
// given context. If the context does not originate from the requestlog middleware,
// it is ignored.
func AddExtraAttr(ctx context.Context, attr slog.Attr) {
	if attrs, ok := ctx.Value(extraAttrsContextKey).(*[]any); ok {
		*attrs = append(*attrs, attr)
	}
}
