package voyager

import (
	"encoding/json"
	"strings"
)

// pathMe is the current-user endpoint, used to validate a session at login.
const pathMe = "/me"

// MeName validates the session by fetching /me and returns a best-effort
// display name. A transport or auth failure is returned as an error (so login
// can reject bad cookies); name extraction is best-effort and an empty name is
// not an error — the greeting is cosmetic, not data output.
func (c *Client) MeName() (string, error) {
	b, err := c.GetRaw(pathMe, nil)
	if err != nil {
		return "", err
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return "", nil
	}
	if n := nameFrom(m); n != "" {
		return n, nil
	}
	// Real /me nests the name under miniProfile.
	if mp, ok := m["miniProfile"].(map[string]any); ok {
		return nameFrom(mp), nil
	}
	return "", nil
}

func MailboxURN(b []byte) string {
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return ""
	}
	if urn := mailboxURNFromMap(m); urn != "" {
		return urn
	}
	if data, ok := m["data"].(map[string]any); ok {
		return mailboxURNFromMap(data)
	}
	return ""
}

func mailboxURNFromMap(m map[string]any) string {
	for _, key := range []string{"*miniProfile", "entityUrn", "objectUrn"} {
		if s, _ := m[key].(string); s != "" {
			if id := urnID(s); id != "" {
				return "urn:li:fsd_profile:" + id
			}
		}
	}
	return ""
}

func nameFrom(m map[string]any) string {
	first, _ := m["firstName"].(string)
	last, _ := m["lastName"].(string)
	return strings.TrimSpace(first + " " + last)
}
