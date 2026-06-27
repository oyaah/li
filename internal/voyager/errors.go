package voyager

import (
	"fmt"

	"github.com/oyaah/li/internal/output"
)

// driftf wraps output.ErrSchemaDrift so callers can errors.Is it and the exit
// code maps to DriftErr. Used by parsers when an expected key is absent — we
// fail loud rather than returning a partial or fabricated result.
func driftf(format string, a ...any) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, a...), output.ErrSchemaDrift)
}

// authf wraps output.ErrAuth for 401/403 responses (expired/invalid cookies).
func authf(format string, a ...any) error {
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, a...), output.ErrAuth)
}
