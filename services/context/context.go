// Package context is the per-request web context: handlers take *Context and fill Data for templates.
package context

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"git.cu3rd.ru/cu3rd/db-access/models/db"
	"git.cu3rd.ru/cu3rd/db-access/modules/templates"

	"github.com/go-chi/chi/v5"
)

// Context wraps a request and the template data.
type Context struct {
	Resp http.ResponseWriter
	Req  *http.Request
	Data map[string]any
}

// Handler is a route handler taking the web context.
type Handler func(*Context)

// ServeHTTP lets a Handler be mounted on chi directly.
func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h(&Context{Resp: w, Req: r, Data: map[string]any{}})
}

// IsHTMX reports whether the request was issued by htmx (HX-Request header).
// Boosted navigations want the full page, so they are not counted.
func (ctx *Context) IsHTMX() bool {
	return ctx.Req.Header.Get("HX-Request") == "true" && ctx.Req.Header.Get("HX-Boosted") != "true"
}

// HTML renders a template with ctx.Data.
func (ctx *Context) HTML(status int, name string) {
	ctx.Resp.Header().Set("Content-Type", "text/html; charset=utf-8")
	// render to a buffer first so a template error never sends a half-written page
	var buf bytes.Buffer
	if err := templates.Renderer().HTML(&buf, name, ctx.Data); err != nil {
		ctx.ServerError("render", err)
		return
	}
	ctx.Resp.WriteHeader(status)
	_, _ = buf.WriteTo(ctx.Resp)
}

// Redirect sends the client to url: for htmx requests via HX-Redirect,
// otherwise with a 303 so that a POST becomes a GET.
func (ctx *Context) Redirect(url string) {
	if ctx.IsHTMX() {
		ctx.Resp.Header().Set("HX-Redirect", url)
		ctx.Resp.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(ctx.Resp, ctx.Req, url, http.StatusSeeOther)
}

// PlainText writes a text response; htmx shows non-2xx bodies as error toasts.
func (ctx *Context) PlainText(status int, text string) {
	ctx.Resp.Header().Set("Content-Type", "text/plain; charset=utf-8")
	ctx.Resp.WriteHeader(status)
	_, _ = ctx.Resp.Write([]byte(text))
}

// NotFoundOrServerError maps model errors to 404 / 422 / 500.
func (ctx *Context) NotFoundOrServerError(logMsg string, err error) {
	switch {
	case errors.Is(err, db.ErrNotExist):
		ctx.PlainText(http.StatusNotFound, err.Error())
	case errors.Is(err, db.ErrInvalidArgument):
		ctx.PlainText(http.StatusUnprocessableEntity, err.Error())
	default:
		ctx.ServerError(logMsg, err)
	}
}

// ServerError logs err and responds with 500 without leaking details.
func (ctx *Context) ServerError(logMsg string, err error) {
	slog.Error(logMsg, "err", err, "method", ctx.Req.Method, "path", ctx.Req.URL.Path)
	ctx.PlainText(http.StatusInternalServerError, "Internal Server Error")
}

// PathParamInt64 returns a chi URL parameter as int64 (0 if invalid).
func (ctx *Context) PathParamInt64(name string) int64 {
	v, _ := strconv.ParseInt(chi.URLParam(ctx.Req, name), 10, 64)
	return v
}

// FormString returns a form value.
func (ctx *Context) FormString(name string) string { return ctx.Req.FormValue(name) }
