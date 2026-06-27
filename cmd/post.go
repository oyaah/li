package cmd

import "github.com/spf13/cobra"

var postCmd = &cobra.Command{
	Use:   "post <text>",
	Short: "Post an update to your feed (auto-paced)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, l, j, err := writeDeps()
		if err != nil {
			return err
		}
		return runPost(c, l, j, out, args[0], flagForce)
	},
}

func init() { rootCmd.AddCommand(postCmd) }
