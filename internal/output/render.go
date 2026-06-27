package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Format selects how machine data is rendered to stdout.
type Format int

const (
	Human Format = iota // tab table with a header row, for terminals
	JSON                // indented JSON, stable keys
	Plain               // TSV rows, no header — for piping to cut/awk
)

// Tabular is implemented by any result that can render as rows. JSON output
// marshals the value directly; Plain/Human render through these methods.
type Tabular interface {
	Columns() []string
	Rows() [][]string
}

// Printer carries the resolved format and the data/human streams. Data goes to
// Out (stdout); human prose, warnings and progress go to Err (stderr) so a
// piped consumer reading stdout never sees decoration.
type Printer struct {
	Format Format
	Out    io.Writer
	Err    io.Writer
}

// New builds a Printer wired to the real process streams.
func New(f Format) *Printer {
	return &Printer{Format: f, Out: os.Stdout, Err: os.Stderr}
}

// ResolveFormat applies precedence: explicit flags win; conflicting flags are a
// usage error; with no flag a non-TTY stdout defaults to JSON (agent/script
// friendly) and a TTY defaults to Human.
func ResolveFormat(jsonFlag, plainFlag, isTTY bool) (Format, error) {
	if jsonFlag && plainFlag {
		return Human, fmt.Errorf("%w: --json and --plain are mutually exclusive", ErrUsage)
	}
	switch {
	case jsonFlag:
		return JSON, nil
	case plainFlag:
		return Plain, nil
	case !isTTY:
		return JSON, nil
	default:
		return Human, nil
	}
}

// Data renders a result to stdout in the active format.
func (p *Printer) Data(v any) error {
	switch p.Format {
	case JSON:
		enc := json.NewEncoder(p.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	case Plain:
		t, ok := v.(Tabular)
		if !ok {
			return fmt.Errorf("value is not tabular; cannot render --plain")
		}
		return writeTSV(p.Out, t, false)
	default: // Human
		t, ok := v.(Tabular)
		if !ok {
			enc := json.NewEncoder(p.Out)
			enc.SetIndent("", "  ")
			return enc.Encode(v)
		}
		return writeTSV(p.Out, t, true)
	}
}

// Human prints a human-facing line to stderr. Use for confirmations, warnings,
// and jitter notices — never for machine data.
func (p *Printer) Human(format string, args ...any) {
	fmt.Fprintf(p.Err, format+"\n", args...)
}

func writeTSV(w io.Writer, t Tabular, header bool) error {
	if header {
		if _, err := fmt.Fprintln(w, strings.Join(t.Columns(), "\t")); err != nil {
			return err
		}
	}
	for _, row := range t.Rows() {
		if _, err := fmt.Fprintln(w, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return nil
}

// IsTTY reports whether the given file is a character device (a terminal).
func IsTTY(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
