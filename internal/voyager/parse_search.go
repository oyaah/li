package voyager

import "encoding/json"

// Person is one people-search hit.
type Person struct {
	PublicID string `json:"public_id"`
	Name     string `json:"name"`
	Headline string `json:"headline"`
}

// People is a list of search hits; implements output.Tabular.
type People []Person

func (People) Columns() []string { return []string{"public_id", "name", "headline"} }
func (p People) Rows() [][]string {
	rows := make([][]string, 0, len(p))
	for _, e := range p {
		rows = append(rows, []string{e.PublicID, e.Name, e.Headline})
	}
	return rows
}

// ParsePeople parses a people-search response. A missing top-level "elements"
// key is drift; an empty list is a valid no-results outcome (not an error).
func ParsePeople(b []byte) (People, error) {
	var raw struct {
		Elements *[]struct {
			PublicID string `json:"publicIdentifier"`
			Title    struct {
				Text string `json:"text"`
			} `json:"title"`
			Headline struct {
				Text string `json:"text"`
			} `json:"headline"`
		} `json:"elements"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, driftf("people: invalid json")
	}
	if raw.Elements == nil {
		return nil, driftf("people: missing 'elements'")
	}
	out := make(People, 0, len(*raw.Elements))
	for _, e := range *raw.Elements {
		out = append(out, Person{PublicID: e.PublicID, Name: e.Title.Text, Headline: e.Headline.Text})
	}
	return out, nil
}
