package voyager

import (
	"bytes"
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"
)

// BaseURL is the Voyager API root. Overridable via Client.Base for tests.
const BaseURL = "https://www.linkedin.com/voyager/api"

// DefaultUserAgent is a recent desktop Chrome UA. login captures the real
// browser UA when available; this is the fallback.
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// Creds are the session cookies (and captured UA) needed to sign requests.
type Creds struct {
	LiAt       string `json:"li_at"`
	JSESSIONID string `json:"jsessionid"`
	UserAgent  string `json:"user_agent,omitempty"`
	// BrowserSource is diagnostic-only metadata for `login --from-browser`;
	// it is not persisted because credentials are the portable contract.
	BrowserSource string `json:"-"`
	// Cookie is the full linkedin.com cookie header captured from the browser
	// (li_at, JSESSIONID, lidc, bcookie, bscookie, ...). Sending the complete set
	// avoids the redirect loop LinkedIn triggers when routing/consent cookies are
	// absent. When empty, the client falls back to li_at + JSESSIONID only.
	Cookie string `json:"cookie,omitempty"`
	// BrowserUserDataDir is the controlled Chrome profile used to refresh this
	// session or run browser-context reads when LinkedIn rejects native replay.
	BrowserUserDataDir string `json:"browser_user_data_dir,omitempty"`
	CapturedAt         string `json:"captured_at,omitempty"`
}

// Client signs and issues Voyager requests with the cloned header set.
type Client struct {
	Base      string
	creds     Creds
	http      *http.Client
	jar       *cookiejar.Jar
	seedJar   sync.Once
	bootstrap sync.Once
	bootErr   error
}

