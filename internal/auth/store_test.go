package auth

import (
	"errors"
	"testing"

	"github.com/oyaah/li/internal/output"
	"github.com/oyaah/li/internal/voyager"
	"github.com/zalando/go-keyring"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	keyring.MockInit()
	in := voyager.Creds{LiAt: "abc", JSESSIONID: `"ajax:1"`, UserAgent: "UA"}
	if err := Save(in); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != in {
		t.Fatalf("round trip: got %+v want %+v", got, in)
	}
}

func TestLoadEmptyIsAuthError(t *testing.T) {
	keyring.MockInit()
	_, err := Load()
	if !errors.Is(err, output.ErrAuth) {
		t.Fatalf("got %v want ErrAuth", err)
	}
}

func TestLoadIncompleteIsAuthError(t *testing.T) {
	keyring.MockInit()
	if err := Save(voyager.Creds{LiAt: "only-li-at"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); !errors.Is(err, output.ErrAuth) {
		t.Fatalf("got %v want ErrAuth", err)
	}
}
