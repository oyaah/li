package cmd

import (
	"errors"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oyaah/li/internal/output"
	"github.com/oyaah/li/internal/safety"
	"github.com/oyaah/li/internal/voyager"
)

func testJitter() *safety.Jitterer {
	return &safety.Jitterer{
		Min: time.Nanosecond, Max: time.Nanosecond,
		Rand: rand.New(rand.NewSource(1)), Sleep: func(time.Duration) {},
	}
}

func testLedger(t *testing.T) *safety.Ledger {
	fixed := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	return &safety.Ledger{Path: filepath.Join(t.TempDir(), "l.json"), Now: func() time.Time { return fixed }}
}

// writeServer records the ordered method+path of each request. profileOK toggles
// whether the warm-up GET returns a resolvable entityUrn.
func writeServer(profileOK bool) (*httptest.Server, *[]string) {
	var mu sync.Mutex
	calls := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		mu.Unlock()
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/identity/profiles") {
			if profileOK {
				w.Write([]byte(`{"profile":{"firstName":"A","lastName":"B","entityUrn":"urn:li:fs_profile:123"}}`))
			} else {
				w.Write([]byte(`{"profile":{"firstName":"A","lastName":"B"}}`)) // no entityUrn -> drift
			}
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	return srv, &calls
}

func discardPrinter() *output.Printer {
	return &output.Printer{Format: output.Human, Out: discard{}, Err: discard{}}
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

func TestConnectWarmUpBeforePost(t *testing.T) {
	srv, calls := writeServer(true)
	defer srv.Close()
	c := clientFor(srv.URL, voyager.Creds{LiAt: "x", JSESSIONID: `"ajax:1"`})
	l := testLedger(t)

	if err := runConnect(c, l, testJitter(), discardPrinter(), "ada", "hi", false); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 2 {
		t.Fatalf("expected 2 calls, got %v", *calls)
	}
	if !strings.HasPrefix((*calls)[0], "GET ") || !strings.Contains((*calls)[0], "/identity/profiles") {
		t.Fatalf("first call should be warm-up GET, got %q", (*calls)[0])
	}
	if !strings.HasPrefix((*calls)[1], "POST ") {
		t.Fatalf("second call should be POST invite, got %q", (*calls)[1])
	}
}

func TestConnectBlockedAtCapMakesNoRequests(t *testing.T) {
	srv, calls := writeServer(true)
	defer srv.Close()
	c := clientFor(srv.URL, voyager.Creds{LiAt: "x", JSESSIONID: `"ajax:1"`})
	l := testLedger(t)
	for i := 0; i < 15; i++ {
		l.Record("connect")
	}
	err := runConnect(c, l, testJitter(), discardPrinter(), "ada", "", false)
	if !errors.Is(err, output.ErrRateBlock) {
		t.Fatalf("got %v want ErrRateBlock", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("blocked connect should make no requests, got %v", *calls)
	}
}

func TestConnectForceOverrides(t *testing.T) {
	srv, calls := writeServer(true)
	defer srv.Close()
	c := clientFor(srv.URL, voyager.Creds{LiAt: "x", JSESSIONID: `"ajax:1"`})
	l := testLedger(t)
	for i := 0; i < 15; i++ {
		l.Record("connect")
	}
	if err := runConnect(c, l, testJitter(), discardPrinter(), "ada", "", true); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 2 {
		t.Fatalf("forced connect should make 2 calls, got %v", *calls)
	}
}

func TestConnectDriftDoesNotRecord(t *testing.T) {
	srv, calls := writeServer(false) // warm-up GET lacks entityUrn
	defer srv.Close()
	c := clientFor(srv.URL, voyager.Creds{LiAt: "x", JSESSIONID: `"ajax:1"`})
	l := testLedger(t)

	err := runConnect(c, l, testJitter(), discardPrinter(), "ada", "", false)
	if !errors.Is(err, output.ErrSchemaDrift) {
		t.Fatalf("got %v want ErrSchemaDrift", err)
	}
	// only the GET happened; no POST
	if len(*calls) != 1 || !strings.HasPrefix((*calls)[0], "GET ") {
		t.Fatalf("drift should stop before POST, calls=%v", *calls)
	}
	d, _, _ := l.Guard("connect", false)
	if d != safety.Proceed {
		t.Fatalf("ledger should not have recorded a connect on drift")
	}
}

func TestMsgResolvesThenPosts(t *testing.T) {
	srv, calls := writeServer(true)
	defer srv.Close()
	c := clientFor(srv.URL, voyager.Creds{LiAt: "x", JSESSIONID: `"ajax:1"`})
	l := testLedger(t)
	if err := runMsg(c, l, testJitter(), discardPrinter(), "ada", "hello", false); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 2 || !strings.Contains((*calls)[1], "/messaging/conversations") {
		t.Fatalf("msg flow calls = %v", *calls)
	}
}

func TestPostCreatesShare(t *testing.T) {
	srv, calls := writeServer(true)
	defer srv.Close()
	c := clientFor(srv.URL, voyager.Creds{LiAt: "x", JSESSIONID: `"ajax:1"`})
	l := testLedger(t)
	if err := runPost(c, l, testJitter(), discardPrinter(), "hello world", false); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 1 || !strings.Contains((*calls)[0], "/contentcreation/normShares") {
		t.Fatalf("post flow calls = %v", *calls)
	}
}