// New builds a Client. UserAgent falls back to DefaultUserAgent when empty.
func New(c Creds) *Client {
	if c.UserAgent == "" {
		c.UserAgent = DefaultUserAgent
	}
	jar, _ := cookiejar.New(nil)
	return &Client{
		Base:  BaseURL,
		creds: c,
		jar:   jar,
		http: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar, // persists Set-Cookie (e.g. lidc datacenter pin) across redirects
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Follow cookie-setting redirects (same API host), but treat a bounce
				// to a login/checkpoint/authwall as an auth failure rather than chasing
				// it to a 200 HTML page that would masquerade as success.
				if isLoginRedirect(req.URL) {
					return errLoginRedirect
				}
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

// ensureCookies seeds the jar against the client's current Base host. Done lazily
// (and once) so tests can point Base at an httptest server before the first call
// and still have cookies delivered.
func (c *Client) ensureCookies() {
	c.seedJar.Do(func() {
		u, err := url.Parse(c.Base)
		if err != nil {
			return
		}
		c.jar.SetCookies(u, parseCookies(c.creds.Cookie, c.creds.LiAt, c.creds.JSESSIONID))
	})
}

// ensureSession guarantees a JSESSIONID (the CSRF token). When one wasn't
// supplied (the autonomous path: only li_at was read from the browser), it
// bootstraps one exactly like a browser does — a single authenticated GET to
// linkedin.com makes the server issue a fresh JSESSIONID via Set-Cookie, which
// the jar captures. This removes any dependency on reading the volatile,
// often-memory-only JSESSIONID cookie from disk.
func (c *Client) ensureSession() error {
	c.ensureCookies()
	c.bootstrap.Do(func() {
		if Csrf(c.creds.JSESSIONID) != "" {
			return // already have a usable token
		}
		root := strings.TrimSuffix(c.Base, "/voyager/api")
		if root == c.Base { // Base wasn't the live API (e.g. tests) — derive host root
			if u, err := url.Parse(c.Base); err == nil {
				root = u.Scheme + "://" + u.Host
			}
		}
		req, err := http.NewRequest(http.MethodGet, root+"/", nil)
		if err != nil {
			c.bootErr = err
			return
		}
		req.Header.Set("user-agent", c.creds.UserAgent)
		req.Header.Set("accept", "text/html,application/xhtml+xml")
		resp, err := c.http.Do(req)
		if err != nil {
			if isLoginRedirectErr(err) {
				c.bootErr = authf("li_at rejected (redirected to login) — re-login")
				return
			}
			c.bootErr = err
			return
		}
		resp.Body.Close()
		u, _ := url.Parse(root + "/")
		for _, ck := range c.jar.Cookies(u) {
			if ck.Name == "JSESSIONID" && ck.Value != "" {
				c.creds.JSESSIONID = ck.Value
			}
		}
		if Csrf(c.creds.JSESSIONID) == "" {
			c.bootErr = authf("could not mint JSESSIONID from linkedin (li_at invalid or expired)")
		}
	})
	return c.bootErr
}

// Csrf derives the csrf-token from the JSESSIONID value. LinkedIn stores
// JSESSIONID wrapped in quotes (e.g. "ajax:123"); the csrf header is the same
// value with surrounding quotes removed.
func Csrf(jsessionID string) string {
	return strings.Trim(jsessionID, `"`)
}

// buildRequest constructs a signed request. Exposed (package-internal) so header
// fidelity can be asserted in tests without a live server.
func (c *Client) buildRequest(method, path string, params url.Values, body io.Reader) (*http.Request, error) {
	u := c.Base + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequest(method, u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("user-agent", c.creds.UserAgent)
	req.Header.Set("accept", "application/vnd.linkedin.normalized+json+2.1")
	req.Header.Set("accept-language", "en-US,en;q=0.9")
	req.Header.Set("x-restli-protocol-version", "2.0.0")
	req.Header.Set("x-li-lang", "en_US")
	// Blend with the real voyager-web client: these device/page hints accompany
	// every browser request. Static plausible values are far less anomalous than
	// their absence.
	req.Header.Set("x-li-track", `{"clientVersion":"1.13.0","mpVersion":"1.13.0","osName":"web","timezoneOffset":0,"deviceFormFactor":"DESKTOP","mpName":"voyager-web","displayDensity":2}`)
	req.Header.Set("x-li-page-instance", "urn:li:page:d_flagship3_feed;"+pageInstanceID())
	req.Header.Set("referer", "https://www.linkedin.com/feed/")
	req.Header.Set("csrf-token", Csrf(c.creds.JSESSIONID))
	// Cookies are carried by the client's jar (so Set-Cookie during redirects is
	// honored), not a static header.
	if body != nil {
		req.Header.Set("content-type", "application/json; charset=UTF-8")
	}
	return req, nil
}

func (c *Client) do(req *http.Request) ([]byte, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		if isLoginRedirectErr(err) {
			return nil, authf("session not authenticated (redirected to login/checkpoint)")
		}
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, authf("voyager returned %d", resp.StatusCode)
	case resp.StatusCode >= 300 && resp.StatusCode < 400:
		return nil, authf("voyager returned %d to %s — session not authenticated",
			resp.StatusCode, resp.Header.Get("location"))
	case resp.StatusCode >= 400:
		return nil, fmt.Errorf("voyager returned %d: %s", resp.StatusCode, snippet(data))
	case strings.Contains(resp.Header.Get("content-type"), "text/html"):
		// A 200 HTML page means we landed on a login/interstitial, not the API.
		return nil, authf("voyager returned HTML, not JSON — session not authenticated")
	}
	return data, nil
}

// GetRaw issues a GET and returns the raw body.
func (c *Client) GetRaw(path string, params url.Values) ([]byte, error) {
	if err := c.ensureSession(); err != nil {
		return nil, err
	}
	req, err := c.buildRequest(http.MethodGet, path, params, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

// PostRaw issues a POST with a JSON payload and returns the raw body.
func (c *Client) PostRaw(path string, params url.Values, payload any) ([]byte, error) {
	if err := c.ensureSession(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&buf).Encode(payload); err != nil {
			return nil, err
		}
	}
	req, err := c.buildRequest(http.MethodPost, path, params, &buf)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

var errLoginRedirect = fmt.Errorf("login redirect")

func isLoginRedirectErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), errLoginRedirect.Error())
}

func isLoginRedirect(u *url.URL) bool {
	p := strings.ToLower(u.Path)
	for _, s := range []string{"/login", "/uas/login", "/checkpoint", "/authwall", "/m/login"} {
		if strings.Contains(p, s) {
			return true
		}
	}
	return false
}

// parseCookies turns a "k=v; k=v" header (and/or the discrete li_at/JSESSIONID)
// into jar cookies for linkedin.com.
func parseCookies(cookieHeader, liAt, jsess string) []*http.Cookie {
	var out []*http.Cookie
	seen := map[string]bool{}
	add := func(name, val string) {
		if name == "" || val == "" || seen[name] {
			return
		}
		seen[name] = true
		// JSESSIONID is stored as a quoted-string ("ajax:..."). Go's cookie
		// serializer rejects literal quotes in a value, so strip them and set
		// Quoted, which re-emits the proper JSESSIONID="ajax:..." form.
		quoted := false
		if len(val) >= 2 && strings.HasPrefix(val, `"`) && strings.HasSuffix(val, `"`) {
			val = val[1 : len(val)-1]
			quoted = true
		}
		val = strings.ReplaceAll(val, `"`, "")
		// No Domain: the jar associates the cookie with the seed URL's host, which
		// keeps this correct for both the live host and httptest in tests.
		out = append(out, &http.Cookie{Name: name, Value: val, Quoted: quoted, Path: "/"})
	}
	for _, part := range strings.Split(cookieHeader, ";") {
		part = strings.TrimSpace(part)
		if i := strings.IndexByte(part, '='); i > 0 {
			if part[:i] == "JSESSIONID" {
				continue
			}
			add(part[:i], part[i+1:])
		}
	}
	add("li_at", liAt)
	add("JSESSIONID", jsess)
	return out
}

// pageInstanceID returns a random UUID-shaped string for x-li-page-instance, as
// the browser sends a fresh one per page view.
func pageInstanceID() string {
	var b [16]byte
	_, _ = crand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func snippet(b []byte) string {
	const n = 200
	if len(b) > n {
		return string(b[:n])
	}
	return string(b)
}
