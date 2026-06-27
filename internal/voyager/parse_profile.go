package voyager

import (
	"encoding/json"
	"fmt"
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
	p, _, err := parseProfileDetails(b)
	return p, err
}

func parseProfileDetails(b []byte) (Profile, string, error) {
	var raw struct {
		Profile struct {
			First     string `json:"firstName"`
			Last      string `json:"lastName"`
			Headline  string `json:"headline"`
			EntityURN string `json:"entityUrn"`
		} `json:"profile"`
		PositionView struct {
			Elements []struct {
				Title   string `json:"title"`
				Company string `json:"companyName"`
			} `json:"elements"`
		} `json:"positionView"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return Profile{}, "", driftf("profile: invalid json")
	}
	if raw.Profile.First != "" || raw.Profile.Last != "" {
		p := Profile{
			Name:     strings.TrimSpace(raw.Profile.First + " " + raw.Profile.Last),
			Headline: raw.Profile.Headline,
		}
		if len(raw.PositionView.Elements) > 0 {
			e := raw.PositionView.Elements[0]
			p.Role = joinRole(e.Title, e.Company)
		}
		return p, raw.Profile.EntityURN, nil
	}

	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return Profile{}, "", driftf("profile: invalid json")
	}
	p, urn := parseDashProfile(v)
	if p.Name == "" {
		return Profile{}, "", driftf("profile: missing firstName/lastName")
	}
	return p, urn, nil
}

func parseDashProfile(v any) (Profile, string) {
	var profileMap map[string]any
	var role string
	walkJSON(v, func(m map[string]any) {
		if profileMap == nil && (stringField(m, "firstName") != "" || stringField(m, "lastName") != "") {
			profileMap = m
		}
		if role == "" {
			title := stringField(m, "title")
			company := stringField(m, "companyName")
			if company == "" {
				company = nestedStringField(m, "company", "name")
			}
			role = joinRole(title, company)
		}
	})
	if profileMap == nil {
		return Profile{}, ""
	}
	first, last := stringField(profileMap, "firstName"), stringField(profileMap, "lastName")
	return Profile{
		Name:     strings.TrimSpace(first + " " + last),
		Headline: stringField(profileMap, "headline"),
		Role:     role,
	}, stringField(profileMap, "entityUrn")
}

func walkJSON(v any, visit func(map[string]any)) {
	switch x := v.(type) {
	case map[string]any:
		visit(x)
		for _, child := range x {
			walkJSON(child, visit)
		}
	case []any:
		for _, child := range x {
			walkJSON(child, visit)
		}
	}
}

func stringField(m map[string]any, key string) string {
	return textValue(m[key])
}

func nestedStringField(m map[string]any, key, nested string) string {
	child, _ := m[key].(map[string]any)
	if child == nil {
		return ""
	}
	return stringField(child, nested)
}

func textValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	case map[string]any:
		for _, key := range []string{"text", "localized", "name", "title"} {
			if s := textValue(x[key]); s != "" {
				return s
			}
		}
	}
	return ""
}

func joinRole(title, company string) string {
	title, company = strings.TrimSpace(title), strings.TrimSpace(company)
	switch {
	case title != "" && company != "":
		return title + " @ " + company
	case title != "":
		return title
	case company != "":
		return company
	default:
		return ""
	}
}
