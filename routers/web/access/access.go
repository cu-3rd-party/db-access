// Package access holds the page where a visitor gets database credentials.
//
// Every mutation works as a plain form post and as an htmx request. Without htmx the
// response is the full page instead of a redirect where a password has to be shown:
// it exists only in this one response.
package access

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	account_model "git.cu3rd.ru/cu3rd/db-access/models/account"
	"git.cu3rd.ru/cu3rd/db-access/models/db"
	"git.cu3rd.ru/cu3rd/db-access/modules/identity"
	"git.cu3rd.ru/cu3rd/db-access/modules/setting"
	"git.cu3rd.ru/cu3rd/db-access/routers/httpmw"
	access_service "git.cu3rd.ru/cu3rd/db-access/services/access"
	"git.cu3rd.ru/cu3rd/db-access/services/context"
	"git.cu3rd.ru/cu3rd/db-access/services/provision"
)

const (
	tplPage        = "access/page"
	tplPanel       = "access/panel"
	tplWriteStatus = "access/write-status"

	cookieLogin = "cu_login"
	cookieSig   = "cu_login_sig"
	// browsers cap cookie lifetime at 400 days; every visit renews it
	cookieMaxAge = 400 * 24 * time.Hour

	maxLoginAttempts = 3
)

// DatabaseView is one database row on the page.
type DatabaseView struct {
	Name        string
	Write       bool
	ExpiresUnix int64
}

func setIdentity(ctx *context.Context, login string) {
	cookie := func(name, value string) {
		http.SetCookie(ctx.Resp, &http.Cookie{
			Name:     name,
			Value:    value,
			Path:     "/",
			MaxAge:   int(cookieMaxAge.Seconds()),
			HttpOnly: true,
			Secure:   setting.CookieSecure,
			SameSite: http.SameSiteLaxMode,
		})
	}
	cookie(cookieLogin, login)
	cookie(cookieSig, identity.Sign(setting.CookieSecret, login))
	httpmw.SetRequestLogin(ctx.Req.Context(), login)
}

// currentLogin returns the login from the cookies, or "" when they are absent or forged.
func currentLogin(ctx *context.Context) string {
	login, err := ctx.Req.Cookie(cookieLogin)
	if err != nil {
		return ""
	}
	sig, err := ctx.Req.Cookie(cookieSig)
	if err != nil || !identity.Verify(setting.CookieSecret, login.Value, sig.Value) {
		return ""
	}
	httpmw.SetRequestLogin(ctx.Req.Context(), login.Value)
	return login.Value
}

func fillPanel(ctx *context.Context, c *access_service.Credentials) bool {
	grants, err := account_model.Grants(ctx.Req.Context(), c.Account.ID)
	if err != nil {
		ctx.ServerError("Grants", err)
		return false
	}
	views := make([]DatabaseView, 0, len(setting.Databases))
	for _, name := range setting.Databases {
		view := DatabaseView{Name: name}
		if g, ok := grants[name]; ok {
			view.Write, view.ExpiresUnix = true, g.ExpiresUnix
		}
		views = append(views, view)
	}
	ctx.Data["Account"] = c.Account
	ctx.Data["Password"] = c.Password
	ctx.Data["Databases"] = views
	ctx.Data["DBHost"] = setting.PublicDBHost
	ctx.Data["DBPort"] = setting.PublicDBPort
	ctx.Data["WriteHours"] = int(setting.WriteTTL.Hours())
	return true
}

// respond renders the panel for htmx and the whole page otherwise.
func respond(ctx *context.Context, c *access_service.Credentials) {
	if !fillPanel(ctx, c) {
		return
	}
	if ctx.IsHTMX() {
		ctx.HTML(http.StatusOK, tplPanel)
		return
	}
	ctx.Data["Title"] = "Доступ к базам данных"
	ctx.HTML(http.StatusOK, tplPage)
}

func serviceError(ctx *context.Context, logMsg string, err error) {
	switch {
	case errors.Is(err, access_service.ErrRateLimited):
		ctx.PlainText(http.StatusTooManyRequests, err.Error())
	case errors.Is(err, access_service.ErrCapacity):
		ctx.PlainText(http.StatusServiceUnavailable, err.Error())
	default:
		ctx.NotFoundOrServerError(logMsg, err)
	}
}

