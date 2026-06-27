package voyager

import (
	"bytes"
	crand "crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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
}

// Client signs and issues Voyager requests with the cloned header set.
type Client struct {
	Base  string
	creds Creds
	http  *http.Client
}

// New builds a Client. UserAgent falls back to DefaultUserAgent when empty.
func New(c Creds) *Client {
	if c.UserAgent == "" {
		c.UserAgent = DefaultUserAgent
	}
	return &Client{
		Base:  BaseURL,
		creds: c,
		http:  &http.Client{Timeout: 30 * time.Second},
	}
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
	req.Header.Set("cookie", fmt.Sprintf("li_at=%s; JSESSIONID=%s", c.creds.LiAt, c.creds.JSESSIONID))
	if body != nil {
		req.Header.Set("content-type", "application/json; charset=UTF-8")
	}
	return req, nil
}

func (c *Client) do(req *http.Request) ([]byte, error) {
	resp, err := c.http.Do(req)
	if err != nil {
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
	case resp.StatusCode >= 400:
		return nil, fmt.Errorf("voyager returned %d: %s", resp.StatusCode, snippet(data))
	}
	return data, nil
}

// GetRaw issues a GET and returns the raw body.
func (c *Client) GetRaw(path string, params url.Values) ([]byte, error) {
	req, err := c.buildRequest(http.MethodGet, path, params, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

// PostRaw issues a POST with a JSON payload and returns the raw body.
func (c *Client) PostRaw(path string, params url.Values, payload any) ([]byte, error) {
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
