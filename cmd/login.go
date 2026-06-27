package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/oyaah/li/internal/auth"
	cdpbrowser "github.com/oyaah/li/internal/browser"
	"github.com/oyaah/li/internal/output"
	"github.com/oyaah/li/internal/voyager"
	"github.com/spf13/cobra"
)

var (
	loginLiAt              string
	loginJSess             string
	loginUA                string
	loginCookie            string
	loginManual            bool
	loginBrowser           bool
	loginSystemBrowser     bool
	loginRealChrome        bool
	loginDryRun            bool
	loginBrowserTimeout    time.Duration
	loginBrowserProfile    string
	loginBrowserCookieFile string
	loginChromePath        string
	loginBrowserUserDir    string
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Open Chrome, log into LinkedIn, and store the session in the OS keyring",
	Long: "Open a controlled Chrome window, let LinkedIn handle normal login, then store\n" +
		"the validated LinkedIn session in the OS keyring. Google SSO and checkpoints\n" +
		"work only when LinkedIn finishes login and sets LinkedIn cookies; li never stores\n" +
		"Google tokens. If Google blocks controlled Chrome, use --system-browser or close\n" +
		"Chrome and use --real-chrome --browser-profile <profile-dir>.\n\n" +
		"Advanced/debug paths can import local browser cookie DBs or accept manual cookies.\n\n" +
		"WARNING: using the internal Voyager API violates LinkedIn's ToS and can get\n" +
		"your account restricted. Use an account you can afford to lose.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if loginDryRun && !loginBrowser {
			return fmt.Errorf("--dry-run is only supported with --from-browser: %w", output.ErrUsage)
		}
		var creds voyager.Creds
		var err error
		if loginBrowser {
			opts := auth.BrowserOptions{
				Timeout:    loginBrowserTimeout,
				Profile:    loginBrowserProfile,
				CookieFile: loginBrowserCookieFile,
			}
			if !loginDryRun {
				return runBrowserLogin(opts)
			}
			creds, err = auth.FromBrowserWithOptions(opts) // reuse the existing browser session (low-detection path)
		} else if loginSystemBrowser {
			return runSystemBrowserLogin(loginBrowserTimeout)
		} else if loginRealChrome {
			return runRealChromeLogin(loginBrowserTimeout, loginBrowserProfile)
		} else if wantsManualLogin() {
			creds, err = gatherCreds()
		} else {
			return runControlledBrowserLogin(auth.BrowserLoginOptions{
				Timeout:     loginBrowserTimeout,
				ChromePath:  loginChromePath,
				UserDataDir: loginBrowserUserDir,
			})
		}
		if err != nil {
			return err
		}
		if loginDryRun {
			// Report cookie presence only — no LinkedIn call, nothing stored.
			if creds.BrowserSource != "" {
				out.Human("profile: %s", creds.BrowserSource)
			}
			out.Human("li_at: %s", mask(creds.LiAt))
			out.Human("JSESSIONID: %s", mask(creds.JSESSIONID))
			out.Human("dry-run: cookies readable, no network call made, nothing stored")
			return nil
		}
		return runLogin(creds, voyager.New(creds))
	},
}

// mask shows only that a value is present and its length, never the secret.
func mask(s string) string {
	if s == "" {
		return "(missing)"
	}
	return fmt.Sprintf("found (%d chars)", len(s))
}

