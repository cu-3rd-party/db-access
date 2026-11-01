// Package httpmw holds middleware shared by the routes.
package httpmw

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"git.cu3rd.ru/cu3rd/db-access/models/accesslog"

	"github.com/go-chi/chi/v5/middleware"
)

const queueSize = 4096

var queue chan *accesslog.AccessLog

// StartAccessLogWriter stores entries in the background until ctx ends, then drains
// the queue; without it entries are stored synchronously. The returned channel closes
// when everything is written.
func StartAccessLogWriter(ctx context.Context) <-chan struct{} {
	queue = make(chan *accesslog.AccessLog, queueSize)
	done := make(chan struct{})
	store := func(e *accesslog.AccessLog) {
		// the request is over and ctx may be too: the entry still has to be saved
		writeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := accesslog.Insert(writeCtx, e); err != nil {
			slog.Error("store access log", "err", err)
		}
	}
	go func() {
		defer close(done)
		for {
			select {
			case e := <-queue:
				store(e)
			case <-ctx.Done():
				for {
					select {
					case e := <-queue:
						store(e)
					default:
						return
					}
				}
			}
		}
	}()
	return done
}

type loginKey struct{}

// SetRequestLogin records whose request this is, for the access log.
func SetRequestLogin(ctx context.Context, login string) {
	if p, ok := ctx.Value(loginKey{}).(*string); ok {
		*p = login
	}
}

// ClientIP returns the address of the client. Traefik appends the address of whoever
// connected to it to X-Forwarded-For, so only the last value cannot be forged by the
// client. The service is reachable only through Traefik (NetworkPolicy).
func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[len(parts)-1])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// clean makes a client-supplied value fit a text column.
func clean(s string, limit int) string {
	s = strings.ReplaceAll(strings.ToValidUTF8(s, "?"), "\x00", "?")
	if len(s) > limit {
		s = strings.ToValidUTF8(s[:limit], "")
	}
	return s
}

// AccessLog writes every request except health probes to the database and to stdout.
// Bodies and cookies are not recorded: passwords must never reach the log.
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		login := new(string)
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		defer func() {
			entry := &accesslog.AccessLog{
				TimeUnixMilli:  start.UnixMilli(),
				IP:             clean(ClientIP(r), 64),
				RemoteAddr:     clean(r.RemoteAddr, 64),
				ForwardedFor:   clean(r.Header.Get("X-Forwarded-For"), 256),
				Login:          *login,
				Method:         clean(r.Method, 16),
				Host:           clean(r.Host, 256),
				Path:           clean(r.URL.Path, 1024),
				Query:          clean(r.URL.RawQuery, 1024),
				Proto:          clean(r.Proto, 16),
				Status:         ww.Status(),
				Bytes:          int64(ww.BytesWritten()),
				DurationMillis: time.Since(start).Milliseconds(),
				UserAgent:      clean(r.UserAgent(), 512),
				Referer:        clean(r.Referer(), 512),
				AcceptLanguage: clean(r.Header.Get("Accept-Language"), 128),
			}
			slog.Info("access", "ip", entry.IP, "login", entry.Login, "method", entry.Method, "path", entry.Path,
				"status", entry.Status, "bytes", entry.Bytes, "ms", entry.DurationMillis, "ua", entry.UserAgent)
			if queue == nil {
				if err := accesslog.Insert(context.WithoutCancel(r.Context()), entry); err != nil {
					slog.Error("store access log", "err", err)
				}
				return
			}
			select {
			case queue <- entry:
			default:
				slog.Warn("access log queue is full, the entry stays only in stdout")
			}
		}()
		next.ServeHTTP(ww, r.WithContext(context.WithValue(r.Context(), loginKey{}, login)))
	})
}

// SecureHeaders stops pages with a password from being cached, framed or leaked by Referer.
func SecureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Cache-Control", "no-store")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
