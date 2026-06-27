package cmd

import (
	"github.com/oyaah/li/internal/output"
	"github.com/oyaah/li/internal/voyager"
	"github.com/spf13/cobra"
)

var inboxCmd = &cobra.Command{
	Use:   "inbox",
	Short: "List recent conversations",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := authedReadClient()
		if err != nil {
			return err
		}
		me, err := c.GetRaw(voyager.Me(), nil)
		if err != nil {
			return err
		}
		mailbox := voyager.MailboxURN(me)
		if mailbox == "" {
			return output.ErrSchemaDrift
		}
		b, err := c.GetRaw(voyager.Conversations(mailbox), nil)
		if err != nil {
			return err
		}
		in, err := voyager.ParseInbox(b)
		if err != nil {
			return err
		}
		return out.Data(in)
	},
}

func init() { rootCmd.AddCommand(inboxCmd) }
