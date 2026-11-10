package httpx

import (
	"log/slog"
	"net/http"
	"time"
)

// Log: protiti request er ek line JSON log (13.8): request_id, method, route, status, bytes, duration.
// Route holo ServeMux er pattern ("GET /api/orders/{token}"), path na: path e public_token ba
// confirm_token thake. Query, IP ar body o log e na (12.2: personal data na).
func Log(log *slog.Logger, mux *http.ServeMux) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w}
			_, route := mux.Handler(r)
			if route == "" {
				route = "-"
			}
			next.ServeHTTP(sw, r)
			if sw.status == 0 {
				sw.status = http.StatusOK
			}
			log.LogAttrs(r.Context(), slog.LevelInfo, "request",
				slog.String("request_id", RequestIDFrom(r.Context())),
				slog.String("method", r.Method),
				slog.String("route", route),
				slog.Int("status", sw.status),
				slog.Int64("bytes", sw.bytes),
				slog.Duration("duration", time.Since(start)))
		})
	}
}

// statusWriter: status ar byte gone. Unwrap thakay http.ResponseController (upload er deadline, 2.2) kaj kore.
type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += int64(n)
	return n, err
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
