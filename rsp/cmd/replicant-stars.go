package cmd

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

// scanCmd represents the scan command
var starsCmd = &cobra.Command{
	Use:   "stars",
	Short: "List the nearest stars",
	RunE: func(cmd *cobra.Command, args []string) error {
		rID, err := getRID(cmd)
		if err != nil {
			return fmt.Errorf("Replicant not found: %v", err)
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
			var target *models.Position
			var err error
			headers := []string{"Location", "Total Stars", "Page"}
			data := [][]string{{
				resp.ReplicantPosition.String(), d(resp.TotalStars),
				fmt.Sprintf("%d/%d", page, resp.TotalPages),
			}}
			dest, _ := cmd.Flags().GetString("destination")
			if dest != "" {
				target, err = models.ParsePosition(dest)
				if err != nil {
					return err
				}
				headers = append(headers, "Destination")
				data[0] = append(data[0], target.String())
			}
			printTable(headers, data)
			var stars [][]string
			dist := make(map[models.LocationID]float32)
			for _, s := range resp.Stars {
				if f, _ := cmd.Flags().GetString("filter"); f != "" {
					if !strings.Contains(strings.ToLower(string(s.Designation)), strings.ToLower(f)) {
						continue
					}
				}

				// See if we have actual scans in the cache.
				var hasCache string
				row := db.DB.QueryRow(`SELECT COUNT(designation) FROM planets WHERE star = ?`, s.Designation)
				if row.Err() == nil {
					var planets int
					row.Scan(&planets)
					if planets > 0 {
						hasCache = "*"
						if planets != s.EstimatedPlanets {
							hasCache = fmt.Sprintf("* (%d)", s.EstimatedPlanets)
							s.EstimatedPlanets = planets
						}
					}
				}

				data := []string{
					string(s.Designation),
					string(s.EntryPoint),
					d(s.EstimatedPlanets) + hasCache,
					f(s.DistanceFromReplicant),
					s.EstimatedTravelTime.Duration().String(),
					s.SpectralType,
					b(s.Explored),
					b(s.HasLife),
					s.Position.String(),
				}
				if dest != "" {
					dist[s.Designation] = s.Position.Distance(target)
					data = append(data, f(dist[s.Designation]))
				}
				if err := db.Update(cache.StarsTable, map[string]any{
					"designation": s.Designation,
					"entry_point": s.EntryPoint,
					"est_planets": s.EstimatedPlanets,
					"explored":    s.Explored,
					"has_life":    s.HasLife,
					"name":        s.Name,
					"position_x":  s.Position.X,
					"position_y":  s.Position.Y,
					"position_z":  s.Position.Z,
					"has_hub":     false,
				}); err != nil {
					log("Error updating cache for %q: %v", s.Designation, err)
				}
				stars = append(stars, data)
			}
			headers = []string{
				"Designation", "Entry Point", "Est Planets", "Distance",
				"ETA", "Spectral Type", "Explored", "Has Life", "Location",
			}
			if dest != "" {
				headers = append(headers, "To destination")
				slices.SortFunc(stars, func(a, b []string) int {
					return cmp.Compare(dist[models.LocationID(a[0])], dist[models.LocationID(b[0])])
				})
			}
			printTable(headers, stars)
		}
		return nil
	},
}

func init() {
	replicantCmd.AddCommand(starsCmd)
	starsCmd.Flags().IntP("page", "p", 0, "Census page to fetch")
	starsCmd.Flags().IntP("count", "n", 10, "Entries per page")
	starsCmd.Flags().StringP("filter", "f", "", "Filter star names")
	starsCmd.Flags().StringP("destination", "d", "", "Show distance from specified x,y,z coordinate")
}