func init() {
	loginCmd.Flags().BoolVar(&loginManual, "manual", false, "advanced: paste LinkedIn cookies instead of opening Chrome")
	loginCmd.Flags().BoolVar(&loginBrowser, "from-browser", false, "debug: import cookies from local Chrome cookie DB files")
	loginCmd.Flags().BoolVar(&loginSystemBrowser, "system-browser", false, "fallback: log in with normal Chrome, then import the fresh LinkedIn session")
	loginCmd.Flags().BoolVar(&loginRealChrome, "real-chrome", false, "fallback: clone your real Chrome profile into li's controlled Chrome profile")
	loginCmd.Flags().BoolVar(&loginDryRun, "dry-run", false, "report whether cookies are readable without contacting LinkedIn or storing anything")
	loginCmd.Flags().DurationVar(&loginBrowserTimeout, "browser-timeout", 5*time.Minute, "max time to wait for browser login or cookie scan")
	loginCmd.Flags().StringVar(&loginBrowserProfile, "browser-profile", "", "debug: Chrome profile name substring to import from")
	loginCmd.Flags().StringVar(&loginBrowserCookieFile, "browser-cookie-file", "", "debug: Chrome Cookies SQLite file to import from")
	loginCmd.Flags().StringVar(&loginChromePath, "chrome", "", "Chrome executable path (defaults to installed Chrome or LI_CHROME)")
	loginCmd.Flags().StringVar(&loginBrowserUserDir, "browser-user-data-dir", "", "controlled Chrome profile directory")
	loginCmd.Flags().StringVar(&loginLiAt, "li-at", "", "advanced: li_at cookie value")
	loginCmd.Flags().StringVar(&loginJSess, "jsessionid", "", "advanced: JSESSIONID cookie value (optional; with quotes if supplied)")
	loginCmd.Flags().StringVar(&loginCookie, "cookie", "", "advanced: full linkedin.com Cookie header")
	loginCmd.Flags().StringVar(&loginUA, "user-agent", "", "advanced: browser User-Agent to clone")
	rootCmd.AddCommand(loginCmd)
}

func wantsManualLogin() bool {
	return loginManual || loginLiAt != "" || loginJSess != "" || loginCookie != "" || loginUA != ""
}

// runLogin validates the session against /me, then stores it. Bad cookies are
// never persisted.
func runLogin(creds voyager.Creds, client *voyager.Client) error {
	name, err := client.MeName()
	if err != nil {
		return err // ErrAuth from a 401/403 maps to exit 77
	}
	if err := auth.Save(creds); err != nil {
		return err
	}
	if name != "" {
		out.Human("logged in as %s", name)
	} else {
		out.Human("credentials valid and stored")
	}
	return nil
}

