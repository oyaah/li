package cmd

import (
	"errors"
	"testing"

	"github.com/oyaah/li/internal/output"
)

func TestNormalizeProfileID(t *testing.T) {
	tests := map[string]string{
		"kshitij-gupta-280131280":                               "kshitij-gupta-280131280",
		"https://www.linkedin.com/in/kshitij-gupta-280131280/":  "kshitij-gupta-280131280",
		"https://linkedin.com/in/kshitij-gupta-280131280?trk=x": "kshitij-gupta-280131280",
	}
	for in, want := range tests {
		got, err := normalizeProfileID(in)
		if err != nil {
			t.Fatalf("normalizeProfileID(%q) error: %v", in, err)
		}
		if got != want {
			t.Fatalf("normalizeProfileID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeProfileIDRejectsBadInput(t *testing.T) {
	tests := []string{"", "https://example.com/in/ada", "https://linkedin.com/company/openai", "a/b"}
	for _, in := range tests {
		if _, err := normalizeProfileID(in); !errors.Is(err, output.ErrUsage) {
			t.Fatalf("normalizeProfileID(%q) err = %v, want ErrUsage", in, err)
		}
	}
}
