package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/rest"
)

// scanCmd represents the scan command
var starsCmd = &cobra.Command{
	Use:   "stars",
	Short: "List the nearest stars",
	RunE: func(cmd *cobra.Command, args []string) error {
		rID, _ := cmd.Flags().GetString("code")
		if rID == "" {
			id, _ := cmd.Flags().GetInt("id")
			code, err := rest.ReplicantID(id)
			if err != nil {
				return fmt.Errorf("Replicant #%d not found: %v", id, err)
			}
			rID = code
		}
		page, _ := cmd.Flags().GetInt("page")
		cnt, _ := cmd.Flags().GetInt("count")
		resp, err := rest.ReplicantCensus(rID, cnt, page)
		if err != nil {
			return fmt.Errorf("Error running stellar census: %v", err)
		}
		if raw, _ := cmd.Flags().GetBool("raw"); raw {
			prettyPrint(resp)
		} else {
			printTable([]string{
				"Location", "Total Stars", "Page",
			}, [][]string{{
				resp.ReplicantPosition.String(), d(resp.TotalStars),
				fmt.Sprintf("%d/%d", page, resp.TotalPages),
			}})
			var stars [][]string
			for _, s := range resp.Stars {
				stars = append(stars, []string{
					s.Designation,
					s.EntryPoint,
					d(s.EstimatedPlanets),
					d(s.EstimatedTravelTime),
					s.SpectralType,
					b(s.Explored),
					b(s.HasLife),
				})
			}
			printTable([]string{
				"Designation", "Entry Point", "Est Planets", "ETA",
				"Spectral Type", "Explored", "Has Life",
			}, stars)
		}
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(starsCmd)
	starsCmd.Flags().IntP("page", "p", 0, "Census page to fetch")
	starsCmd.Flags().IntP("count", "n", 10, "Entries per page")
}
