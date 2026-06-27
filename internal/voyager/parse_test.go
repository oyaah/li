package voyager

import (
	"errors"
	"testing"

	"github.com/oyaah/li/internal/output"
)

func TestParseProfile(t *testing.T) {
	b := []byte(`{
		"profile":{"firstName":"Ada","lastName":"Lovelace","headline":"Mathematician"},
		"positionView":{"elements":[{"title":"Analyst","companyName":"Babbage Co"}]}
	}`)
	p, err := ParseProfile(b)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "Ada Lovelace" || p.Headline != "Mathematician" || p.Role != "Analyst @ Babbage Co" {
		t.Fatalf("got %+v", p)
	}
}

func TestParseProfileDriftMissingName(t *testing.T) {
	_, err := ParseProfile([]byte(`{"profile":{"headline":"x"}}`))
	if !errors.Is(err, output.ErrSchemaDrift) {
		t.Fatalf("got %v want drift", err)
	}
}

func TestParsePeople(t *testing.T) {
	b := []byte(`{"elements":[
		{"publicIdentifier":"ada","title":{"text":"Ada"},"headline":{"text":"Math"}},
		{"publicIdentifier":"bob","title":{"text":"Bob"},"headline":{"text":"PM"}}
	]}`)
	p, err := ParsePeople(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 2 || p[0].PublicID != "ada" || p[1].Name != "Bob" {
		t.Fatalf("got %+v", p)
	}
}

func TestParsePeopleEmptyIsNotError(t *testing.T) {
	p, err := ParsePeople([]byte(`{"elements":[]}`))
	if err != nil {
		t.Fatalf("empty results should not error: %v", err)
	}
	if len(p) != 0 {
		t.Fatalf("expected 0 hits got %d", len(p))
	}
}

func TestParsePeopleDriftMissingElements(t *testing.T) {
	_, err := ParsePeople([]byte(`{"data":{}}`))
	if !errors.Is(err, output.ErrSchemaDrift) {
		t.Fatalf("got %v want drift", err)
	}
}

func TestParseJobs(t *testing.T) {
	b := []byte(`{"elements":[{"jobPostingId":"123","title":"SWE","companyName":"X","formattedLocation":"NYC"}]}`)
	j, err := ParseJobs(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(j) != 1 || j[0].JobID != "123" || j[0].Location != "NYC" {
		t.Fatalf("got %+v", j)
	}
}

func TestParseJobsDrift(t *testing.T) {
	_, err := ParseJobs([]byte(`{"nope":1}`))
	if !errors.Is(err, output.ErrSchemaDrift) {
		t.Fatalf("got %v want drift", err)
	}
}

func TestParseInbox(t *testing.T) {
	b := []byte(`{"elements":[{"entityUrn":"urn:li:conv:1","name":"Ada","snippet":"hi"}]}`)
	in, err := ParseInbox(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(in) != 1 || in[0].With != "Ada" || in[0].URN != "urn:li:conv:1" {
		t.Fatalf("got %+v", in)
	}
}

func TestParseInboxDrift(t *testing.T) {
	_, err := ParseInbox([]byte(`{}`))
	if !errors.Is(err, output.ErrSchemaDrift) {
		t.Fatalf("got %v want drift", err)
	}
}
