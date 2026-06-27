package cmd

import (
	"context"
	"errors"
	"time"

	"github.com/oyaah/li/internal/output"
	"github.com/oyaah/li/internal/voyager"
	"github.com/spf13/cobra"
)

var connectNote string

var connectCmd = &cobra.Command{
	Use:   "connect <publicId>",
	Short: "Send a connection invitation (auto-paced; warm-up before send)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		creds, c, l, j, err := writeDeps()
		if err != nil {
			return err
		}
		publicID, err := normalizeProfileID(args[0])
		if err != nil {
			return err
		}
		err = runConnect(c, l, j, out, publicID, connectNote, flagForce)
		if err == nil || creds.BrowserUserDataDir == "" || !writeBrowserFallbackOK(err) {
			return err
		}
		out.Human("native connect rejected; retrying through logged-in Chrome")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		status, berr := voyager.BrowserSendInvite(ctx, creds, publicID, connectNote)
		if berr != nil {
			return berr
		}
		if status == "sent" {
			if err := l.Record("connect"); err != nil {
				return err
			}
			out.Human("invite sent to %s", publicID)
			return nil
		}
		out.Human("invite already handled for %s: %s", publicID, status)
		return nil
	},
}

func writeBrowserFallbackOK(err error) bool {
	return errors.Is(err, output.ErrAuth) || errors.Is(err, output.ErrSchemaDrift) || errors.Is(err, output.ErrUsage)
}

func init() {
	connectCmd.Flags().StringVar(&connectNote, "note", "", "optional note to include with the invite")
	rootCmd.AddCommand(connectCmd)
}
