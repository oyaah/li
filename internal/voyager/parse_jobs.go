package voyager

import "encoding/json"

// Job is one job-search hit.
type Job struct {
	JobID    string `json:"job_id"`
	Title    string `json:"title"`
	Company  string `json:"company"`
	Location string `json:"location"`
}

// Jobs is a list of jobs; implements output.Tabular.
type Jobs []Job

func (Jobs) Columns() []string { return []string{"job_id", "title", "company", "location"} }
func (js Jobs) Rows() [][]string {
	rows := make([][]string, 0, len(js))
	for _, j := range js {
		rows = append(rows, []string{j.JobID, j.Title, j.Company, j.Location})
	}
	return rows
}

// ParseJobs parses a job-search response. Missing "elements" is drift; empty is
// a valid no-results outcome.
func ParseJobs(b []byte) (Jobs, error) {
	var raw struct {
		Elements *[]struct {
			JobPostingID string `json:"jobPostingId"`
			Title        string `json:"title"`
			Company      string `json:"companyName"`
			Location     string `json:"formattedLocation"`
		} `json:"elements"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, driftf("jobs: invalid json: %v", err)
	}
	if raw.Elements != nil {
		out := make(Jobs, 0, len(*raw.Elements))
		for _, e := range *raw.Elements {
			out = append(out, Job{JobID: e.JobPostingID, Title: e.Title, Company: e.Company, Location: e.Location})
		}
		return out, nil
	}
	return parseDashJobs(b)
}

func parseDashJobs(b []byte) (Jobs, error) {
	var raw struct {
		Data struct {
			Elements []struct {
				JobCardUnion struct {
					JobPostingCard string `json:"*jobPostingCard"`
				} `json:"jobCardUnion"`
			} `json:"elements"`
		} `json:"data"`
		Included []json.RawMessage `json:"included"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, driftf("jobs: invalid json: %v", err)
	}
	if raw.Data.Elements == nil {
		return nil, driftf("jobs: missing 'elements'")
	}
	byURN := map[string]dashJobCard{}
	for _, item := range raw.Included {
		var inc dashJobCard
		if json.Unmarshal(item, &inc) != nil || inc.JobPosting == "" {
			continue
		}
		byURN[inc.EntityURN] = inc
	}
	out := make(Jobs, 0, len(raw.Data.Elements))
	for _, e := range raw.Data.Elements {
		card, ok := byURN[e.JobCardUnion.JobPostingCard]
		if !ok {
			continue
		}
		out = append(out, Job{
			JobID:    urnID(card.JobPosting),
			Title:    card.Title.Text,
			Company:  card.PrimaryDescription.Text,
			Location: card.SecondaryDescription.Text,
		})
	}
	return out, nil
}

type textVM struct {
	Text string `json:"text"`
}

type dashJobCard struct {
	EntityURN            string `json:"entityUrn"`
	JobPosting           string `json:"*jobPosting"`
	Title                textVM `json:"title"`
	PrimaryDescription   textVM `json:"primaryDescription"`
	SecondaryDescription textVM `json:"secondaryDescription"`
}

func urnID(urn string) string {
	for i := len(urn) - 1; i >= 0; i-- {
		if urn[i] == ':' {
			return urn[i+1:]
		}
	}
	return urn
}
