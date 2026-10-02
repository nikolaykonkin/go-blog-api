package middleware

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestLoggingMiddleware() (*LoggingMiddleware, *bytes.Buffer) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	return NewLoggingMiddleware(logger), &buf
}

func TestLoggingMiddleware_Logger(t *testing.T) {
	mw, buf := newTestLoggingMiddleware()

	handler := mw.Logger(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/posts", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rec.Code)
	}

	logged := buf.String()
	if !strings.Contains(logged, "POST") || !strings.Contains(logged, "/api/posts") || !strings.Contains(logged, "201") {
		t.Fatalf("expected log line to contain method, path and status code, got: %s", logged)
	}
}

func TestLoggingMiddleware_Logger_DefaultsToStatus200WhenHandlerNeverWritesHeader(t *testing.T) {
	mw, buf := newTestLoggingMiddleware()

	handler := mw.Logger(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok")) // неявно выставляет статус 200
	})

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if !strings.Contains(buf.String(), "200") {
		t.Fatalf("expected default status 200 in log, got: %s", buf.String())
	}
}

func TestLoggingMiddleware_Recovery(t *testing.T) {
	mw, buf := newTestLoggingMiddleware()

	handler := mw.Recovery(func(w http.ResponseWriter, r *http.Request) {
		panic("something went terribly wrong")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	rec := httptest.NewRecorder()

	// паника не должна вылететь из самого теста
	handler(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 after recovering from panic, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Internal server error") {
		t.Fatalf("expected a generic error message in the body, got: %s", rec.Body.String())
	}
	if !strings.Contains(buf.String(), "PANIC recovered") {
		t.Fatalf("expected the panic to be logged, got: %s", buf.String())
	}
}

func TestLoggingMiddleware_Recovery_PassesThroughNormalRequests(t *testing.T) {
	mw, _ := newTestLoggingMiddleware()

	handler := mw.Recovery(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 for a request with no panic, got %d", rec.Code)
	}
}

func TestLoggingMiddleware_CORS(t *testing.T) {
	mw, _ := newTestLoggingMiddleware()

	t.Run("sets CORS headers and calls through for normal requests", func(t *testing.T) {
		called := false
		handler := mw.CORS(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
		rec := httptest.NewRecorder()
		handler(rec, req)

		if !called {
			t.Fatal("expected the wrapped handler to be called")
		}
		if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Fatalf("expected Access-Control-Allow-Origin header, got %q", rec.Header().Get("Access-Control-Allow-Origin"))
		}
		if rec.Header().Get("Access-Control-Allow-Methods") == "" {
			t.Fatal("expected Access-Control-Allow-Methods header to be set")
		}
	})

	t.Run("short-circuits OPTIONS preflight requests with 204 and does not call through", func(t *testing.T) {
		called := false
		handler := mw.CORS(func(w http.ResponseWriter, r *http.Request) {
			called = true
		})

		req := httptest.NewRequest(http.MethodOptions, "/api/posts", nil)
		rec := httptest.NewRecorder()
		handler(rec, req)

		if called {
			t.Fatal("expected the wrapped handler NOT to be called for an OPTIONS request")
		}
		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected status 204 for OPTIONS, got %d", rec.Code)
		}
	})
}

func TestLoggingMiddleware_RequestID(t *testing.T) {
	mw, buf := newTestLoggingMiddleware()

	var idFromContext string
	handler := mw.RequestID(func(w http.ResponseWriter, r *http.Request) {
		idFromContext, _ = r.Context().Value(requestIDKey).(string)
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	headerID := rec.Header().Get("X-Request-ID")
	if headerID == "" {
		t.Fatal("expected X-Request-ID response header to be set")
	}
	if idFromContext != headerID {
		t.Fatalf("expected context request ID %q to match header %q", idFromContext, headerID)
	}
	if !strings.Contains(buf.String(), headerID) {
		t.Fatalf("expected request ID to be logged, got: %s", buf.String())
	}
}

func TestLoggingMiddleware_RateLimiter(t *testing.T) {
	mw, _ := newTestLoggingMiddleware()

	limited := mw.RateLimiter(2, time.Minute)
	calls := 0
	handler := limited(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
	})

	makeRequest := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
		req.RemoteAddr = "203.0.113.10:12345"
		rec := httptest.NewRecorder()
		handler(rec, req)
		return rec
	}

	if rec := makeRequest(); rec.Code != http.StatusOK {
		t.Fatalf("expected 1st request to succeed, got %d", rec.Code)
	}
	if rec := makeRequest(); rec.Code != http.StatusOK {
		t.Fatalf("expected 2nd request to succeed, got %d", rec.Code)
	}
	rec := makeRequest()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 3rd request within the window to be rate limited (429), got %d", rec.Code)
	}
	if calls != 2 {
		t.Fatalf("expected the handler to have been called exactly twice, got %d", calls)
	}
}

