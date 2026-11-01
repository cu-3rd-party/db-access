// Integration tests drive the full router in-process (no socket):
// NewRequest + MakeRequest + HTML assertions.
package integration

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"git.cu3rd.ru/cu3rd/db-access/models/unittest"
	"git.cu3rd.ru/cu3rd/db-access/routers"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testWebRoutes http.Handler

func TestMain(m *testing.M) {
	unittest.MainTest(m, func() {
		testWebRoutes = routers.NormalRoutes()
	})
}

// NoExpectedStatus skips the status assertion in MakeRequest.
const NoExpectedStatus = -1

// NewRequest builds a GET-style request without a body.
func NewRequest(t testing.TB, method, urlStr string) *http.Request {
	t.Helper()
	return NewRequestWithBody(t, method, urlStr, nil)
}

// NewRequestWithValues builds a form POST.
func NewRequestWithValues(t testing.TB, method, urlStr string, values map[string]string) *http.Request {
	t.Helper()
	form := url.Values{}
	for k, v := range values {
		form.Set(k, v)
	}
	req := NewRequestWithBody(t, method, urlStr, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

// NewRequestWithBody builds a same-origin request.
func NewRequestWithBody(t testing.TB, method, urlStr string, body io.Reader) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, urlStr, body)
	require.NoError(t, err)
	req.RequestURI = urlStr
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	return req
}

// AsHTMX marks a request as sent by htmx.
func AsHTMX(req *http.Request) *http.Request {
	req.Header.Set("HX-Request", "true")
	return req
}

// MakeRequest serves req and asserts the status code.
func MakeRequest(t testing.TB, req *http.Request, expectedStatus int) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	testWebRoutes.ServeHTTP(recorder, req)
	if expectedStatus != NoExpectedStatus {
		require.Equal(t, expectedStatus, recorder.Code, "unexpected status, body: %s", recorder.Body.String())
	}
	return recorder
}

// HTMLDoc wraps goquery for assertions.
type HTMLDoc struct {
	*goquery.Document
}

// NewHTMLParser parses a response body.
func NewHTMLParser(t testing.TB, body io.Reader) *HTMLDoc {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(body)
	require.NoError(t, err)
	return &HTMLDoc{doc}
}

// AssertElement asserts whether selector matches.
func (doc *HTMLDoc) AssertElement(t testing.TB, selector string, exists bool) {
	t.Helper()
	assert.Equal(t, exists, doc.Find(selector).Length() > 0, "selector %q", selector)
}
