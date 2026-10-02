package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// LoggingMiddleware объединяет служебные middleware (логи, CORS, recovery и др.), которым нужен общий logger
type LoggingMiddleware struct {
	logger *log.Logger
}

func NewLoggingMiddleware(logger *log.Logger) *LoggingMiddleware {
	return &LoggingMiddleware{
		logger: logger,
	}
}

func (m *LoggingMiddleware) Logger(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		wrapped := newResponseWriter(w)
		next(wrapped, r)

		m.logger.Printf(
			"%s %s %s %d %s",
			r.Method, r.URL.Path, getClientIP(r), wrapped.statusCode, time.Since(start),
		)
	}
}

// Recovery превращает панику в 500, чтобы один запрос не ронял весь сервер
func (m *LoggingMiddleware) Recovery(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				m.logger.Printf("PANIC recovered: %v\nPath: %s %s", rec, r.Method, r.URL.Path)
				writeJSONError(w, "Internal server error", http.StatusInternalServerError)
			}
		}()

		next(w, r)
	}
}

// CORS сам отвечает 204 на preflight OPTIONS и не пропускает его дальше
// Работает только если стоит снаружи роутера, иначе chi успеет вернуть 405
func (m *LoggingMiddleware) CORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "3600")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next(w, r)
	}
}

// RequestID кладет ID запроса в контекст и в заголовок X-Request-ID
func (m *LoggingMiddleware) RequestID(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestID := generateRequestID()

		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		w.Header().Set("X-Request-ID", requestID)

		m.logger.Printf("[%s] %s %s", requestID, r.Method, r.URL.Path)

		next(w, r.WithContext(ctx))
	}
}

type rateLimiterEntry struct {
	count       int
	windowStart time.Time
}

// RateLimiter ограничивает число запросов с одного IP за окно
// Счетчики лежат в памяти процесса: для одного инстанса хватает,
// для нескольких понадобилось бы общее хранилище вроде Redis
func (m *LoggingMiddleware) RateLimiter(maxRequests int, window time.Duration) func(http.HandlerFunc) http.HandlerFunc {
	var mu sync.Mutex
	clients := make(map[string]*rateLimiterEntry)

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)

			mu.Lock()
			entry, exists := clients[ip]
			now := time.Now()

			if !exists || now.Sub(entry.windowStart) > window {
				clients[ip] = &rateLimiterEntry{count: 1, windowStart: now}
				mu.Unlock()
				next(w, r)
				return
			}

			if entry.count >= maxRequests {
				mu.Unlock()
				writeJSONError(w, "Too many requests", http.StatusTooManyRequests)
				return
			}

			entry.count++
			mu.Unlock()

			next(w, r)
		}
	}
}

func (m *LoggingMiddleware) ContentTypeJSON(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next(w, r)
	}
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// В X-Forwarded-For может быть цепочка client, proxy1, proxy2 — берем первый адрес
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}

	return r.RemoteAddr
}

func generateRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// responseWriter запоминает статус ответа, чтобы Logger мог его залогировать
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// WriteHeader фиксирует только первый статус: повторные вызовы net/http все равно игнорирует
func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.ResponseWriter.WriteHeader(code)
		rw.written = true
	}
}

// Write без явного WriteHeader отдает 200, как и net/http
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		written:        false,
	}
}
