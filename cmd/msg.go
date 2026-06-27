package cmd

import (
	"context"
	"time"

	"github.com/oyaah/li/internal/voyager"
	"github.com/spf13/cobra"
)

var msgCmd = &cobra.Command{
	Use:   "msg <publicId> <text>",
	Short: "Send a message to someone (auto-paced)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		creds, c, l, j, err := writeDeps()
		if err != nil {
			return err
		}
		publicID, err := normalizeProfileID(args[0])
		if err != nil {
			return err
		}
		err = runMsg(c, l, j, out, publicID, args[1], flagForce)
		if err == nil || creds.BrowserUserDataDir == "" || !writeBrowserFallbackOK(err) {
			return err
		}
		out.Human("native message rejected; retrying through logged-in Chrome")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		status, berr := voyager.BrowserSendMessage(ctx, creds, publicID, args[1])
		if berr != nil {
			return berr
		}
		if status == "sent" {
			if err := l.Record("msg"); err != nil {
				return err
			}
			out.Human("message sent to %s", publicID)
		}
		return nil
	},
}

func init() { rootCmd.AddCommand(msgCmd) }
