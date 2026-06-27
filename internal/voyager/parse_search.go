package voyager

import (
	"context"
	"encoding/json"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

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

func SearchPeoplePage(creds Creds, keywords, title, company string) (People, error) {
	pageURL := peopleSearchPageURL(keywords, title, company)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("cookie", cookieHeader(creds))
	req.Header.Set("user-agent", userAgent(creds))
	req.Header.Set("accept", "text/html,application/xhtml+xml")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, authf("search page returned %d", resp.StatusCode)
	}
	if isLoginRedirect(resp.Request.URL) || looksLikeAuthPage(body) {
		return nil, authf("search page returned login/checkpoint HTML")
	}
	if resp.StatusCode >= 400 {
		return nil, driftf("search page returned %d", resp.StatusCode)
	}
	return ParsePeopleSearchHTML(body), nil
}

func peopleSearchPageURL(keywords, title, company string) string {
	q := url.Values{}
	q.Set("keywords", keywords)
	q.Set("origin", "GLOBAL_SEARCH_HEADER")
	if title != "" {
		q.Set("title", title)
	}
	if company != "" {
		q.Set("company", company)
	}
	return "https://www.linkedin.com/search/results/people/?" + q.Encode()
}

func cookieHeader(creds Creds) string {
	cookies := parseCookies(creds.Cookie, creds.LiAt, creds.JSESSIONID)
	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		parts = append(parts, c.String())
	}
	return strings.Join(parts, "; ")
}

func looksLikeAuthPage(body []byte) bool {
	s := strings.ToLower(string(body))
	for _, marker := range []string{"/login", "/uas/login", "/checkpoint", "/authwall", "session_redirect"} {
		if strings.Contains(s, marker) {
			return true
		}
	}
	return false
}

func userAgent(creds Creds) string {
	if creds.UserAgent != "" {
		return creds.UserAgent
	}
	return DefaultUserAgent
}

func ParsePeopleSearchHTML(b []byte) People {
	text := string(b)
	idxs := regexp.MustCompile(`data-view-name="people-search-result"`).FindAllStringIndex(text, -1)
	out := make(People, 0, len(idxs))
	seen := map[string]bool{}
	for i, idx := range idxs {
		start := idx[0]
		end := len(text)
		if i+1 < len(idxs) {
			end = idxs[i+1][0]
		}
		card := text[start:end]
		pid := firstSubmatch(card, `href="https://www\.linkedin\.com/in/([^"/?]+)`)
		if pid == "" || seen[pid] {
			continue
		}
		seen[pid] = true
		name := cleanHTML(firstSubmatch(card, `data-view-name="search-result-lockup-title"[^>]*>([^<]+)`))
		headline := ""
		for _, m := range regexp.MustCompile(`<span>([^<]+)</span>`).FindAllStringSubmatch(card, -1) {
			got := cleanHTML(m[1])
			if got == "" || strings.HasPrefix(got, "•") || strings.Contains(got, "2nd") || strings.Contains(got, "3rd") {
				continue
			}
			if got != name {
				headline = got
				break
			}
		}
		out = append(out, Person{PublicID: pid, Name: name, Headline: headline})
	}
	return out
}

func firstSubmatch(s, pattern string) string {
	m := regexp.MustCompile(pattern).FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func cleanHTML(s string) string {
	s = html.UnescapeString(s)
	return strings.Join(strings.Fields(s), " ")
}
