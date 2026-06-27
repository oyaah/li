package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/all" // register chrome/firefox/safari/edge readers

	"github.com/oyaah/li/internal/voyager"
)

// FromBrowser reads the LinkedIn session cookies directly from a locally
// installed browser's cookie store (Chrome/Firefox/Safari/Edge), decrypting
// HttpOnly + OS-keychain-protected values. This is the low-detection path: the
// tool reuses the *exact* session your browser already holds, so LinkedIn sees
// no new login, no new device, and (running locally) the same residential IP.
func FromBrowser() (voyager.Creds, error) {
	li, err := readCookie("li_at")
	if err != nil {
		return voyager.Creds{}, err
	}
	js, err := readCookie("JSESSIONID")
	if err != nil {
		return voyager.Creds{}, err
	}
	// Voyager expects JSESSIONID wrapped in quotes; the stored value usually has
	// them, but normalize in case a reader strips them.
	if !strings.HasPrefix(js, `"`) {
		js = `"` + js + `"`
	}
	return voyager.Creds{LiAt: li, JSESSIONID: js, UserAgent: voyager.DefaultUserAgent}, nil
}

func readCookie(name string) (string, error) {
	seq := kooky.TraverseCookies(
		context.Background(),
		kooky.Valid,
		kooky.DomainHasSuffix("linkedin.com"),
		kooky.Name(name),
	).OnlyCookies()
	for c := range seq {
		if c.Value != "" {
			return c.Value, nil
		}
	}
	return "", fmt.Errorf("cookie %q not found in any browser — log into linkedin.com first", name)
}
