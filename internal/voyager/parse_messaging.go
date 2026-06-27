package voyager

import "encoding/json"

// Conversation is one inbox thread.
type Conversation struct {
	URN     string `json:"urn"`
	With    string `json:"with"`
	Snippet string `json:"snippet"`
}

// Inbox is a list of conversations; implements output.Tabular.
type Inbox []Conversation

func (Inbox) Columns() []string { return []string{"urn", "with", "snippet"} }
func (in Inbox) Rows() [][]string {
	rows := make([][]string, 0, len(in))
	for _, c := range in {
		rows = append(rows, []string{c.URN, c.With, c.Snippet})
	}
	return rows
}

// ParseInbox parses a conversations response. Missing "elements" is drift;
// empty is a valid empty-inbox outcome.
func ParseInbox(b []byte) (Inbox, error) {
	var raw struct {
		Elements *[]struct {
			EntityURN string `json:"entityUrn"`
			Name      string `json:"name"`
			Snippet   string `json:"snippet"`
		} `json:"elements"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, driftf("inbox: invalid json")
	}
	if raw.Elements != nil {
		out := make(Inbox, 0, len(*raw.Elements))
		for _, e := range *raw.Elements {
			out = append(out, Conversation{URN: e.EntityURN, With: e.Name, Snippet: e.Snippet})
		}
		return out, nil
	}
	return parseGraphQLInbox(b)
}

func parseGraphQLInbox(b []byte) (Inbox, error) {
	var raw struct {
		Data struct {
			Data struct {
				Query struct {
					Elements []string `json:"*elements"`
				} `json:"messengerConversationsByCategoryQuery"`
			} `json:"data"`
		} `json:"data"`
		Included []struct {
			Type         string   `json:"$type"`
			EntityURN    string   `json:"entityUrn"`
			Participants []string `json:"*conversationParticipants"`
			Messages     struct {
				Elements []string `json:"*elements"`
			} `json:"messages"`
			Body struct {
				Text string `json:"text"`
			} `json:"body"`
		} `json:"included"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, driftf("inbox: invalid json")
	}
	if raw.Data.Data.Query.Elements == nil {
		return nil, driftf("inbox: missing 'elements'")
	}
	conversations := map[string]struct {
		Participants []string
		Messages     []string
	}{}
	messages := map[string]string{}
	for _, inc := range raw.Included {
		switch inc.Type {
		case "com.linkedin.messenger.Conversation":
			conversations[inc.EntityURN] = struct {
				Participants []string
				Messages     []string
			}{Participants: inc.Participants, Messages: inc.Messages.Elements}
		case "com.linkedin.messenger.Message":
			messages[inc.EntityURN] = inc.Body.Text
		}
	}
	out := make(Inbox, 0, len(raw.Data.Data.Query.Elements))
	for _, urn := range raw.Data.Data.Query.Elements {
		c := conversations[urn]
		with := ""
		if len(c.Participants) > 0 {
			with = c.Participants[0]
		}
		snippet := ""
		if len(c.Messages) > 0 {
			snippet = messages[c.Messages[0]]
		}
		out = append(out, Conversation{URN: urn, With: with, Snippet: snippet})
	}
	return out, nil
}
