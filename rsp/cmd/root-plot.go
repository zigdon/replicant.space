package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var plotCmd = &cobra.Command{
	Use:   "plot",
	Short: "Plan a multi-hop trip",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			return fmt.Errorf("Source and destination are required: plot <src> <dst>")
		}
		cfg := &common.PlotCfg{
			Debug:       getBool(cmd, "debug"),
			Hop:         getFloat32(cmd, "max_hop"),
			Recalculate: getBool(cmd, "recalculate"),
		}
		trip, err := common.PlotTrip(args[0], args[1], cfg)
		if err != nil {
			log("Error plotting trip: %v", err)
		}
		if trip == nil {
			return nil
		}
		data := [][]string{[]string{trip.Source, trip.Dest, "-", "-"},
			{"", "", "", ""}}
		for _, l := range trip.Legs {
			data = append(data, []string{
				l.From, l.To, f(l.DistFromSrc), f(l.DistToDest),
			})
		}
		printTable([]string{"Source", "Destination", "From Source", "To Destination"},
			data)
		return nil
	},
}

var nearestCmd = &cobra.Command{
	Use:   "nearest",
	Short: "Find the nearest star to an arbitrary position",
	RunE:  nearestStar,
}

var nearestHubCmd = &cobra.Command{
	Use:   "hub",
	Short: "Find the nearest star with our system hub to a specified destination",
	RunE:  nearestHub,
}

var neighboursCmd = &cobra.Command{
	Use:   "neighbours",
	Short: "List the nearest stars in a radius",
	RunE:  neighbourStars,
}

var sectorStarsCmd = &cobra.Command{
	Use:   "sector",
	Short: "List the stars in a specific vector of the galaxy",
	RunE:  sectorStars,
}

func init() {
	rootCmd.AddCommand(plotCmd)
	plotCmd.Flags().Float32P("max_hop", "m", 7.5, "Maximum allow hop, in ly")
	plotCmd.Flags().BoolP("recalculate", "c", false, "Ignore any cached routes")
	plotCmd.PersistentFlags().Bool("debug", false, "Output additional debugging data")

	plotCmd.AddCommand(nearestCmd)
	plotCmd.AddCommand(nearestHubCmd)

	plotCmd.AddCommand(neighboursCmd)
	neighboursCmd.Flags().Float32P("radius", "r", 7.5, "Radius for search")

	plotCmd.AddCommand(sectorStarsCmd)
	sectorStarsCmd.Flags().StringP("source", "s", "SOL", "Vector source, STAR or x,y,z")
	sectorStarsCmd.Flags().IntP("cone", "c", 1, "Radius of the cone, as a percentage of each vector element")
	sectorStarsCmd.Flags().IntP("margin", "m", 5, "Depth of the sector, as a percentage of the distance from origin")
}

func getPosFromString(dst string) (*models.Position, error) {
	if strings.Contains(dst, ",") || strings.Contains(dst, ".") {
		return models.ParsePosition(dst)
	} else {
		starDst, err := models.NewStar(dst)
		if err != nil {
			return nil, err
		}
		return starDst.Position, nil
	}
}

func sectorStars(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Missing required args: plot vector <star or location>")
	}

	cone := getInt(cmd, "cone")
	margin := getInt(cmd, "margin")

	dPos, err := getPosFromString(args[0])
	if err != nil {
		return err
	}

	log("Finding stars in the direction of %s within a %d%% cone, %d%% margin",
		dPos.String(), cone, margin,
	)

	res, err := db.GetSector(dPos.X, dPos.Y, dPos.Z, cone, margin)
	if err != nil {
		return err
	}
	var data [][]string
	for _, s := range res {
		st, err := models.NewStar(s)
		if err != nil {
			return err
		}
		data = append(data, []string{
			s, st.Position.String(), f(st.DistanceFromSol), f(dPos.Distance(st.Position)),
		})
	}

	printTable([]string{"Designation", "Position", "LY from origin", "LY from destination"}, data)

	return nil
}

func neighbourStars(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Missing required args: plot neighbours <star>")
	}
	r := getFloat32(cmd, "radius")
	src, err := models.NewStar(args[0])
	if err != nil {
		return err
	}
	rows, err := db.DB.Query(
		`SELECT designation, position_x, position_y, position_z,
		    sqrt(
				power(position_x-$1,2) +
				power(position_y-$2,2) +
				power(position_z-$3,2)) AS dist
		FROM stars
		WHERE dist <= $4
		ORDER BY dist
		`, src.Position.X, src.Position.Y, src.Position.Z, r)
	if err != nil {
		return err
	}

	var data [][]string
	var errs []error
	for rows.Next() {
		var n string
		var x, y, z, d float32
		errs = append(errs, rows.Scan(&n, &x, &y, &z, &d))
		data = append(data, []string{
			n, models.NewPosition(x, y, z).String(), f(d),
		})
	}
	printTable([]string{"Designation", "Position", "Distance"}, data)
	return rows.Err()
}

func nearestStar(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Missing required args: plot nearest x,y,z")
	}
	pos, err := models.ParsePosition(args[0])
	if err != nil {
		return err
	}
	nearest, dist, err := db.FindNearestStar(pos.X, pos.Y, pos.Z)
	if err != nil {
		return fmt.Errorf("Can't find nearest star: %v", err)
	}
	log("Nearest star: %s (%.2fly away)", nearest, dist)
	return nil
}

func nearestHub(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Missing required args: plot hub STAR")
	}
	// Update the db with our hubs
	hubs, err := rest.Devices(map[string]string{
		"device_type": "system_hub",
	})
	locs := make(map[string]string)
	for _, h := range hubs {
		if h.Status != "relaying" {
			log("Ignoring inactive hub %s at %s", h.Code.Alias(), h.Location)
			continue
		}
		star := h.Location.Star()
		locs[star] = h.Code.Alias()
		if _, err := db.DB.Exec(`UPDATE stars SET has_my_hub=true WHERE designation = $1`, star); err != nil {
			return fmt.Errorf("Can't update %s with hub: %v", star, err)
		}
	}

	s, err := models.NewStar(args[0])
	if err != nil {
		return err
	}
	nearest, dist, err := db.FindNearestHub(s.Position.X, s.Position.Y, s.Position.Z)
	if err != nil {
		return fmt.Errorf("Can't find nearest hub: %v", err)
	}
	log("Nearest hub: %s at %s (%.2fly away)", locs[nearest], nearest, dist)
	return nil
}
