package voyager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	cdpbrowser "github.com/oyaah/li/internal/browser"
)

type BrowserTransport struct {
	Base string
	br   *cdpbrowser.Browser
}

func NewBrowserTransport(ctx context.Context, creds Creds) (*BrowserTransport, error) {
	br, err := launchLinkedInBrowser(ctx, creds, "https://www.linkedin.com/feed/")
	if err != nil {
		return nil, err
	}
	return &BrowserTransport{Base: BaseURL, br: br}, nil
}

func (t *BrowserTransport) Close() error {
	if t == nil || t.br == nil {
		return nil
	}
	return t.br.Close()
}

func (t *BrowserTransport) GetRaw(path string, params url.Values) ([]byte, error) {
	return t.GetRawContext(context.Background(), path, params)
}

func (t *BrowserTransport) GetRawContext(ctx context.Context, path string, params url.Values) ([]byte, error) {
	return t.fetchRaw(ctx, http.MethodGet, path, params, nil)
}

func (t *BrowserTransport) PostRaw(path string, params url.Values, payload any) ([]byte, error) {
	return t.PostRawContext(context.Background(), path, params, payload)
}

func (t *BrowserTransport) PostRawContext(ctx context.Context, path string, params url.Values, payload any) ([]byte, error) {
	var body []byte
	if payload != nil {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(payload); err != nil {
			return nil, err
		}
		body = buf.Bytes()
	}
	return t.fetchRaw(ctx, http.MethodPost, path, params, body)
}

func (t *BrowserTransport) fetchRaw(ctx context.Context, method, path string, params url.Values, body []byte) ([]byte, error) {
	if t.Base == "" {
		t.Base = BaseURL
	}
	u := t.Base + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	input, _ := json.Marshal(map[string]any{
		"url":    u,
		"method": method,
		"body":   string(body),
	})
	raw, err := t.br.Evaluate(ctx, fmt.Sprintf(`(async (input) => {
  try {
    let csrf = "";
    try {
      csrf = ((document.cookie.match(/(?:^|;\s*)JSESSIONID=([^;]+)/) || [])[1] || "");
    } catch (_) {}
    const headers = {
      "accept": "application/vnd.linkedin.normalized+json+2.1",
      "x-restli-protocol-version": "2.0.0",
      "x-li-lang": "en_US"
    };
    if (csrf) headers["csrf-token"] = decodeURIComponent(csrf).replace(/^"|"$/g, "");
    const init = {method: input.method, credentials: "include", headers};
    if (input.body) {
      headers["content-type"] = "application/json; charset=UTF-8";
      init.body = input.body;
    }
    const res = await fetch(input.url, init);
    const text = await res.text();
    return {status: res.status, contentType: res.headers.get("content-type") || "", text};
  } catch (e) {
    return {status: 0, contentType: "", text: String(e)};
  }
})(%s)`, string(input)))
	if err != nil {
		return nil, err
	}
	var res struct {
		Status      int    `json:"status"`
		ContentType string `json:"contentType"`
		Text        string `json:"text"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, err
	}
	switch {
	case res.Status == 0:
		return nil, authf("browser transport failed: %s", res.Text)
	case res.Status == http.StatusUnauthorized || res.Status == http.StatusForbidden:
		return nil, authf("browser transport returned %d", res.Status)
	case res.Status >= 300 && res.Status < 400:
		return nil, authf("browser transport returned %d", res.Status)
	case res.Status >= 400:
		return nil, fmt.Errorf("voyager returned %d: %s", res.Status, snippet([]byte(res.Text)))
	case strings.Contains(res.ContentType, "text/html"):
		return nil, authf("browser transport returned HTML, not JSON")
	}
	return []byte(res.Text), nil
}
