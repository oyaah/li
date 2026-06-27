package cmd

import (
	"os"

	"github.com/oyaah/li/internal/output"
	"github.com/spf13/cobra"
)

var (
	flagJSON  bool
	flagPlain bool
	flagForce bool

	// out is the resolved Printer, available to all subcommands after
	// PersistentPreRunE runs.
	out *output.Printer
)

var rootCmd = &cobra.Command{
	Use:           "li",
	Short:         "li — free, lightweight LinkedIn CLI over the Voyager API",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		f, err := output.ResolveFormat(flagJSON, flagPlain, output.IsTTY(os.Stdout))
		if err != nil {
			return err
		}
		out = output.New(f)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "machine-readable JSON output")
	rootCmd.PersistentFlags().BoolVar(&flagPlain, "plain", false, "TSV output, no header (for piping)")
	rootCmd.PersistentFlags().BoolVar(&flagForce, "force", false, "override rate soft-block on write actions")
}

// Execute runs the root command and maps any error to a sysexit code.
func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		// out may be nil if PersistentPreRunE itself failed; fall back to stderr.
		if out != nil {
			out.Human("error: %v", err)
		} else {
			os.Stderr.WriteString("error: " + err.Error() + "\n")
		}
		return output.ExitCode(err)
	}
	return output.OK
}
