package safety

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/oyaah/li/internal/output"
)

// Decision is the outcome of a pre-action cap check.
type Decision int

const (
	Proceed Decision = iota // under thresholds
	Warn                    // approaching a cap (>=80%), still allowed
	Block                   // at/over a hard cap; allowed only with --force
)

// cap holds per-action daily and weekly ceilings. A 0 disables that window.
// Defaults sit ~20% below known-safe thresholds (LinkedIn's invite cap is
// ~100/week, so weekly connect is 80).
type cap struct {
	DailyMax  int
	WeeklyMax int
}

var caps = map[string]cap{
	"connect": {DailyMax: 15, WeeklyMax: 80},
	"msg":     {DailyMax: 40, WeeklyMax: 0},
	"post":    {DailyMax: 8, WeeklyMax: 0},
}

const warnRatio = 0.8

type entry struct {
	Action string    `json:"action"`
	At     time.Time `json:"at"`
}

// Ledger is a JSON-backed rolling-window action counter. Now is injectable for
// tests; a nil Now uses time.Now.
type Ledger struct {
	Path string
	Now  func() time.Time

	mu sync.Mutex
}

// DefaultLedger returns a Ledger at <user-config>/li/ledger.json.
func DefaultLedger() (*Ledger, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return &Ledger{Path: filepath.Join(dir, "li", "ledger.json")}, nil
}

func (l *Ledger) now() time.Time {
	if l.Now != nil {
		return l.Now()
	}
	return time.Now()
}

func (l *Ledger) load() ([]entry, error) {
	b, err := os.ReadFile(l.Path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var es []entry
	if err := json.Unmarshal(b, &es); err != nil {
		return nil, fmt.Errorf("corrupt ledger %s: %w", l.Path, err)
	}
	return es, nil
}

// atomic write: temp file in the same dir + rename.
func (l *Ledger) save(es []entry) error {
	if err := os.MkdirAll(filepath.Dir(l.Path), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(es)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(l.Path), "ledger-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, l.Path)
}

func countInWindow(es []entry, action string, since time.Time) int {
	n := 0
	for _, e := range es {
		if e.Action == action && e.At.After(since) {
			n++
		}
	}
	return n
}

// Guard checks the caps for an action without recording it. The caller records
// only after the action succeeds, so a blocked or failed action never inflates
// the count.
func (l *Ledger) Guard(action string, force bool) (Decision, string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	es, err := l.load()
	if err != nil {
		return Block, "", err
	}
	c, ok := caps[action]
	if !ok {
		return Proceed, "", nil
	}
	now := l.now()

	type win struct {
		name string
		max  int
		dur  time.Duration
	}
	wins := []win{
		{"day", c.DailyMax, 24 * time.Hour},
		{"week", c.WeeklyMax, 7 * 24 * time.Hour},
	}

	decision := Proceed
	msg := ""
	for _, w := range wins {
		if w.max <= 0 {
			continue
		}
		cnt := countInWindow(es, action, now.Add(-w.dur))
		switch {
		case cnt >= w.max:
			if !force {
				return Block, fmt.Sprintf("%s cap reached: %d/%d this %s — use --force to override",
					action, cnt, w.max, w.name), nil
			}
			if decision < Warn {
				decision = Warn
			}
			msg = fmt.Sprintf("FORCED past %s cap: %d/%d this %s", action, cnt, w.max, w.name)
		case float64(cnt) >= warnRatio*float64(w.max):
			if decision < Warn {
				decision = Warn
				msg = fmt.Sprintf("approaching %s cap: %d/%d this %s", action, cnt, w.max, w.name)
			}
		}
	}
	return decision, msg, nil
}

// Record appends a successful action at the current time.
func (l *Ledger) Record(action string) error {
	return l.Append(action, l.now())
}

// Append adds an action at a specific time (used by Record and by tests).
func (l *Ledger) Append(action string, at time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	es, err := l.load()
	if err != nil {
		return err
	}
	es = append(es, entry{Action: action, At: at})
	return l.save(es)
}

// GuardOrBlock is a convenience used by write commands: it runs Guard, emits the
// warning/notice via the printer, and returns ErrRateBlock when blocked.
func (l *Ledger) GuardOrBlock(p *output.Printer, action string, force bool) error {
	d, msg, err := l.Guard(action, force)
	if err != nil {
		return err
	}
	switch d {
	case Block:
		return fmt.Errorf("%s: %w", msg, output.ErrRateBlock)
	case Warn:
		if msg != "" {
			p.Human("warning: %s", msg)
		}
	}
	return nil
}
