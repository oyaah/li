package voyager

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/oyaah/li/internal/output"
)

// healthServer routes the four probe endpoints. peopleBody lets a test inject a
// drifted shape for people-search; the rest return healthy empty lists.
func healthServer(authFail bool, peopleBody string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authFail {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/me"):
			w.Write([]byte(`{"firstName":"A","lastName":"B","*miniProfile":"urn:li:fs_miniProfile:abc"}`))
		case strings.Contains(r.URL.Path, "/search/blended"):
			w.Write([]byte(peopleBody))
		case strings.Contains(r.URL.Path, "JobCards"):
			w.Write([]byte(`{"elements":[]}`))
		case strings.Contains(r.URL.Path, "/messaging/conversations") || strings.Contains(r.URL.Path, "voyagerMessagingGraphQL"):
			w.Write([]byte(`{"elements":[]}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
}

func healthClient(base string) *Client {
	c := New(Creds{LiAt: "x", JSESSIONID: `"ajax:1"`})
	c.Base = base
	return c
}

func TestHealthAllOK(t *testing.T) {
	srv := healthServer(false, `{"elements":[]}`)
	defer srv.Close()
	r := healthClient(srv.URL).Health()
	if err := DoctorError(r); err != nil {
		t.Fatalf("expected all OK, got %v; report=%+v", err, r)
	}
	if r.SchemaVersion != SchemaVersion {
		t.Fatalf("schema version = %q", r.SchemaVersion)
	}
}

func TestHealthDetectsDrift(t *testing.T) {
	srv := healthServer(false, `{"no_elements":true}`) // people-search drift
	defer srv.Close()
	r := healthClient(srv.URL).Health()
	if !errors.Is(DoctorError(r), output.ErrSchemaDrift) {
		t.Fatalf("expected drift, report=%+v", r)
	}
	var ps Probe
	for _, p := range r.Probes {
		if p.Endpoint == "people-search" {
			ps = p
		}
	}
	if ps.Status != StatusDrift {
		t.Fatalf("people-search status = %q want DRIFT", ps.Status)
	}
}

func TestHealthDetectsAuthFail(t *testing.T) {
	srv := healthServer(true, "")
	defer srv.Close()
	r := healthClient(srv.URL).Health()
	if !errors.Is(DoctorError(r), output.ErrAuth) {
		t.Fatalf("expected auth fail, report=%+v", r)
	}
}
