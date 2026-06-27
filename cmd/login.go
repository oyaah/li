package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/oyaah/li/internal/auth"
	"github.com/oyaah/li/internal/output"
	"github.com/oyaah/li/internal/voyager"
	"github.com/spf13/cobra"
)

var (
	loginLiAt    string
	loginJSess   string
	loginUA      string
	loginBrowser bool
	loginDryRun  bool
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Store LinkedIn session cookies (li_at + JSESSIONID) in the OS keyring",
	Long: "Capture your LinkedIn session cookies and store them in the OS keyring.\n" +
		"Get them from your browser devtools (Application > Cookies > linkedin.com):\n" +
		"  li_at        — the session token\n" +
		"  JSESSIONID   — include the surrounding quotes, e.g. \"ajax:1234\"\n\n" +
		"WARNING: using the internal Voyager API violates LinkedIn's ToS and can get\n" +
		"your account restricted. Use an account you can afford to lose.",
	RunE: func(cmd *cobra.Command, args []string) error {
		var creds voyager.Creds
		var err error
		if loginBrowser {
			creds, err = auth.FromBrowser() // reuse the existing browser session (low-detection path)
		} else {
			creds, err = gatherCreds()
		}
		if err != nil {
			return err
		}
		if loginDryRun {
			// Report cookie presence only — no LinkedIn call, nothing stored.
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
	loginCmd.Flags().BoolVar(&loginBrowser, "from-browser", false, "import cookies from a logged-in local browser (recommended: reuses your existing session)")
	loginCmd.Flags().BoolVar(&loginDryRun, "dry-run", false, "report whether cookies are readable without contacting LinkedIn or storing anything")
	loginCmd.Flags().StringVar(&loginLiAt, "li-at", "", "li_at cookie value")
	loginCmd.Flags().StringVar(&loginJSess, "jsessionid", "", "JSESSIONID cookie value (with quotes)")
	loginCmd.Flags().StringVar(&loginUA, "user-agent", "", "browser User-Agent to clone (optional)")
	rootCmd.AddCommand(loginCmd)
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

// gatherCreds reads from flags, falling back to interactive prompts on stdin.
func gatherCreds() (voyager.Creds, error) {
	li, js, ua := loginLiAt, loginJSess, loginUA
	r := bufio.NewReader(os.Stdin)
	if li == "" {
		li = prompt(r, "Paste li_at: ")
	}
	if js == "" {
		js = prompt(r, "Paste JSESSIONID (with quotes): ")
	}
	if li == "" || js == "" {
		return voyager.Creds{}, fmt.Errorf("%w: li_at and JSESSIONID are required", output.ErrUsage)
	}
	return voyager.Creds{LiAt: li, JSESSIONID: js, UserAgent: ua}, nil
}

func prompt(r *bufio.Reader, label string) string {
	fmt.Fprint(os.Stderr, label)
	line, _ := r.ReadString('\n')
	return strings.TrimSpace(line)
}