// Page marks the visitor with the cookie and shows their access: GET /
// The role itself is created by the POST the page sends on load, so that crawlers,
// scanners and link previews following a link do not create database roles.
func Page(ctx *context.Context) {
	reqCtx := ctx.Req.Context()
	login := currentLogin(ctx)
	if login == "" {
		var err error
		if login, err = access_service.NewLogin(reqCtx); err != nil {
			ctx.ServerError("NewLogin", err)
			return
		}
	}
	setIdentity(ctx, login)

	acc, err := account_model.GetByLogin(reqCtx, login)
	switch {
	case err == nil:
		respond(ctx, &access_service.Credentials{Account: acc})
	case errors.Is(err, db.ErrNotExist):
		ctx.Data["Title"] = "Доступ к базам данных"
		// nothing personal in it: the login is in the cookie and the panel comes with the POST
		ctx.HTMLStatic(http.StatusOK, tplPage)
	default:
		ctx.ServerError("GetByLogin", err)
	}
}

// Create creates the visitor's role on first use: POST /account
func Create(ctx *context.Context) {
	login := currentLogin(ctx)
	if login == "" {
		ctx.PlainText(http.StatusBadRequest, "Не удалось прочитать cookie: разрешите cookie для этого сайта и обновите страницу.")
		return
	}
	reqCtx := ctx.Req.Context()
	for attempt := range maxLoginAttempts {
		c, err := access_service.Ensure(reqCtx, login, httpmw.ClientIP(ctx.Req))
		if err == nil {
			respond(ctx, c)
			return
		}
		if !errors.Is(err, provision.ErrRoleExists) || attempt == maxLoginAttempts-1 {
			serviceError(ctx, "Ensure", err)
			return
		}
		// the identifier is a name another role of the cluster already has
		if login, err = access_service.NewLogin(reqCtx); err != nil {
			ctx.ServerError("NewLogin", err)
			return
		}
		setIdentity(ctx, login)
	}
}

func currentAccount(ctx *context.Context) *account_model.Account {
	login := currentLogin(ctx)
	if login == "" {
		ctx.PlainText(http.StatusBadRequest, "Не удалось прочитать cookie: обновите страницу.")
		return nil
	}
	acc, err := account_model.GetByLogin(ctx.Req.Context(), login)
	if err != nil {
		ctx.NotFoundOrServerError("GetByLogin", err)
		return nil
	}
	return acc
}

// Rotate replaces the login and password: POST /account/rotate
func Rotate(ctx *context.Context) {
	acc := currentAccount(ctx)
	if acc == nil {
		return
	}
	reqCtx := ctx.Req.Context()
	for attempt := range maxLoginAttempts {
		login, err := access_service.NewLogin(reqCtx)
		if err != nil {
			ctx.ServerError("NewLogin", err)
			return
		}
		c, err := access_service.Rotate(reqCtx, acc, login, httpmw.ClientIP(ctx.Req))
		if err == nil {
			setIdentity(ctx, login)
			respond(ctx, c)
			return
		}
		if !errors.Is(err, provision.ErrRoleExists) || attempt == maxLoginAttempts-1 {
			serviceError(ctx, "Rotate", err)
			return
		}
	}
}

// Write turns write access to one database on or off: POST /write
func Write(ctx *context.Context) {
	acc := currentAccount(ctx)
	if acc == nil {
		return
	}
	name, enabled := ctx.FormString("database"), ctx.FormString("enabled") == "on"
	grant, err := access_service.SetWrite(ctx.Req.Context(), acc, name, enabled)
	if err != nil {
		serviceError(ctx, fmt.Sprintf("SetWrite %s", name), err)
		return
	}
	if !ctx.IsHTMX() {
		ctx.Redirect("/")
		return
	}
	// only the status: a password on the screen has to stay in the connection string
	ctx.Data["Name"], ctx.Data["Write"] = name, grant != nil
	if grant != nil {
		ctx.Data["ExpiresUnix"] = grant.ExpiresUnix
	}
	ctx.HTML(http.StatusOK, tplWriteStatus)
}
