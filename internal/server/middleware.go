package server

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/ruskiiamov/school/internal/logger"
	"github.com/ruskiiamov/school/internal/server/web"
)

type middleware func(http.Handler) http.Handler

func chain(handler http.Handler, middlewares ...middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}

	return handler
}

type recordingWriter struct {
	http.ResponseWriter
	status int
}

func (w *recordingWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
		w.ResponseWriter.WriteHeader(status)
	}
}

func (w *recordingWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}

	return w.ResponseWriter.Write(b)
}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 8)
		if _, err := rand.Read(buf); err != nil {
			next.ServeHTTP(w, r)
			return
		}

		id := hex.EncodeToString(buf)
		w.Header().Set("X-Request-Id", id)

		next.ServeHTTP(w, r.WithContext(logger.WithRequestID(r.Context(), id)))
	})
}

func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "same-origin")
		header.Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")

		next.ServeHTTP(w, r)
	})
}

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &recordingWriter{ResponseWriter: w}
		info := &web.RequestInfo{}

		next.ServeHTTP(recorder, r.WithContext(web.WithRequestInfo(r.Context(), info)))

		attrs := []any{
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", recorder.status),
			slog.String("duration", time.Since(started).Round(time.Microsecond).String()),
		}

		if info.UserID != 0 {
			attrs = append(attrs, slog.Int64("user_id", info.UserID))
		}

		s.log.InfoContext(r.Context(), "request", attrs...)
	})
}

func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			s.log.ErrorContext(r.Context(), "panic in handler",
				slog.Any("error", recovered),
				slog.String("path", r.URL.Path),
				slog.String("stack", string(debug.Stack())))

			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		}()

		next.ServeHTTP(w, r)
	})
}

func (s *Server) crossOriginProtection(next http.Handler) http.Handler {
	protection := http.NewCrossOriginProtection()
	protection.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.log.WarnContext(r.Context(), "cross-origin request rejected",
			slog.String("origin", r.Header.Get("Origin")),
			slog.String("path", r.URL.Path))

		http.Error(w, "cross-origin request rejected", http.StatusForbidden)
	}))

	return protection.Handler(next)
}

func (s *Server) warnInsecureCookie(next http.Handler) http.Handler {
	if s.secureCookie {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
			s.insecureCookieWarned.Do(func() {
				s.log.WarnContext(r.Context(),
					"requests arrive over https but session.secure is false; set session.secure: true in the config")
			})
		}

		next.ServeHTTP(w, r)
	})
}
