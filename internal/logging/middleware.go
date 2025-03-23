package logging

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"
)

type responseWriteInterceptor struct {
	http.ResponseWriter
	statusCode int
}

func (r *responseWriteInterceptor) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

type AccessLogMiddleware struct {
	logger             *slog.Logger
	ignorePathPrefixes []string
}

func NewAccessLogMiddleware(logger *slog.Logger) *AccessLogMiddleware {
	mw := &AccessLogMiddleware{
		logger: logger,
	}
	return mw
}

func (mw *AccessLogMiddleware) Use(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if mw.isIgnorePath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		writer := &responseWriteInterceptor{ResponseWriter: w}
		next.ServeHTTP(writer, r)
		mw.logger.InfoContext(
			r.Context(),
			"access log",
			"topic", "accesslog",
			"method", r.Method,
			"url", r.URL.String(),
			"status", writer.statusCode,
			"remote_addr", r.RemoteAddr,
			"user_agent", r.UserAgent(),
		)
	}
}

func (mw *AccessLogMiddleware) isIgnorePath(path string) bool {
	return slices.ContainsFunc(mw.ignorePathPrefixes, func(prefix string) bool {
		return strings.HasPrefix(path, prefix)
	})
}
