package auth

import (
	"encoding/json"
	"fmt"

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
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return keyring.Set(service, user, string(b))
}

// Load reads credentials from the keyring. A missing, corrupt, or incomplete
// entry is reported as ErrAuth (exit 77) so callers prompt the user to log in.
func Load() (voyager.Creds, error) {
	s, err := keyring.Get(service, user)
	if err != nil {
		return voyager.Creds{}, fmt.Errorf("no stored credentials: %w", output.ErrAuth)
	}
	var c voyager.Creds
	if err := json.Unmarshal([]byte(s), &c); err != nil {
		return voyager.Creds{}, fmt.Errorf("corrupt stored credentials: %w", output.ErrAuth)
	}
	if c.LiAt == "" || c.JSESSIONID == "" {
		return voyager.Creds{}, fmt.Errorf("incomplete stored credentials: %w", output.ErrAuth)
	}
	return c, nil
}

// Clear removes stored credentials (used by logout/re-login).
func Clear() error {
	return keyring.Delete(service, user)
}
