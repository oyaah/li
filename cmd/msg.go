package cmd

import "github.com/spf13/cobra"

var msgCmd = &cobra.Command{
	Use:   "msg <publicId> <text>",
	Short: "Send a message to someone (auto-paced)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, l, j, err := writeDeps()
		if err != nil {
			return err
		}
		return runMsg(c, l, j, out, args[0], args[1], flagForce)
	},
}

func init() { rootCmd.AddCommand(msgCmd) }
