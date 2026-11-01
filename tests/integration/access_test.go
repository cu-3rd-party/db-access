package integration

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"git.cu3rd.ru/cu3rd/db-access/models/accesslog"
	account_model "git.cu3rd.ru/cu3rd/db-access/models/account"
	"git.cu3rd.ru/cu3rd/db-access/models/db"
	"git.cu3rd.ru/cu3rd/db-access/modules/identity"
	"git.cu3rd.ru/cu3rd/db-access/modules/setting"
	access_service "git.cu3rd.ru/cu3rd/db-access/services/access"
	"git.cu3rd.ru/cu3rd/db-access/services/provision"
	"git.cu3rd.ru/cu3rd/db-access/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// session keeps cookies between requests like a browser.
type session struct {
	t       *testing.T
	cookies map[string]*http.Cookie
}

func newSession(t *testing.T) *session { return &session{t: t, cookies: map[string]*http.Cookie{}} }

func (s *session) do(req *http.Request, status int) *httptest.ResponseRecorder {
	s.t.Helper()
	for _, c := range s.cookies {
		req.AddCookie(c)
	}
	resp := MakeRequest(s.t, req, status)
	for _, c := range resp.Result().Cookies() {
		s.cookies[c.Name] = c
	}
	return resp
}

func (s *session) login() string { return s.cookies["cu_login"].Value }

var passwordRe = regexp.MustCompile(`id="password"[^>]*>([a-zA-Z0-9]{32})<`)

func prepare(t *testing.T) *provision.Fake {
	t.Helper()
	tests.PrepareTestEnv(t)
	access_service.ResetLimits()
	fake := provision.NewFake()
	access_service.SetProvisioner(fake)
	return fake
}

func TestFirstVisitSetsCookieButCreatesNothing(t *testing.T) {
	fake := prepare(t)
	s := newSession(t)
	resp := s.do(NewRequest(t, "GET", "/"), http.StatusOK)

	assert.True(t, identity.Valid(s.login()), s.login())
	c := s.cookies["cu_login"]
	assert.True(t, c.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, c.SameSite)
	assert.Greater(t, c.MaxAge, 300*24*3600)
	assert.Equal(t, "no-store", resp.Header().Get("Cache-Control"))
	assert.Empty(t, fake.Roles, "a crawler following a link must not create a role")
	NewHTMLParser(t, resp.Body).AssertElement(t, `#panel[hx-post="/account"][hx-trigger="load"]`, true)
}

func TestPasswordIsShownOnce(t *testing.T) {
	fake := prepare(t)
	s := newSession(t)
	s.do(NewRequest(t, "GET", "/"), http.StatusOK)

	resp := s.do(AsHTMX(NewRequest(t, "POST", "/account")), http.StatusOK)
	body := resp.Body.String()
	m := passwordRe.FindStringSubmatch(body)
	require.Len(t, m, 2, body)
	assert.Contains(t, fake.Roles, s.login())
	assert.Contains(t, body, "postgresql://"+s.login()+":"+m[1]+"@db.cu3rd.ru:5432/cu-sql?sslmode=require")
	assert.Contains(t, body, "/cu-db?sslmode=require")

	again := s.do(NewRequest(t, "GET", "/"), http.StatusOK).Body.String()
	assert.NotContains(t, again, m[1])
	assert.Contains(t, again, `id="login"`)
	repeated := s.do(AsHTMX(NewRequest(t, "POST", "/account")), http.StatusOK).Body.String()
	assert.NotContains(t, repeated, m[1])
	assert.Len(t, fake.Roles, 1)
}

func TestCreateWithoutJavaScriptShowsPasswordInFullPage(t *testing.T) {
	prepare(t)
	s := newSession(t)
	s.do(NewRequest(t, "GET", "/"), http.StatusOK)
	body := s.do(NewRequest(t, "POST", "/account"), http.StatusOK).Body.String()
	assert.Regexp(t, passwordRe, body)
	assert.Contains(t, body, "<html")
}

