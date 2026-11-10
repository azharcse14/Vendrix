package httpx

import (
	"net/http"
	"strconv"
)

// JSONErrors: route na mille ServeMux er nijer 404/405 (text) er bodle 7.1 er JSON; 405 e Allow header
// thake. Mux er handler ke chupchap chaliye status ar header dhore, body fele dey.
func JSONErrors(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h, pattern := mux.Handler(r)
		if pattern != "" {
			mux.ServeHTTP(w, r)
			return
		}
		cw := &captureWriter{header: http.Header{}}
		h.ServeHTTP(cw, r)
		switch cw.status {
		case http.StatusMethodNotAllowed:
			w.Header().Set("Allow", cw.header.Get("Allow"))
			WriteError(w, r, cw.status, ErrorBody{Code: "method_not_allowed"})
		case http.StatusNotFound:
			WriteError(w, r, cw.status, ErrorBody{Code: "not_found"})
		default: // redirect (trailing slash ba path clean)
			for k, v := range cw.header {
				w.Header()[k] = v
			}
			w.WriteHeader(cw.status)
		}
	})
}

type captureWriter struct {
	header http.Header
	status int
}

func (c *captureWriter) Header() http.Header { return c.header }
func (c *captureWriter) WriteHeader(s int) {
	if c.status == 0 {
		c.status = s
	}
}
func (c *captureWriter) Write(b []byte) (int, error) {
	c.WriteHeader(http.StatusOK)
	return len(b), nil
}

const (
	defaultLimit = 24
	maxLimit     = 100
)

// Page (7.1): ?page=1&limit=24 (max 100). Bhul hole nijei 400 validation_failed likhe ok = false dey.
func Page(w http.ResponseWriter, r *http.Request) (page, limit int, ok bool) {
	fields := map[string]string{}
	num := func(name string, def, max int) int {
		s := r.URL.Query().Get(name)
		if s == "" {
			return def
		}
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > max {
			fields[name] = "invalid"
		}
		return n
	}
	page = num("page", 1, int(^uint(0)>>1))
	limit = num("limit", defaultLimit, maxLimit)
	if len(fields) > 0 {
		WriteError(w, r, http.StatusBadRequest, ErrorBody{Code: "validation_failed", Fields: fields})
		return 0, 0, false
	}
	return page, limit, true
}

// Paged (7.1): list er response.
type Paged[T any] struct {
	Items []T   `json:"items"`
	Page  int   `json:"page"`
	Limit int   `json:"limit"`
	Total int64 `json:"total"`
}

// NewPaged: items nil holeo JSON e [] (null na).
func NewPaged[T any](items []T, page, limit int, total int64) Paged[T] {
	if items == nil {
		items = []T{}
	}
	return Paged[T]{Items: items, Page: page, Limit: limit, Total: total}
}
