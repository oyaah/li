package voyager

import "encoding/json"

// ResolveURN fetches a profile and returns its entity URN. This doubles as the
// warm-up GET before a connect invite (a real browser loads the profile first).
// A missing URN is drift — we never POST an invite against a guessed URN.
func (c *Client) ResolveURN(publicID string) (string, error) {
	b, err := c.GetRaw(ProfileView(publicID), nil)
	if err != nil {
		return "", err
	}
	var raw struct {
		Profile struct {
			EntityURN string `json:"entityUrn"`
		} `json:"profile"`
	}
	if json.Unmarshal(b, &raw) != nil || raw.Profile.EntityURN == "" {
		return "", driftf("connect: cannot resolve profile urn for %q", publicID)
	}
	return raw.Profile.EntityURN, nil
}

// SendInvite POSTs a connection invitation. Payload shape is drift-prone — verify
// live and re-pin SchemaVersion if LinkedIn changes it.
func (c *Client) SendInvite(inviteeURN, note string) error {
	payload := map[string]any{
		"invitee": map[string]any{
			"com.linkedin.voyager.growth.invitation.InviteeProfile": map[string]any{
				"profileId": inviteeURN,
			},
		},
	}
	if note != "" {
		payload["message"] = note
	}
	_, err := c.PostRaw(Invite(), nil, payload)
	return err
}
