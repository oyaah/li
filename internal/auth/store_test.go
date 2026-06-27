package auth

import (
	"errors"
	"strings"
	"testing"

	"github.com/oyaah/li/internal/output"
	"github.com/oyaah/li/internal/voyager"
	"github.com/zalando/go-keyring"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	keyring.MockInit()
	in := voyager.Creds{LiAt: "abc", JSESSIONID: `"ajax:1"`, UserAgent: "UA", Cookie: "li_at=abc; JSESSIONID=\"ajax:1\""}
	if err := Save(in); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := voyager.Creds{LiAt: "abc", JSESSIONID: `"ajax:1"`, UserAgent: "UA", Cookie: "li_at=abc"}
	if got != want {
		t.Fatalf("round trip: got %+v want %+v", got, want)
	}
}

func TestSaveCompactsCookieHeader(t *testing.T) {
	keyring.MockInit()
	in := voyager.Creds{
		LiAt: "abc",
		Cookie: strings.Repeat("x", 600) + "=bad; " +
			`JSESSIONID="ajax:1"; li_at=abc; bcookie="b"; g_state={"i":0}; lidc="l"`,
	}
	if err := Save(in); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.JSESSIONID != "" || got.Cookie != `li_at=abc; bcookie="b"; lidc="l"` {
		t.Fatalf("got %+v", got)
	}
}

func TestSaveKeepsSmallLinkedInCookies(t *testing.T) {
	keyring.MockInit()
	in := voyager.Creds{
		LiAt:   "abc",
		Cookie: `_uetvid=v; AMCV_14215E3D5995C57C0A495C55%40AdobeOrg=a; li_gc=g; sdui_ver=s; li_at=abc; JSESSIONID="ajax:1"`,
	}
	if err := Save(in); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got.Cookie, "JSESSIONID") ||
		!strings.Contains(got.Cookie, "_uetvid=v") ||
		!strings.Contains(got.Cookie, "AMCV_14215E3D5995C57C0A495C55%40AdobeOrg=a") ||
		!strings.Contains(got.Cookie, "li_gc=g") ||
		!strings.Contains(got.Cookie, "sdui_ver=s") {
		t.Fatalf("cookie = %q", got.Cookie)
	}
}

func TestLoadEmptyIsAuthError(t *testing.T) {
	keyring.MockInit()
	_, err := Load()
	if !errors.Is(err, output.ErrAuth) {
		t.Fatalf("got %v want ErrAuth", err)
	}
}

func TestLoadLiAtOnlyIsValid(t *testing.T) {
	keyring.MockInit()
	if err := Save(voyager.Creds{LiAt: "only-li-at"}); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.LiAt != "only-li-at" || got.JSESSIONID != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestLoadMissingLiAtIsAuthError(t *testing.T) {
	keyring.MockInit()
	if err := Save(voyager.Creds{JSESSIONID: `"ajax:1"`}); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); !errors.Is(err, output.ErrAuth) {
		t.Fatalf("got %v want ErrAuth", err)
	}
}
