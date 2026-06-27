package voyager

import "strings"

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
	_, err := c.PostRaw(pathConversations, nil, payload)
	if err != nil && strings.Contains(err.Error(), "voyager returned 400") {
		return usagef("message rejected by LinkedIn; recipient may not be messageable from this account yet")
	}
	return err
}
