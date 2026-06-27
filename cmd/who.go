package cmd

import (
	"github.com/oyaah/li/internal/voyager"
	"github.com/spf13/cobra"
)

var whoCmd = &cobra.Command{
	Use:   "who <publicId>",
	Short: "Show a profile: name, headline, current role",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := authedClient()
		if err != nil {
			return err
		}
		b, err := c.GetRaw(voyager.ProfileView(args[0]), nil)
		if err != nil {
			return err
		}
		p, err := voyager.ParseProfile(b)
		if err != nil {
			return err
		}
		return out.Data(p)
	},
}

func init() { rootCmd.AddCommand(whoCmd) }
