package cmd

import "github.com/spf13/cobra"

var connectNote string

var connectCmd = &cobra.Command{
	Use:   "connect <publicId>",
	Short: "Send a connection invitation (auto-paced; warm-up before send)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, l, j, err := writeDeps()
		if err != nil {
			return err
		}
		return runConnect(c, l, j, out, args[0], connectNote, flagForce)
	},
}

func init() {
	connectCmd.Flags().StringVar(&connectNote, "note", "", "optional note to include with the invite")
	rootCmd.AddCommand(connectCmd)
}
