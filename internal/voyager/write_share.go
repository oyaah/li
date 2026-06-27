package voyager

// CreateShare posts an update to the user's own feed. Payload shape is
// drift-prone — verify live.
func (c *Client) CreateShare(text string) error {
	payload := map[string]any{
		"visibility": "PUBLIC",
		"text": map[string]any{
			"text": text,
		},
	}
	_, err := c.PostRaw(Share(), nil, payload)
	return err
}
