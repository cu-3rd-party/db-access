// Package web registers the HTML (htmx) routes.
package web

import (
	"net/http"

	"git.cu3rd.ru/cu3rd/db-access/models/db"
	"git.cu3rd.ru/cu3rd/db-access/modules/assets"
	"git.cu3rd.ru/cu3rd/db-access/routers/httpmw"
	"git.cu3rd.ru/cu3rd/db-access/routers/web/access"
	"git.cu3rd.ru/cu3rd/db-access/services/context"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const maxFormBytes = 16 << 10

func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
		next.ServeHTTP(w, r)
	})
}

// Routes returns the router for the web UI.
func Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(httpmw.AccessLog, middleware.Recoverer)

	// public/ contains assets/ (webpack output) and img/, so the URL path maps 1:1
	r.Handle("/assets/*", http.FileServerFS(assets.Public()))
	r.Handle("/img/*", http.FileServerFS(assets.Public()))
	r.Get("/api/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := db.GetEngine(r.Context()).Ping(); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})

	r.Group(func(r chi.Router) {
		// rejects cross-site POST/PATCH/DELETE using Sec-Fetch-Site / Origin
		r.Use(http.NewCrossOriginProtection().Handler, httpmw.SecureHeaders, limitBody)

		r.Method(http.MethodGet, "/", context.Handler(access.Page))
		r.Method(http.MethodPost, "/account", context.Handler(access.Create))
		r.Method(http.MethodPost, "/account/rotate", context.Handler(access.Rotate))
		r.Method(http.MethodPost, "/write", context.Handler(access.Write))
	})
	return r
}
