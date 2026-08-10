package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
)

var plotCmd = &cobra.Command{
	Use:               "plot",
	Short:             "Plan a multi-hop trip",
	ValidArgsFunction: completeStars,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			return fmt.Errorf("Source and destination are required: plot <src> <dst>")
		}
		cfg := &common.PlotCfg{
			Debug:       getBool(cmd, "debug"),
			Hop:         getFloat32(cmd, "max_hop"),
			UseStation:  getBool(cmd, "use_station"),
			Recalculate: getBool(cmd, "recalculate"),
		}
		trip, err := common.PlotTrip(args[0], args[1], cfg)
		if err != nil {
			log("Error plotting trip: %v", err)
		}
		if trip == nil {
			return nil
		}
		pos := func(s string) *models.Position {
			st, err := models.NewStar(s)
			if err != nil {
				return nil
			}
			return st.Position
		}
		data := [][]any{{trip.Source, trip.Dest, "-", "-", "-"},
			{"", "", "", "", ""}}
		for _, l := range trip.Legs {
			l.FromPosition = pos(l.From)
			l.ToPosition = pos(l.To)
			d := l.FromPosition.Distance(l.ToPosition)
			var dist string
			if d > cfg.Hop {
				dist = fmt.Sprintf("%.2f *", d)
			} else {
				dist = fmt.Sprintf("%.2f", d)
			}
			data = append(data, []any{
				l.From, l.To, l.DistFromSrc, dist, l.DistToDest})
		}
		printTable([]string{"Source", "Destination", "From Source", "From Previous", "To Destination"},
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
	Use:               "hub",
	Short:             "Find the nearest star with our system hub to a specified destination",
	ValidArgsFunction: completeStars,
	RunE:              nearestHub,
}

var nearestRelayCmd = &cobra.Command{
	Use:               "relay",
	Short:             "Find the nearest relay to the specified location",
	ValidArgsFunction: completeStars,
	RunE:              nearestRelay,
}

var neighboursCmd = &cobra.Command{
	Use:               "neighbours",
	Short:             "List the nearest stars in a radius",
	ValidArgsFunction: completeStars,
	RunE:              neighbourStars,
}

var plotDistanceCmd = &cobra.Command{
	Use:               "distance",
	Short:             "Measure the distance between two points",
	ValidArgsFunction: completeStars,
	RunE:              plotDistance,
}

func init() {
	rootCmd.AddCommand(plotCmd)
	plotCmd.Flags().Float32P("max_hop", "m", 7.5, "Maximum allow hop, in ly")
	plotCmd.Flags().BoolP("use_station", "s", false, "Allow using deep space relay stations to bridge gaps")
	plotCmd.Flags().BoolP("recalculate", "c", false, "Ignore any cached routes")
	plotCmd.Flags().Bool("partial", true, "Allow extracting a partial route from a longer one")
	plotCmd.PersistentFlags().Bool("debug", false, "Output additional debugging data")

	plotCmd.AddCommand(nearestCmd)
	plotCmd.AddCommand(nearestHubCmd)
	plotCmd.AddCommand(nearestRelayCmd)
	plotCmd.AddCommand(plotDistanceCmd)

	plotCmd.AddCommand(neighboursCmd)
	neighboursCmd.Flags().Float32P("radius", "r", 7.5, "Radius for search")
}

func plotDistance(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("src and dst are required")
	}
	src, dst := args[0], args[1]
	dist, err := common.Distance(src, dst)
	if err != nil {
		return err
	}
	log("Distance between %s and %s: %.2fly", src, dst, dist)
	return nil
}

func getPosFromString(dst string) (*models.Position, error) {
	if strings.Contains(dst, ",") || strings.Contains(dst, ":") {
		return models.ParsePosition(dst)
	} else {
		starDst, err := models.NewStar(dst)
		if err != nil {
			return nil, err
		}
		return starDst.Position, nil
	}
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
	rows, err := db.DB.Query(`
		SELECT designation, position, position <-> $1::cube as dist
		FROM stars
		WHERE position <-> $1::cube < $2
		ORDER BY dist
		`, src.Position.AsCube(), r)
	if err != nil {
		return err
	}

	var data [][]any
	var errs []error
	for rows.Next() {
		var n string
		var p cache.Position
		var d float32
		errs = append(errs, rows.Scan(&n, &p, &d))
		data = append(data, []any{n, models.ParseCube(p).String(), d})
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

	code, star, dist, err := common.NearestHub(args[0])
	if err != nil {
		return err
	}
	log("Nearest hub: %s at %s (%.2fly away)", code, star, dist)
	return nil
}

func nearestRelay(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Missing required args: plot relay STAR")
	}

	star, err := common.NearestRelay(args[0])
	if err != nil {
		return err
	}
	src, _ := models.NewStar(args[0])
	dst, _ := models.NewStar(star)
	log("Nearest system with relay: %s (%.2fly away)", star, src.Position.Distance(dst.Position))
	return nil
}
