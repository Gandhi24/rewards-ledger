package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

type contextKey string

const requestIDKey contextKey = "request_id"

func requestIDFromCtx(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// statusCapture wraps ResponseWriter to intercept the written status code.
type statusCapture struct {
	http.ResponseWriter
	status int
}

func (sc *statusCapture) WriteHeader(status int) {
	sc.status = status
	sc.ResponseWriter.WriteHeader(status)
}

func (sc *statusCapture) Write(b []byte) (int, error) {
	if sc.status == 0 {
		sc.status = http.StatusOK
	}
	return sc.ResponseWriter.Write(b)
}

// requestID extracts X-Request-ID from the incoming request or generates one,
// injects it into the context, and echoes it back as a response header.
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// logging emits one structured log line per request.
// 5xx responses are logged at ERROR; everything else at INFO.
func logging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sc := &statusCapture{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()

		next.ServeHTTP(sc, r)

		level := slog.LevelInfo
		if sc.status >= 500 {
			level = slog.LevelError
		}

		logger.Log(r.Context(), level, "request",
			"request_id", requestIDFromCtx(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", sc.status,
			"duration_us", time.Since(start).Microseconds(),
		)
	})
}
