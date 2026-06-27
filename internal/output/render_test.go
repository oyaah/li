package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

type fakeTable struct{}

func (fakeTable) Columns() []string { return []string{"name", "role"} }
func (fakeTable) Rows() [][]string  { return [][]string{{"Ada", "Eng"}, {"Bob", "PM"}} }

func newTestPrinter(f Format) (*Printer, *bytes.Buffer, *bytes.Buffer) {
	var out, errb bytes.Buffer
	return &Printer{Format: f, Out: &out, Err: &errb}, &out, &errb
}

func TestDataJSONGoesToStdoutOnly(t *testing.T) {
	p, out, errb := newTestPrinter(JSON)
	if err := p.Data(map[string]string{"name": "Ada"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("stdout not valid json: %v", err)
	}
	if got["name"] != "Ada" {
		t.Fatalf("got %v", got)
	}
	if errb.Len() != 0 {
		t.Fatalf("stderr should be empty, got %q", errb.String())
	}
}

func TestDataPlainIsTSVNoHeader(t *testing.T) {
	p, out, _ := newTestPrinter(Plain)
	if err := p.Data(fakeTable{}); err != nil {
		t.Fatal(err)
	}
	want := "Ada\tEng\nBob\tPM\n"
	if out.String() != want {
		t.Fatalf("plain output = %q want %q", out.String(), want)
	}
}

func TestDataHumanHasHeader(t *testing.T) {
	p, out, _ := newTestPrinter(Human)
	if err := p.Data(fakeTable{}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "name\trole\n") {
		t.Fatalf("human output missing header: %q", out.String())
	}
}

func TestDataPlainNonTabularErrors(t *testing.T) {
	p, _, _ := newTestPrinter(Plain)
	if err := p.Data(map[string]string{"a": "b"}); err == nil {
		t.Fatal("expected error for non-tabular plain output")
	}
}

func TestResolveFormat(t *testing.T) {
	cases := []struct {
		name             string
		jsonF, plainF, tty bool
		want             Format
		wantErr          bool
	}{
		{"non-tty defaults json", false, false, false, JSON, false},
		{"tty defaults human", false, false, true, Human, false},
		{"json flag wins", true, false, true, JSON, false},
		{"plain flag wins", false, true, true, Plain, false},
		{"conflict errors", true, true, false, Human, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ResolveFormat(c.jsonF, c.plainF, c.tty)
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				if !errors.Is(err, ErrUsage) {
					t.Fatalf("conflict should wrap ErrUsage, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestExitCodeMapping(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{nil, OK},
		{fmt.Errorf("wrap: %w", ErrAuth), AuthErr},
		{fmt.Errorf("wrap: %w", ErrSchemaDrift), DriftErr},
		{fmt.Errorf("wrap: %w", ErrRateBlock), RateErr},
		{fmt.Errorf("wrap: %w", ErrUsage), UsageErr},
		{errors.New("random"), GenError},
	}
	for _, c := range cases {
		if got := ExitCode(c.err); got != c.want {
			t.Fatalf("ExitCode(%v) = %d want %d", c.err, got, c.want)
		}
	}
}
