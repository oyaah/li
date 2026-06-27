package cmd

import (
	"github.com/oyaah/li/internal/output"
	"github.com/oyaah/li/internal/voyager"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check auth validity and probe each Voyager endpoint for drift",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := authedClient()
		if err != nil {
			return err
		}
		report := c.Health()
		if out.Format == output.Human {
			out.Human("schema version: %s", report.SchemaVersion)
		}
		if err := out.Data(report); err != nil {
			return err
		}
		// Non-nil → non-zero exit (77 auth / 69 drift) so doctor is scriptable.
		return voyager.DoctorError(report)
	},
}

func init() { rootCmd.AddCommand(doctorCmd) }
