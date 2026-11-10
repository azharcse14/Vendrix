// Package httpx: HTTP er shared jinish (4.2): middleware chain, request id, client IP (3.2), request log,
// panic recover, CORS (admin), JSON ar error format (7.1). Business niyom ekhane na.
//
// Feature doc: docs/features/http-foundation.md
package httpx

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Middleware: ekta handler ke ar ekta handler e mure.
type Middleware func(http.Handler) http.Handler

// Chain: mws[0] shob cheye bairer: Chain(h, A, B) = A(B(h)).
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

type ctxKey int

const (
	requestIDKey ctxKey = iota
	clientIPKey
	shopKey
	staffKey
)

// RequestID: protiti request e notun random id, X-Request-Id response header ar log e. Client er pathano
// id mane na, jate keu log e bhul id boshate na pare.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := rand.Text()
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

// RequestIDFrom: RequestID middleware er id ("" jodi middleware na thake).
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// Recover: handler panic korle 500 `internal`; details shudhu log e, client e stack na (12.2).
// http.ErrAbortHandler abar panic hoy (net/http er niyom: connection bondho, log na).
func Recover(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				v := recover()
				if v == nil {
					return
				}
				if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(v)
				}
				log.ErrorContext(r.Context(), "panic", "request_id", RequestIDFrom(r.Context()),
					"panic", fmt.Sprint(v), "stack", string(debug.Stack()))
				WriteError(w, r, http.StatusInternalServerError, ErrorBody{Code: "internal"})
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// ErrorBody: 7.1 er error. Client `code` diye logic kore, `message` shudhu dekhay.
type ErrorBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
	Details map[string]any    `json:"details,omitempty"`
}

// WriteJSON: JSON response (UTF-8).
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError: {"error": {...}} (7.1). Message khali hole code ar Accept-Language theke (v1.10).
func WriteError(w http.ResponseWriter, r *http.Request, status int, e ErrorBody) {
	if e.Message == "" {
		e.Message = message(r, e.Code)
	}
	WriteJSON(w, status, map[string]ErrorBody{"error": e})
}

// MaxBody: default request body er shima (7.1). Chobi upload er route nijer shima dey (10 MB, 2.2).
const MaxBody = 1 << 20

// DecodeJSON: nijer client er request body (7.1): MaxBody er beshi na, ochena field e error
// (DisallowUnknownFields), ekta-i JSON value. Third-party webhook e use korbe na (7.1). Caller error ke
// 400 validation_failed banay (http.MaxBytesError hole 413).
func DecodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("body te ekta-i JSON value thakbe")
	}
	return nil
}
