package cmd

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oyaah/li/internal/auth"
	"github.com/oyaah/li/internal/output"
	"github.com/oyaah/li/internal/voyager"
	"github.com/zalando/go-keyring"
)

func init() {
	// Silence output during tests.
	out = &output.Printer{Format: output.Human, Out: io.Discard, Err: io.Discard}
}

func clientFor(base string, creds voyager.Creds) *voyager.Client {
	c := voyager.New(creds)
	c.Base = base
	return c
}

func TestRunLoginValidStores(t *testing.T) {
	keyring.MockInit()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"firstName":"Ada","lastName":"Lovelace"}`))
	}))
	defer srv.Close()

	creds := voyager.Creds{LiAt: "x", JSESSIONID: `"ajax:1"`}
	if err := runLogin(creds, clientFor(srv.URL, creds)); err != nil {
		t.Fatal(err)
	}
	got, err := auth.Load()
	if err != nil {
		t.Fatalf("expected stored creds: %v", err)
	}
	if got.LiAt != "x" {
		t.Fatalf("stored %+v", got)
	}
}

func TestRunLoginInvalidDoesNotStore(t *testing.T) {
	keyring.MockInit()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	creds := voyager.Creds{LiAt: "bad", JSESSIONID: `"ajax:bad"`}
	err := runLogin(creds, clientFor(srv.URL, creds))
	if !errors.Is(err, output.ErrAuth) {
		t.Fatalf("got %v want ErrAuth", err)
	}
	if _, lerr := auth.Load(); !errors.Is(lerr, output.ErrAuth) {
		t.Fatalf("creds should not be stored; Load err = %v", lerr)
	}
}
