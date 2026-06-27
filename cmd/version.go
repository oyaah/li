package cmd

import (
	"github.com/oyaah/li/internal/voyager"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type versionInfo struct {
	Version       string `json:"version"`
	Commit        string `json:"commit"`
	Date          string `json:"date"`
	SchemaVersion string `json:"schema_version"`
}

func (v versionInfo) Columns() []string {
	return []string{"version", "commit", "date", "schema_version"}
}

func (v versionInfo) Rows() [][]string {
	return [][]string{{v.Version, v.Commit, v.Date, v.SchemaVersion}}
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version and schema information",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return out.Data(currentVersionInfo())
	},
}

func currentVersionInfo() versionInfo {
	return versionInfo{
		Version:       version,
		Commit:        commit,
		Date:          date,
		SchemaVersion: voyager.SchemaVersion,
	}
}

func init() { rootCmd.AddCommand(versionCmd) }
