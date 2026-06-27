package auth

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/browserutils/kooky"
	chromecookies "github.com/browserutils/kooky/browser/chrome"

	"github.com/oyaah/li/internal/voyager"
)

// FromBrowser reads the LinkedIn session cookies directly from Chrome's local
// cookie stores across profiles, decrypting
// HttpOnly + OS-keychain-protected values. This is the low-detection path: the
// tool reuses the *exact* session your browser already holds, so LinkedIn sees
// no new login, no new device, and (running locally) the same residential IP.
// profileCookies groups one browser-profile's linkedin cookies.
type profileCookies struct {
	source string
	byName map[string]string
	order  []string
}

type BrowserOptions struct {
	Timeout    time.Duration
	Profile    string
	CookieFile string
}

// FromBrowser reads the LinkedIn session from a single browser profile. It must
// pick li_at and JSESSIONID from the SAME profile — pairing cookies across
// profiles yields a mismatched, rejected (401) session. It selects the profile
// that has both, preferring one with an active JSESSIONID.
func FromBrowser() (voyager.Creds, error) {
	return FromBrowserWithTimeout(15 * time.Second)
}

// FromBrowserWithTimeout is FromBrowser with a caller-controlled scan timeout.
func FromBrowserWithTimeout(timeout time.Duration) (voyager.Creds, error) {
	return FromBrowserWithOptions(BrowserOptions{Timeout: timeout})
}

func FromBrowserWithOptions(opts BrowserOptions) (voyager.Creds, error) {
	creds, err := FromBrowserCandidatesWithOptions(opts)
	if err != nil {
		return voyager.Creds{}, err
	}
	return creds[0], nil
}

func FromBrowserCandidatesWithOptions(opts BrowserOptions) ([]voyager.Creds, error) {
	groups, err := collectByProfile(opts)
	if err != nil {
		return nil, err
	}
	if opts.Profile != "" {
		groups = filterProfiles(groups, opts.Profile)
	}
	profiles := pickProfiles(groups)
	if len(profiles) == 0 {
		if opts.Profile != "" {
			return nil, fmt.Errorf(
				"Chrome profile %q has no li_at — log into linkedin.com in that profile",
				opts.Profile)
		}
		return nil, fmt.Errorf(
			"no Chrome profile has a linkedin li_at — log into linkedin.com first")
	}
	creds := make([]voyager.Creds, 0, len(profiles))
	for _, pc := range profiles {
		creds = append(creds, credsFromProfile(pc))
	}
	return creds, nil
}

func credsFromProfile(pc *profileCookies) voyager.Creds {
	li := pc.byName["li_at"]
	js := pc.byName["JSESSIONID"]
	if js != "" && !strings.HasPrefix(js, `"`) {
		js = `"` + js + `"`
	}
	parts := make([]string, 0, len(pc.order))
	for _, name := range pc.order {
		if !persistBrowserCookie(name, pc.byName[name]) {
			continue
		}
		parts = append(parts, name+"="+pc.byName[name])
	}
	return voyager.Creds{
		LiAt:          li,
		JSESSIONID:    js,
		Cookie:        strings.Join(parts, "; "),
		UserAgent:     voyager.DefaultUserAgent,
		BrowserSource: pc.source,
	}
}

func persistBrowserCookie(name, value string) bool {
	return name != "" && value != "" && name != "JSESSIONID" && len(name) <= 128 && len(value) <= 512
}

func safePersistentCookie(name, value string) bool {
	if name == "" || value == "" || name == "JSESSIONID" || len(name) > 128 || len(value) > 512 {
		return false
	}
	if strings.Contains(value, `"`) && !(strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) {
		return false
	}
	return true
}

func collectByProfile(opts BrowserOptions) (map[string]*profileCookies, error) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	type result struct {
		groups map[string]*profileCookies
	}
	done := make(chan result, 1)

	go func() {
		if opts.CookieFile != "" {
			done <- result{groups: readProfileFile(ctx, opts.CookieFile)}
			return
		}
		done <- result{groups: readProfiles(ctx, timeout)}
	}()

	select {
	case res := <-done:
		return res.groups, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("browser cookie scan timed out after %s", timeout)
	}
}