func TestCreateRetriesWithNewLoginOnRoleCollision(t *testing.T) {
	fake := prepare(t)
	s := newSession(t)
	s.do(NewRequest(t, "GET", "/"), http.StatusOK)
	taken := s.login()
	fake.Roles[taken] = "another application"

	body := s.do(AsHTMX(NewRequest(t, "POST", "/account")), http.StatusOK).Body.String()
	assert.NotEqual(t, taken, s.login())
	assert.Regexp(t, passwordRe, body)
	assert.Contains(t, fake.Roles, s.login())
}

func TestForgedCookieIsNotAccepted(t *testing.T) {
	fake := prepare(t)
	victim := newSession(t)
	victim.do(NewRequest(t, "GET", "/"), http.StatusOK)
	victim.do(AsHTMX(NewRequest(t, "POST", "/account")), http.StatusOK)

	// any database user can read the login in pg_roles
	req := AsHTMX(NewRequest(t, "POST", "/account/rotate"))
	req.AddCookie(&http.Cookie{Name: "cu_login", Value: victim.login()})
	req.AddCookie(&http.Cookie{Name: "cu_login_sig", Value: strings.Repeat("0", 64)})
	MakeRequest(t, req, http.StatusBadRequest)
	assert.Contains(t, fake.Roles, victim.login())

	req = NewRequest(t, "GET", "/")
	req.AddCookie(&http.Cookie{Name: "cu_login", Value: victim.login()})
	resp := MakeRequest(t, req, http.StatusOK)
	var issued string
	for _, c := range resp.Result().Cookies() {
		if c.Name == "cu_login" {
			issued = c.Value
		}
	}
	assert.NotEqual(t, victim.login(), issued)
}

func TestRotate(t *testing.T) {
	fake := prepare(t)
	s := newSession(t)
	s.do(NewRequest(t, "GET", "/"), http.StatusOK)
	first := passwordRe.FindStringSubmatch(s.do(AsHTMX(NewRequest(t, "POST", "/account")), http.StatusOK).Body.String())[1]
	oldLogin := s.login()

	body := s.do(AsHTMX(NewRequest(t, "POST", "/account/rotate")), http.StatusOK).Body.String()
	second := passwordRe.FindStringSubmatch(body)[1]
	assert.NotEqual(t, first, second)
	assert.NotEqual(t, oldLogin, s.login())
	assert.True(t, identity.Valid(s.login()))
	assert.NotContains(t, fake.Roles, oldLogin)
	assert.Contains(t, fake.Roles, s.login())
	assert.Len(t, fake.Roles, 1)
}

func TestWriteCheckbox(t *testing.T) {
	fake := prepare(t)
	s := newSession(t)
	s.do(NewRequest(t, "GET", "/"), http.StatusOK)
	s.do(AsHTMX(NewRequest(t, "POST", "/account")), http.StatusOK)

	page := s.do(NewRequest(t, "GET", "/"), http.StatusOK).Body.String()
	assert.Contains(t, page, "будет забран через день")

	on := AsHTMX(NewRequestWithValues(t, "POST", "/write", map[string]string{"database": "cu-db", "enabled": "on"}))
	body := s.do(on, http.StatusOK).Body.String()
	doc := NewHTMLParser(t, strings.NewReader(body))
	doc.AssertElement(t, `#db-cu-db input[name="enabled"][checked]`, true)
	assert.Contains(t, body, "МСК")
	assert.Contains(t, body, "postgresql://"+s.login()+":ПАРОЛЬ@db.cu3rd.ru:5432/cu-db?sslmode=require")
	assert.True(t, fake.Writes[s.login()+"/cu-db"])
	assert.False(t, fake.Writes[s.login()+"/cu-sql"])

	page = s.do(NewRequest(t, "GET", "/"), http.StatusOK).Body.String()
	pageDoc := NewHTMLParser(t, strings.NewReader(page))
	pageDoc.AssertElement(t, `#db-cu-db input[name="enabled"][checked]`, true)
	pageDoc.AssertElement(t, `#db-cu-sql input[name="enabled"][checked]`, false)

	off := AsHTMX(NewRequestWithValues(t, "POST", "/write", map[string]string{"database": "cu-db"}))
	body = s.do(off, http.StatusOK).Body.String()
	assert.NotContains(t, body, "checked")
	assert.False(t, fake.Writes[s.login()+"/cu-db"])

	bad := AsHTMX(NewRequestWithValues(t, "POST", "/write", map[string]string{"database": "postgres", "enabled": "on"}))
	s.do(bad, http.StatusUnprocessableEntity)
}

