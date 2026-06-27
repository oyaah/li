package voyager

// SendMessage creates a message to a recipient (a new conversation thread).
// Payload shape is drift-prone — verify live.
func (c *Client) SendMessage(recipientURN, text string) error {
	payload := map[string]any{
		"eventCreate": map[string]any{
			"value": map[string]any{
				"com.linkedin.voyager.messaging.create.MessageCreate": map[string]any{
					"body": text,
				},
			},
		},
		"recipients": []string{recipientURN},
	}
	_, err := c.PostRaw(Conversations(), nil, payload)
	return err
}
