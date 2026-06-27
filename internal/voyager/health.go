package voyager

import (
	"errors"

	"github.com/oyaah/li/internal/output"
)

// Probe statuses reported by `li doctor`.
const (
	StatusOK    = "OK"
	StatusDrift = "DRIFT"
	StatusAuth  = "AUTH-FAIL"
)

// Probe is the health result for one endpoint category.
type Probe struct {
	Endpoint string `json:"endpoint"`
	Status   string `json:"status"`
	Detail   string `json:"detail,omitempty"`
}

// Report is the full doctor output; implements output.Tabular.
type Report struct {
	SchemaVersion string  `json:"schema_version"`
	Probes        []Probe `json:"probes"`
}

func (Report) Columns() []string { return []string{"endpoint", "status", "detail"} }
func (r Report) Rows() [][]string {
	rows := make([][]string, 0, len(r.Probes))
	for _, p := range r.Probes {
		rows = append(rows, []string{p.Endpoint, p.Status, p.Detail})
	}
	return rows
}

func classify(name string, err error) Probe {
	p := Probe{Endpoint: name, Status: StatusOK}
	switch {
	case err == nil:
	case errors.Is(err, output.ErrAuth):
		p.Status, p.Detail = StatusAuth, err.Error()
	case errors.Is(err, output.ErrSchemaDrift):
		p.Status, p.Detail = StatusDrift, err.Error()
	default:
		// An unexpected error against a pinned endpoint is treated as drift —
		// the contract no longer holds as expected.
		p.Status, p.Detail = StatusDrift, err.Error()
	}
	return p
}

// Health probes each pinned endpoint category and returns per-endpoint status.
func (c *Client) Health() Report {
	var probes []Probe

	_, err := c.MeName()
	probes = append(probes, classify("profile", err))

	probes = append(probes, classify("people-search", peopleSearchHealth(c)))

	jp, jParams := JobSearch("test", "")
	probes = append(probes, classify("job-search", getAndParse(c, jp, jParams, func(b []byte) error {
		_, e := ParseJobs(b)
		return e
	})))

	probes = append(probes, classify("messaging", messagingHealth(c)))

	return Report{SchemaVersion: SchemaVersion, Probes: probes}
}

func peopleSearchHealth(c *Client) error {
	if c.Base != BaseURL {
		pp, pParams := PeopleSearch("test", "", "")
		return getAndParse(c, pp, pParams, func(b []byte) error {
			_, e := ParsePeople(b)
			return e
		})
	}
	_, err := SearchPeoplePage(c.creds, "founder", "", "")
	return err
}

func messagingHealth(c *Client) error {
	me, err := c.GetRaw(Me(), nil)
	if err != nil {
		return err
	}
	mailbox := MailboxURN(me)
	if mailbox == "" {
		return driftf("messaging: missing mailbox urn")
	}
	return getAndParse(c, Conversations(mailbox), nil, func(b []byte) error {
		_, e := ParseInbox(b)
		return e
	})
}

func getAndParse(c *Client, path string, params map[string][]string, parse func([]byte) error) error {
	b, err := c.GetRaw(path, params)
	if err != nil {
		return err
	}
	return parse(b)
}

// DoctorError maps a report to an exit-bearing error: auth failures take
// precedence (exit 77), then drift (exit 69); all-OK returns nil.
func DoctorError(r Report) error {
	hasDrift := false
	for _, p := range r.Probes {
		switch p.Status {
		case StatusAuth:
			return output.ErrAuth
		case StatusDrift:
			hasDrift = true
		}
	}
	if hasDrift {
		return output.ErrSchemaDrift
	}
	return nil
}
