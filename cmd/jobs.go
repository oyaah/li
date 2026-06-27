package cmd

import (
	"strings"

	"github.com/oyaah/li/internal/voyager"
	"github.com/spf13/cobra"
)

var jobsLocation string

var jobsCmd = &cobra.Command{
	Use:   "jobs <query>",
	Short: "Search jobs by keyword",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := authedClient()
		if err != nil {
			return err
		}
		path, params := voyager.JobSearch(strings.Join(args, " "), jobsLocation)
		b, err := c.GetRaw(path, params)
		if err != nil {
			return err
		}
		jobs, err := voyager.ParseJobs(b)
		if err != nil {
			return err
		}
		return out.Data(jobs)
	},
}

func init() {
	jobsCmd.Flags().StringVar(&jobsLocation, "location", "", "filter by location")
	rootCmd.AddCommand(jobsCmd)
}
