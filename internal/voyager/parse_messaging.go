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
	if raw.Elements == nil {
		return nil, driftf("inbox: missing 'elements'")
	}
	out := make(Inbox, 0, len(*raw.Elements))
	for _, e := range *raw.Elements {
		out = append(out, Conversation{URN: e.EntityURN, With: e.Name, Snippet: e.Snippet})
	}
	return out, nil
}
