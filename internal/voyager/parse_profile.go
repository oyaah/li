package voyager

import (
	"encoding/json"
	"strings"
)

// Profile is the trimmed result of a profileView fetch.
type Profile struct {
	Name     string `json:"name"`
	Headline string `json:"headline"`
	Role     string `json:"role"`
}

func (p Profile) Columns() []string { return []string{"name", "headline", "role"} }
func (p Profile) Rows() [][]string  { return [][]string{{p.Name, p.Headline, p.Role}} }

// ParseProfile extracts name/headline/role from a profileView response. The JSON
// paths reflect the pinned schema (SchemaVersion) and need live verification.
// Missing identity fields are a drift error — never a blank Profile.
func ParseProfile(b []byte) (Profile, error) {
	var raw struct {
		Profile struct {
			First    string `json:"firstName"`
			Last     string `json:"lastName"`
			Headline string `json:"headline"`
		} `json:"profile"`
		PositionView struct {
			Elements []struct {
				Title   string `json:"title"`
				Company string `json:"companyName"`
			} `json:"elements"`
		} `json:"positionView"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return Profile{}, driftf("profile: invalid json")
	}
	if raw.Profile.First == "" && raw.Profile.Last == "" {
		return Profile{}, driftf("profile: missing firstName/lastName")
	}
	p := Profile{
		Name:     strings.TrimSpace(raw.Profile.First + " " + raw.Profile.Last),
		Headline: raw.Profile.Headline,
	}
	if len(raw.PositionView.Elements) > 0 {
		e := raw.PositionView.Elements[0]
		p.Role = strings.TrimSpace(strings.TrimSpace(e.Title+" @ "+e.Company))
	}
	return p, nil
}
