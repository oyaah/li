package safety

import (
	"errors"
	"io"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/oyaah/li/internal/output"
)

func tmpLedger(t *testing.T) *Ledger {
	t.Helper()
	fixed := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	return &Ledger{
		Path: filepath.Join(t.TempDir(), "ledger.json"),
		Now:  func() time.Time { return fixed },
	}
}

func TestGuardUnderCapProceeds(t *testing.T) {
	l := tmpLedger(t)
	d, _, err := l.Guard("connect", false)
	if err != nil {
		t.Fatal(err)
	}
	if d != Proceed {
		t.Fatalf("got %v want Proceed", d)
	}
}

func TestGuardWarnsAtThreshold(t *testing.T) {
	l := tmpLedger(t)
	// daily cap connect = 15; 80% = 12.
	for i := 0; i < 12; i++ {
		if err := l.Record("connect"); err != nil {
			t.Fatal(err)
		}
	}
	d, msg, _ := l.Guard("connect", false)
	if d != Warn {
		t.Fatalf("got %v (%q) want Warn", d, msg)
	}
}

func TestGuardBlocksAtCapAndDoesNotCount(t *testing.T) {
	l := tmpLedger(t)
	for i := 0; i < 15; i++ {
		l.Record("connect")
	}
	d, _, _ := l.Guard("connect", false)
	if d != Block {
		t.Fatalf("got %v want Block", d)
	}
	// Guard must not have added an entry.
	es, _ := l.load()
	if len(es) != 15 {
		t.Fatalf("Guard mutated ledger: %d entries", len(es))
	}
}

func TestGuardForceOverridesBlock(t *testing.T) {
	l := tmpLedger(t)
	for i := 0; i < 15; i++ {
		l.Record("connect")
	}
	d, _, _ := l.Guard("connect", true)
	if d == Block {
		t.Fatal("force should not Block")
	}
}

func TestRollingWindowExcludesOld(t *testing.T) {
	l := tmpLedger(t)
	old := l.now().Add(-8 * 24 * time.Hour) // outside the weekly window
	for i := 0; i < 90; i++ {
		l.Append("connect", old)
	}
	d, _, _ := l.Guard("connect", false)
	if d != Proceed {
		t.Fatalf("old entries should not count; got %v", d)
	}
}

func TestWeeklyCapAcrossDays(t *testing.T) {
	l := tmpLedger(t)
	// 80 connects spread over the past 6 days (each day < daily cap 15)
	// trips the weekly cap (80).
	for day := 1; day <= 6; day++ {
		at := l.now().Add(-time.Duration(day) * 24 * time.Hour).Add(time.Hour)
		for i := 0; i < 14; i++ {
			l.Append("connect", at)
		}
	}
	d, msg, _ := l.Guard("connect", false)
	if d != Block {
		t.Fatalf("got %v (%q) want Block from weekly cap", d, msg)
	}
}

func TestConcurrentAppendNoCorruption(t *testing.T) {
	l := tmpLedger(t)
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.Record("msg")
		}()
	}
	wg.Wait()
	es, err := l.load()
	if err != nil {
		t.Fatalf("ledger corrupt: %v", err)
	}
	if len(es) != 30 {
		t.Fatalf("got %d entries want 30", len(es))
	}
}

func TestGuardOrBlockReturnsRateError(t *testing.T) {
	l := tmpLedger(t)
	for i := 0; i < 15; i++ {
		l.Record("connect")
	}
	p := &output.Printer{Format: output.Human, Out: io.Discard, Err: io.Discard}
	err := l.GuardOrBlock(p, "connect", false)
	if !errors.Is(err, output.ErrRateBlock) {
		t.Fatalf("got %v want ErrRateBlock", err)
	}
}