func runBrowserLogin(opts auth.BrowserOptions) error {
	candidates, err := auth.FromBrowserCandidatesWithOptions(opts)
	if err != nil {
		return err
	}
	var lastErr error
	for _, creds := range candidates {
		if creds.BrowserSource != "" {
			out.Human("trying %s", creds.BrowserSource)
		}
		if err := runLogin(creds, voyager.New(creds)); err != nil {
			lastErr = err
			if creds.BrowserSource != "" {
				out.Human("skipped %s: %v", creds.BrowserSource, err)
			}
			continue
		}
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("%w: no browser profile validated", output.ErrAuth)
}

func runControlledBrowserLogin(opts auth.BrowserLoginOptions) error {
	out.Human("opening LinkedIn. Log in once, then return here. Continue with Google is fine if Google accepts this browser.")
	res, err := auth.LoginWithBrowser(context.Background(), opts)
	if err != nil {
		if errors.Is(err, output.ErrAuth) {
			out.Human("controlled Chrome did not finish login; opening normal Chrome as a Google SSO fallback")
			return runSystemBrowserLogin(opts.Timeout)
		}
		return err
	}
	if err := auth.Save(res.Creds); err != nil {
		return err
	}
	if res.Name != "" {
		out.Human("logged in as %s", res.Name)
	} else {
		out.Human("browser session valid and stored")
	}
	return nil
}

func runSystemBrowserLogin(timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	if err := openSystemLinkedInLogin(); err != nil {
		return err
	}
	out.Human("normal Chrome opened. Complete LinkedIn login there; li will import and validate the fresh session.")
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		candidates, err := auth.FromBrowserCandidatesWithOptions(auth.BrowserOptions{Timeout: 20 * time.Second})
		if err != nil {
			lastErr = err
			time.Sleep(2 * time.Second)
			continue
		}
		for _, creds := range candidates {
			if creds.BrowserSource != "" {
				out.Human("trying %s", creds.BrowserSource)
			}
			if err := runLogin(creds, voyager.New(creds)); err != nil {
				lastErr = err
				continue
			}
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("%w: no normal Chrome LinkedIn session validated", output.ErrAuth)
}

func runRealChromeLogin(timeout time.Duration, profile string) error {
	if profile == "" {
		profile = "Default"
	}
	cloneDir, err := cloneChromeProfile(profile)
	if err != nil {
		return err
	}
	out.Human("cloned Chrome profile %q into li's controlled profile; opening it with DevTools", profile)
	res, err := auth.LoginWithBrowser(context.Background(), auth.BrowserLoginOptions{
		Timeout:          timeout,
		UserDataDir:      cloneDir,
		ProfileDirectory: profile,
		StartURL:         "https://www.linkedin.com/feed/",
	})
	if err != nil {
		return err
	}
	if err := auth.Save(res.Creds); err != nil {
		return err
	}
	if res.Name != "" {
		out.Human("logged in as %s", res.Name)
	} else {
		out.Human("real Chrome session valid and stored")
	}
	return nil
}

func cloneChromeProfile(profile string) (string, error) {
	srcRoot := cdpbrowser.DefaultChromeUserDataDir()
	srcProfile := filepath.Join(srcRoot, profile)
	if st, err := os.Stat(srcProfile); err != nil || !st.IsDir() {
		return "", fmt.Errorf("Chrome profile %q not found under %s", profile, srcRoot)
	}
	if err := clearStaleChromeSingletons(srcRoot); err != nil {
		return "", fmt.Errorf("quit Chrome completely before cloning profile %q: %w", profile, output.ErrUsage)
	}
	base := filepath.Join(cdpbrowser.DefaultUserDataDir(), "clones")
	dstRoot := filepath.Join(base, safeProfileDir(profile))
	if err := os.RemoveAll(dstRoot); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dstRoot, 0o700); err != nil {
		return "", err
	}
	if err := copyFile(filepath.Join(srcRoot, "Local State"), filepath.Join(dstRoot, "Local State")); err != nil {
		return "", err
	}
	if err := copyDir(srcProfile, filepath.Join(dstRoot, profile)); err != nil {
		return "", err
	}
	return dstRoot, nil
}

func clearStaleChromeSingletons(root string) error {
	lock := filepath.Join(root, "SingletonLock")
	if target, err := os.Readlink(lock); err == nil {
		pid := target[strings.LastIndex(target, "-")+1:]
		if pid != "" && exec.Command("ps", "-p", pid).Run() == nil {
			return fmt.Errorf("Chrome is still running as pid %s", pid)
		}
	}
	for _, name := range []string{"SingletonLock", "SingletonSocket", "SingletonCookie"} {
		if err := os.Remove(filepath.Join(root, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func safeProfileDir(profile string) string {
	return strings.NewReplacer("/", "_", "\\", "_", " ", "-").Replace(profile)
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o700)
		}
		if shouldSkipChromeClonePath(rel, d) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if info.Mode()&os.ModeType != 0 {
			return nil
		}
		return copyFile(path, target)
	})
}

func shouldSkipChromeClonePath(rel string, d fs.DirEntry) bool {
	parts := strings.Split(rel, string(os.PathSeparator))
	for _, part := range parts {
		switch part {
		case "Cache", "Code Cache", "GPUCache", "GrShaderCache", "GraphiteDawnCache", "DawnGraphiteCache",
			"ShaderCache", "DawnWebGPUCache", "Media Cache", "optimization_guide_model_store":
			return true
		}
	}
	name := d.Name()
	return strings.HasSuffix(name, "-journal") || strings.HasSuffix(name, ".tmp")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func openSystemLinkedInLogin() error {
	const loginURL = "https://www.linkedin.com/login/?session_redirect=https%3A%2F%2Fwww.linkedin.com%2Ffeed%2F"
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", "-a", "Google Chrome", loginURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", loginURL)
	default:
		cmd = exec.Command("xdg-open", loginURL)
	}
	return cmd.Start()
}

// gatherCreds reads from flags, falling back to interactive prompts on stdin.
func gatherCreds() (voyager.Creds, error) {
	li, js, ua, cookie := loginLiAt, loginJSess, loginUA, loginCookie
	r := bufio.NewReader(os.Stdin)
	if li == "" {
		li = prompt(r, "Paste li_at: ")
	}
	if li == "" {
		return voyager.Creds{}, fmt.Errorf("%w: li_at is required", output.ErrUsage)
	}
	return voyager.Creds{LiAt: li, JSESSIONID: js, UserAgent: ua, Cookie: cookie}, nil
}

func prompt(r *bufio.Reader, label string) string {
	fmt.Fprint(os.Stderr, label)
	line, _ := r.ReadString('\n')
	return strings.TrimSpace(line)
}
