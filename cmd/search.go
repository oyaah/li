package cmd

import (
	"strings"

	"github.com/oyaah/li/internal/voyager"
	"github.com/spf13/cobra"
)

var (
	searchTitle   string
	searchCompany string
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search people by keyword",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := authedClient()
		if err != nil {
			return err
		}
		path, params := voyager.PeopleSearch(strings.Join(args, " "), searchTitle, searchCompany)
		b, err := c.GetRaw(path, params)
		if err != nil {
			return err
		}
		people, err := voyager.ParsePeople(b)
		if err != nil {
			return err
		}
		return out.Data(people)
	},
}

func init() {
	searchCmd.Flags().StringVar(&searchTitle, "title", "", "filter by job title")
	searchCmd.Flags().StringVar(&searchCompany, "company", "", "filter by company")
	rootCmd.AddCommand(searchCmd)
}