func readProfileFile(ctx context.Context, file string) map[string]*profileCookies {
	cookies, err := chromecookies.ReadCookies(ctx, file, kooky.DomainHasSuffix("linkedin.com"))
	if err != nil {
		return map[string]*profileCookies{}
	}
	g := &profileCookies{source: file, byName: map[string]string{}}
	for _, c := range cookies {
		addCookie(g, c)
	}
	if len(g.order) == 0 {
		return map[string]*profileCookies{}
	}
	return map[string]*profileCookies{file: g}
}

func readProfiles(ctx context.Context, timeout time.Duration) map[string]*profileCookies {
	groups := map[string]*profileCookies{}
	for _, file := range chromeCookieFiles() {
		select {
		case <-ctx.Done():
			return groups
		default:
		}
		perFile := 5 * time.Second
		if timeout > 0 && timeout < perFile {
			perFile = timeout
		}
		fileCtx, cancel := context.WithTimeout(ctx, perFile)
		fileGroups := readProfileFile(fileCtx, file)
		cancel()
		for key, g := range fileGroups {
			groups[key] = g
		}
	}
	return groups
}

func chromeCookieFiles() []string {
	home, _ := os.UserHomeDir()
	var patterns []string
	switch runtime.GOOS {
	case "darwin":
		patterns = []string{
			filepath.Join(home, "Library/Application Support/Google/Chrome/*/Cookies"),
			filepath.Join(home, "Library/Application Support/Google/Chrome/*/Network/Cookies"),
		}
	case "windows":
		base := os.Getenv("LOCALAPPDATA")
		patterns = []string{
			filepath.Join(base, "Google/Chrome/User Data/*/Cookies"),
			filepath.Join(base, "Google/Chrome/User Data/*/Network/Cookies"),
		}
	default:
		patterns = []string{
			filepath.Join(home, ".config/google-chrome/*/Cookies"),
			filepath.Join(home, ".config/google-chrome/*/Network/Cookies"),
			filepath.Join(home, ".config/chromium/*/Cookies"),
			filepath.Join(home, ".config/chromium/*/Network/Cookies"),
		}
	}
	seen := map[string]bool{}
	var files []string
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		for _, file := range matches {
			if seen[file] {
				continue
			}
			seen[file] = true
			files = append(files, file)
		}
	}
	sort.Strings(files)
	return files
}

func addCookie(g *profileCookies, c *kooky.Cookie) {
	if c == nil || c.Value == "" {
		return
	}
	if !usableLinkedInDomain(c.Domain) {
		return
	}
	if _, dup := g.byName[c.Name]; dup {
		return
	}
	g.byName[c.Name] = c.Value
	g.order = append(g.order, c.Name)
}

func usableLinkedInDomain(domain string) bool {
	domain = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), ".")
	return domain == "linkedin.com" || domain == "www.linkedin.com"
}

func filterProfiles(groups map[string]*profileCookies, needle string) map[string]*profileCookies {
	needle = strings.ToLower(strings.TrimSpace(needle))
	out := map[string]*profileCookies{}
	for key, g := range groups {
		if strings.Contains(strings.ToLower(key), needle) {
			out[key] = g
		}
	}
	return out
}

func pickProfile(groups map[string]*profileCookies) *profileCookies {
	profiles := pickProfiles(groups)
	if len(profiles) == 0 {
		return nil
	}
	return profiles[0]
}

func pickProfiles(groups map[string]*profileCookies) []*profileCookies {
	candidates := make([]*profileCookies, 0, len(groups))
	for _, g := range groups {
		if g.byName["li_at"] != "" { // li_at is the only hard requirement
			candidates = append(candidates, g)
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		is, js := profileScore(candidates[i]), profileScore(candidates[j])
		if is != js {
			return is > js
		}
		return candidates[i].source < candidates[j].source
	})
	return candidates
}

func profileScore(g *profileCookies) int {
	score := len(g.order)
	// Prefer a profile that already has JSESSIONID (skips the bootstrap round-trip)
	// and routing/identity cookies (more complete, more browser-like session).
	for _, name := range []string{"JSESSIONID", "lidc", "bcookie", "bscookie", "liap"} {
		if g.byName[name] != "" {
			score += 10
		}
	}
	return score
}
