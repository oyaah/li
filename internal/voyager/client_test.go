package voyager

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oyaah/li/internal/output"
)

func testClient(base string) *Client {
	c := New(Creds{LiAt: "LIAT_VAL", JSESSIONID: `"ajax:9876"`, UserAgent: "UA/1.0"})
	c.Base = base
	return c
}

func TestCsrfStripsQuotes(t *testing.T) {
	if got := Csrf(`"ajax:9876"`); got != "ajax:9876" {
		t.Fatalf("Csrf = %q", got)
	}
	if got := Csrf("ajax:nochar"); got != "ajax:nochar" {
		t.Fatalf("Csrf = %q", got)
	}
}

func TestBuildRequestHeaders(t *testing.T) {
	c := testClient(BaseURL)
	req, err := c.buildRequest(http.MethodGet, "/x", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]string{
		"x-restli-protocol-version": "2.0.0",
		"x-li-lang":                 "en_US",
		"csrf-token":                "ajax:9876",
		"user-agent":                "UA/1.0",
	}
	for k, want := range checks {
		if got := req.Header.Get(k); got != want {
			t.Errorf("header %s = %q want %q", k, got, want)
		}
	}
}

func TestParseCookies(t *testing.T) {
	cs := parseCookies(`li_at=A; lidc=B; JSESSIONID="stale"; g_state={"i":0}`, "", `"ajax:1"`)
	got := map[string]string{}
	quoted := map[string]bool{}
	for _, c := range cs {
		got[c.Name] = c.Value
		quoted[c.Name] = c.Quoted
	}
	if got["li_at"] != "A" || got["lidc"] != "B" || got["JSESSIONID"] != "ajax:1" || !quoted["JSESSIONID"] {
		t.Fatalf("parsed values=%v quoted=%v", got, quoted)
	}
	if got["g_state"] != "{i:0}" {
		t.Fatalf("g_state should be cookie-safe, got %q", got["g_state"])
	}
}

func TestParseCookiesSkipsHeaderJSESSIONIDWhenBootstrapping(t *testing.T) {
	cs := parseCookies(`li_at=A; JSESSIONID="stale"`, "", "")
	for _, c := range cs {
		if c.Name == "JSESSIONID" {
			t.Fatalf("JSESSIONID should be skipped when client will bootstrap, got %+v", c)
		}
	}
}

func TestDoMapsAuthError(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
		}))
		c := testClient(srv.URL)
		_, err := c.GetRaw("/x", nil)
		if !errors.Is(err, output.ErrAuth) {
			t.Errorf("status %d: got %v, want ErrAuth", code, err)
		}
		srv.Close()
	}
}

func TestGetRawSendsCookiesToServer(t *testing.T) {
	var gotCookie, gotCsrf string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("cookie")
		gotCsrf = r.Header.Get("csrf-token")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c := testClient(srv.URL)
	if _, err := c.GetRaw("/identity", nil); err != nil {
		t.Fatal(err)
	}
	if !contains(gotCookie, "li_at=LIAT_VAL") ||
		!contains(gotCookie, `JSESSIONID="ajax:9876"`) ||
		gotCsrf != "ajax:9876" {
		t.Fatalf("server saw cookie=%q csrf=%q", gotCookie, gotCsrf)
	}
}

func TestGetRawBootstrapsJSESSIONIDFromRoot(t *testing.T) {
	var gotCookie, gotCsrf string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			http.SetCookie(w, &http.Cookie{Name: "JSESSIONID", Value: "ajax:minted", Quoted: true, Path: "/"})
			w.Write([]byte(`<html></html>`))
		case "/x":
			gotCookie = r.Header.Get("cookie")
			gotCsrf = r.Header.Get("csrf-token")
			w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c := New(Creds{LiAt: "LIAT_VAL", UserAgent: "UA/1.0"})
	c.Base = srv.URL
	if _, err := c.GetRaw("/x", nil); err != nil {
		t.Fatal(err)
	}
	if !contains(gotCookie, "li_at=LIAT_VAL") ||
		!contains(gotCookie, `JSESSIONID="ajax:minted"`) ||
		gotCsrf != "ajax:minted" {
		t.Fatalf("server saw cookie=%q csrf=%q", gotCookie, gotCsrf)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
