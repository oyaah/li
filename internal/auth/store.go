package auth

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/oyaah/li/internal/output"
	"github.com/oyaah/li/internal/voyager"
	"github.com/zalando/go-keyring"
)

// Credentials are stored in the OS keyring (Keychain / Secret Service /
// Credential Manager) under this service+user, never in plaintext on disk.
const (
	service = "li"
	user    = "default"
)

// Save writes credentials to the keyring as JSON.
func Save(c voyager.Creds) error {
	c.Cookie = compactCookieHeader(c.Cookie, c.LiAt)
	c.BrowserSource = ""
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return keyring.Set(service, user, string(b))
}

func compactCookieHeader(header, liAt string) string {
	seen := map[string]bool{}
	var parts []string
	for _, part := range strings.Split(header, ";") {
		part = strings.TrimSpace(part)
		i := strings.IndexByte(part, '=')
		if i <= 0 {
			continue
		}
		name, value := part[:i], part[i+1:]
		if !safePersistentCookie(name, value) || seen[name] {
			continue
		}
		seen[name] = true
		parts = append(parts, name+"="+value)
	}
	if liAt != "" && !seen["li_at"] {
		parts = append([]string{"li_at=" + liAt}, parts...)
	}
	return strings.Join(parts, "; ")
}

// Load reads credentials from the keyring. A missing, corrupt, or incomplete
// entry is reported as ErrAuth (exit 77) so callers prompt the user to log in.
// JSESSIONID is intentionally optional: the Voyager client can mint a fresh one
// from li_at, which is the durable auth cookie.
func Load() (voyager.Creds, error) {
	s, err := keyring.Get(service, user)
	if err != nil {
		return voyager.Creds{}, fmt.Errorf("no stored credentials: %w", output.ErrAuth)
	}
	var c voyager.Creds
	if err := json.Unmarshal([]byte(s), &c); err != nil {
		return voyager.Creds{}, fmt.Errorf("corrupt stored credentials: %w", output.ErrAuth)
	}
	if c.LiAt == "" {
		return voyager.Creds{}, fmt.Errorf("incomplete stored credentials: %w", output.ErrAuth)
	}
	return c, nil
}

// Clear removes stored credentials (used by logout/re-login).
func Clear() error {
	return keyring.Delete(service, user)
}
