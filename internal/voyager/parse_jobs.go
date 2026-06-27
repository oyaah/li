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
		return nil, driftf("jobs: invalid json")
	}
	if raw.Elements == nil {
		return nil, driftf("jobs: missing 'elements'")
	}
	out := make(Jobs, 0, len(*raw.Elements))
	for _, e := range *raw.Elements {
		out = append(out, Job{JobID: e.JobPostingID, Title: e.Title, Company: e.Company, Location: e.Location})
	}
	return out, nil
}