func TestLoggingMiddleware_RateLimiter_DifferentClientsHaveSeparateBudgets(t *testing.T) {
	mw, _ := newTestLoggingMiddleware()
	limited := mw.RateLimiter(1, time.Minute)
	handler := limited(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req1 := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	req1.RemoteAddr = "203.0.113.1:1111"
	rec1 := httptest.NewRecorder()
	handler(rec1, req1)

	req2 := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	req2.RemoteAddr = "203.0.113.2:2222"
	rec2 := httptest.NewRecorder()
	handler(rec2, req2)

	if rec1.Code != http.StatusOK || rec2.Code != http.StatusOK {
		t.Fatalf("expected both distinct clients' first request to succeed, got %d and %d", rec1.Code, rec2.Code)
	}
}

func TestLoggingMiddleware_ContentTypeJSON(t *testing.T) {
	mw, _ := newTestLoggingMiddleware()

	handler := mw.ContentTypeJSON(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", got)
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(r *http.Request)
		remoteAddr string
		expected   string
	}{
		{
			name: "uses first entry of X-Forwarded-For when present",
			setup: func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", "198.51.100.1, 10.0.0.1, 10.0.0.2")
			},
			remoteAddr: "10.0.0.99:5555",
			expected:   "198.51.100.1",
		},
		{
			name: "falls back to X-Real-IP when no X-Forwarded-For",
			setup: func(r *http.Request) {
				r.Header.Set("X-Real-IP", "198.51.100.2")
			},
			remoteAddr: "10.0.0.99:5555",
			expected:   "198.51.100.2",
		},
		{
			name:       "falls back to RemoteAddr when no proxy headers are present",
			setup:      func(r *http.Request) {},
			remoteAddr: "203.0.113.55:4444",
			expected:   "203.0.113.55:4444",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			tt.setup(req)

			if got := getClientIP(req); got != tt.expected {
				t.Fatalf("expected client IP %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestGenerateRequestID(t *testing.T) {
	id1 := generateRequestID()
	id2 := generateRequestID()

	if id1 == "" || id2 == "" {
		t.Fatal("expected non-empty request IDs")
	}
	if id1 == id2 {
		t.Fatal("expected distinct request IDs on consecutive calls")
	}
}

func TestResponseWriter_WriteHeaderCalledOnce(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := newResponseWriter(rec)

	rw.WriteHeader(http.StatusCreated)
	rw.WriteHeader(http.StatusInternalServerError) // игнорируется, как и в net/http

	if rw.statusCode != http.StatusCreated {
		t.Fatalf("expected captured status to remain 201, got %d", rw.statusCode)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected underlying recorder status to remain 201, got %d", rec.Code)
	}
}

func TestResponseWriter_WriteWithoutExplicitHeaderDefaultsTo200(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := newResponseWriter(rec)

	if _, err := rw.Write([]byte("hello")); err != nil {
		t.Fatalf("unexpected error writing: %v", err)
	}

	if rw.statusCode != http.StatusOK {
		t.Fatalf("expected default status 200, got %d", rw.statusCode)
	}
}
