package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	cdpbrowser "github.com/oyaah/li/internal/browser"
	"github.com/oyaah/li/internal/output"
	"github.com/oyaah/li/internal/voyager"
)

type BrowserLoginOptions struct {
	Timeout          time.Duration
	ChromePath       string
	UserDataDir      string
	ProfileDirectory string
	StartURL         string
}

type BrowserLoginResult struct {
	Creds voyager.Creds
	Name  string
}

func LoginWithBrowser(ctx context.Context, opts BrowserLoginOptions) (BrowserLoginResult, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Minute
	}
	if opts.UserDataDir == "" {
		opts.UserDataDir = cdpbrowser.DefaultUserDataDir()
	}
	if opts.StartURL == "" {
		opts.StartURL = "https://www.linkedin.com/login/?session_redirect=https%3A%2F%2Fwww.linkedin.com%2Ffeed%2F"
	}
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	br, err := cdpbrowser.Launch(ctx, cdpbrowser.Options{
		ChromePath:       opts.ChromePath,
		UserDataDir:      opts.UserDataDir,
		ProfileDirectory: opts.ProfileDirectory,
		StartURL:         opts.StartURL,
		StartupTimeout:   30 * time.Second,
	})
	if err != nil {
		return BrowserLoginResult{}, err
	}
	defer br.Close()

	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		status, err := browserVoyagerMe(ctx, br)
		if err == nil && status.authenticated() {
			creds, cerr := credsFromControlledBrowser(ctx, br, opts.UserDataDir)
			if cerr != nil {
				return BrowserLoginResult{}, cerr
			}
			return BrowserLoginResult{Creds: creds, Name: status.name()}, nil
		}
		select {
		case <-ctx.Done():
			return BrowserLoginResult{}, fmt.Errorf("browser login timed out after %s: %w", opts.Timeout, output.ErrAuth)
		case <-tick.C:
		}
	}
}

type browserMeStatus struct {
	Status      int    `json:"status"`
	ContentType string `json:"contentType"`
	Text        string `json:"text"`
	Location    string `json:"location"`
	Error       string `json:"error"`
}

func (s browserMeStatus) authenticated() bool {
	return s.Status == 200 && strings.Contains(s.ContentType, "json") && strings.Contains(s.Text, "{")
}

func (s browserMeStatus) name() string {
	var m map[string]any
	if json.Unmarshal([]byte(s.Text), &m) != nil {
		return ""
	}
	if first, _ := m["firstName"].(string); first != "" {
		last, _ := m["lastName"].(string)
		return strings.TrimSpace(first + " " + last)
	}
	if mp, ok := m["miniProfile"].(map[string]any); ok {
		first, _ := mp["firstName"].(string)
		last, _ := mp["lastName"].(string)
		return strings.TrimSpace(first + " " + last)
	}
	return ""
}

func browserVoyagerMe(ctx context.Context, br *cdpbrowser.Browser) (browserMeStatus, error) {
	raw, err := br.Evaluate(ctx, `(async () => {
  try {
    const csrf = ((document.cookie.match(/(?:^|;\s*)JSESSIONID=([^;]+)/) || [])[1] || "");
    const token = decodeURIComponent(csrf).replace(/^"|"$/g, "");
    const res = await fetch("https://www.linkedin.com/voyager/api/me", {
      credentials: "include",
      headers: {
        "accept": "application/vnd.linkedin.normalized+json+2.1",
        "csrf-token": token,
        "x-restli-protocol-version": "2.0.0"
      }
    });
    const text = await res.text();
    return {status: res.status, contentType: res.headers.get("content-type") || "", text: text.slice(0, 4000), location: location.href};
  } catch (e) {
    return {status: 0, contentType: "", text: "", location: location.href, error: String(e)};
  }
})()`)
	if err != nil {
		return browserMeStatus{}, err
	}
	var status browserMeStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		return browserMeStatus{}, err
	}
	return status, nil
}

func credsFromControlledBrowser(ctx context.Context, br *cdpbrowser.Browser, userDataDir string) (voyager.Creds, error) {
	cookies, err := br.Cookies(ctx, "https://www.linkedin.com/", "https://www.linkedin.com/voyager/api/me")
	if err != nil {
		return voyager.Creds{}, err
	}
	byName := map[string]string{}
	var order []string
	for _, c := range cookies {
		if c.Value == "" || !usableLinkedInDomain(c.Domain) {
			continue
		}
		if _, exists := byName[c.Name]; exists {
			continue
		}
		byName[c.Name] = c.Value
		order = append(order, c.Name)
	}
	liAt := byName["li_at"]
	if liAt == "" {
		return voyager.Creds{}, fmt.Errorf("authenticated browser did not expose li_at: %w", output.ErrAuth)
	}
	js := byName["JSESSIONID"]
	if js != "" && !strings.HasPrefix(js, `"`) {
		js = `"` + js + `"`
	}
	parts := make([]string, 0, len(order))
	for _, name := range order {
		value := byName[name]
		if name == "JSESSIONID" || safePersistentCookie(name, value) {
			parts = append(parts, name+"="+value)
		}
	}
	ua := voyager.DefaultUserAgent
	if raw, err := br.Evaluate(ctx, `navigator.userAgent`); err == nil {
		var got string
		if json.Unmarshal(raw, &got) == nil && got != "" {
			ua = got
		}
	}
	return voyager.Creds{
		LiAt:               liAt,
		JSESSIONID:         js,
		UserAgent:          ua,
		Cookie:             strings.Join(parts, "; "),
		BrowserSource:      userDataDir,
		BrowserUserDataDir: userDataDir,
		CapturedAt:         time.Now().UTC().Format(time.RFC3339),
	}, nil
}