func TestWriteWithoutAccountIsRejected(t *testing.T) {
	prepare(t)
	s := newSession(t)
	s.do(NewRequest(t, "GET", "/"), http.StatusOK)
	s.do(NewRequestWithValues(t, "POST", "/write", map[string]string{"database": "cu-db", "enabled": "on"}), http.StatusNotFound)
}

func TestCrossSitePostIsRejected(t *testing.T) {
	prepare(t)
	s := newSession(t)
	s.do(NewRequest(t, "GET", "/"), http.StatusOK)
	req := NewRequest(t, "POST", "/account/rotate")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	s.do(req, http.StatusForbidden)
}

func TestRateLimitReturnsMessage(t *testing.T) {
	prepare(t)
	limit := setting.CredentialsPerIPHour
	setting.CredentialsPerIPHour = 0
	t.Cleanup(func() { setting.CredentialsPerIPHour = limit })
	s := newSession(t)
	s.do(NewRequest(t, "GET", "/"), http.StatusOK)
	s.do(AsHTMX(NewRequest(t, "POST", "/account")), http.StatusTooManyRequests)
}

func TestAccessLogRecordsRequestsButNoSecrets(t *testing.T) {
	prepare(t)
	s := newSession(t)

	req := NewRequest(t, "GET", "/?utm=1")
	req.Host = "db.cu3rd.ru"
	req.Header.Set("User-Agent", "test-agent/1.0")
	req.Header.Set("X-Forwarded-For", "6.6.6.6, 203.0.113.7")
	req.Header.Set("Referer", "https://example.org/")
	req.Header.Set("Accept-Language", "ru-RU")
	s.do(req, http.StatusOK)

	var rows []accesslog.AccessLog
	require.NoError(t, db.GetEngine(t.Context()).Find(&rows))
	require.Len(t, rows, 1)
	e := rows[0]
	assert.Equal(t, "203.0.113.7", e.IP, "the value a client could forge is not trusted")
	assert.Equal(t, "6.6.6.6, 203.0.113.7", e.ForwardedFor)
	assert.Equal(t, s.login(), e.Login)
	assert.Equal(t, "GET", e.Method)
	assert.Equal(t, "db.cu3rd.ru", e.Host)
	assert.Equal(t, "/", e.Path)
	assert.Equal(t, "utm=1", e.Query)
	assert.Equal(t, http.StatusOK, e.Status)
	assert.Equal(t, "test-agent/1.0", e.UserAgent)
	assert.Equal(t, "https://example.org/", e.Referer)
	assert.Equal(t, "ru-RU", e.AcceptLanguage)
	assert.Positive(t, e.Bytes)

	body := s.do(AsHTMX(NewRequest(t, "POST", "/account")), http.StatusOK).Body.String()
	password := passwordRe.FindStringSubmatch(body)[1]
	rows = nil
	require.NoError(t, db.GetEngine(t.Context()).Find(&rows))
	require.Len(t, rows, 2)
	assert.NotContains(t, rows[1].Path+rows[1].Query+rows[1].Referer, password)

	MakeRequest(t, NewRequest(t, "GET", "/api/healthz"), http.StatusOK)
	unittestCount, err := db.GetEngine(t.Context()).Count(new(accesslog.AccessLog))
	require.NoError(t, err)
	assert.EqualValues(t, 2, unittestCount, "probes stay out of the log")
}

func TestExistingFixtureAccountKeepsGrant(t *testing.T) {
	prepare(t)
	acc, err := account_model.GetByLogin(t.Context(), "fixtureA")
	require.NoError(t, err)
	grants, err := account_model.Grants(t.Context(), acc.ID)
	require.NoError(t, err)
	assert.Contains(t, grants, "cu-sql")
}
