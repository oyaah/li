package output

import "errors"

// sysexits-style exit codes. Kept small and stable so scripts can branch on them.
const (
	OK       = 0
	GenError = 1
	UsageErr = 64 // bad invocation / conflicting flags
	DriftErr = 69 // voyager response shape changed (contract failure)
	RateErr  = 75 // local rate cap reached (soft-block)
	AuthErr  = 77 // missing or expired credentials
)

// Sentinel errors. Commands return these (optionally wrapped) and ExitCode maps
// them to the codes above. We never swallow them into a fabricated success.
var (
	ErrUsage       = errors.New("usage error")
	ErrSchemaDrift = errors.New("voyager schema drift: response shape changed")
	ErrRateBlock   = errors.New("rate cap reached")
	ErrAuth        = errors.New("not authenticated: run `li login`")
)

// ExitCode maps an error to its sysexit code via errors.Is, so wrapped errors
// (fmt.Errorf("...: %w", ErrAuth)) still resolve correctly.
func ExitCode(err error) int {
	switch {
	case err == nil:
		return OK
	case errors.Is(err, ErrUsage):
		return UsageErr
	case errors.Is(err, ErrSchemaDrift):
		return DriftErr
	case errors.Is(err, ErrRateBlock):
		return RateErr
	case errors.Is(err, ErrAuth):
		return AuthErr
	default:
		return GenError
	}
}
