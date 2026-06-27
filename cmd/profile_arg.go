package cmd

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/oyaah/li/internal/output"
)

func normalizeProfileID(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("%w: empty profile id", output.ErrUsage)
	}
	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err != nil {
			return "", fmt.Errorf("%w: invalid LinkedIn profile URL", output.ErrUsage)
		}
		host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
		if host != "linkedin.com" {
			return "", fmt.Errorf("%w: expected linkedin.com profile URL", output.ErrUsage)
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 2 || parts[0] != "in" || parts[1] == "" {
			return "", fmt.Errorf("%w: expected linkedin.com/in/<publicId>", output.ErrUsage)
		}
		s = parts[1]
	}
	s = strings.Trim(s, "/")
	if strings.ContainsAny(s, "?#") || strings.Contains(s, "/") {
		return "", fmt.Errorf("%w: expected publicId or linkedin.com/in/<publicId>", output.ErrUsage)
	}
	return s, nil
}
