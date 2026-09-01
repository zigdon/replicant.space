package cmd

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/lib/pq"
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/constants"
	"github.com/zigdon/rsp/models"
	"golang.org/x/sync/errgroup"
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
			UseHub:      getBool(cmd, "use_hub"),
			Recalculate: getBool(cmd, "recalculate"),
		}
		trip, err := common.PlotTrip(args[0], args[1], cfg)
		if err != nil {
			log("Error plotting trip: %v", err)
		}
		if trip == nil {
			return nil
		}
		// Get all relaying devices
		rows, err := db.Query(`
			SELECT type, location
			FROM json_devices
			WHERE status = 'relaying'`)
		if err != nil {
			return err
		}
		relays := make(map[string]string)
		for rows.Next() {
			var t, l string
			if err := rows.Scan(&t, &l); err != nil {
				return err
			}
			star, _, _ := strings.Cut(l, "-")
			if r, ok := relays[star]; ok {
				if constants.RelayDistance[r] > constants.RelayDistance[t] {
					continue
				}
			}
			relays[star] = t
		}
		if err := rows.Close(); err != nil {
			return err
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
			if d > 10 {
				dist = fmt.Sprintf("%.2f H", d)
			} else if d > 7.5 {
				dist = fmt.Sprintf("%.2f S", d)
			} else {
				dist = fmt.Sprintf("%.2f", d)
			}
			if r, ok := relays[l.To]; ok {
				l.To = fmt.Sprintf("%s (%s)", l.To, r)
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

var nearestHomeCmd = &cobra.Command{
	Use:               "home",
	Short:             "Find the nearest base to the specified location",
	ValidArgsFunction: completeStars,
	RunE:              nearestHome,
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

var plotIslandCmd = &cobra.Command{
	Use:   "island",
	Short: "Identify clusters of stars that are isolated",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("Island starting point is required")
		}
		hop := getFloat32(cmd, "max_hop")
		limit := getInt(cmd, "limit")
		island, err := common.RelayIsland(args[0], hop, limit)
		if err != nil {
			return err
		}
		log("Island identified:")
		for _, i := range island {
			log("  %s", i)
		}

		return nil
	},
}

var plotBridgeCmd = &cobra.Command{
	Use:   "bridge",
	Short: "Find the best system of an island to bridge to the main network",
	RunE:  plotBridge,
}

func init() {
	rootCmd.AddCommand(plotCmd)
	plotCmd.Flags().Float32P("max_hop", "m", 7.5, "Maximum allow hop, in ly")
	plotCmd.Flags().BoolP("use_station", "s", false, "Allow using deep space relay stations to bridge gaps")
	plotCmd.Flags().BoolP("use_hub", "H", false, "Allow using system hubs to bridge gaps")
	plotCmd.Flags().BoolP("recalculate", "c", false, "Ignore any cached routes")
	plotCmd.Flags().Bool("partial", true, "Allow extracting a partial route from a longer one")
	plotCmd.PersistentFlags().Bool("debug", false, "Output additional debugging data")

	plotCmd.AddCommand(nearestCmd)
	plotCmd.AddCommand(nearestHubCmd)
	plotCmd.AddCommand(nearestRelayCmd)
	plotCmd.AddCommand(nearestHomeCmd)
	plotCmd.AddCommand(plotDistanceCmd)

	plotCmd.AddCommand(neighboursCmd)
	neighboursCmd.Flags().Float32P("radius", "r", 7.5, "Radius for search")

	plotCmd.AddCommand(plotIslandCmd)
	plotIslandCmd.Flags().Float32P("max_hop", "r", 7.5, "Radius for inclusion in the island")
	plotIslandCmd.Flags().IntP("limit", "l", 100, "Max systems in the island")

	plotCmd.AddCommand(plotBridgeCmd)
	plotBridgeCmd.Flags().Float32P("max_hop", "r", 7.5, "Radius for inclusion in the island")
	plotBridgeCmd.Flags().IntP("limit", "l", 100, "Max systems in the island")
	plotBridgeCmd.Flags().Bool("only_landing", false, "Skip finding the path, only find the origin")
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

	star, dist, err := common.NearestHub(false, args[0])
	if err != nil {
		return err
	}
	ownedStar, ownedDist, err := common.NearestHub(true, args[0])
	if err != nil {
		return err
	}
	if star == ownedStar {
		log("Nearest hub: %s (%.2fly away)", star, dist)
	} else {
		log("Nearest hub: %s (%.2fly away); Nearest owned hub: %s (%.2fly away)",
			star, dist, ownedStar, ownedDist)
	}
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

func nearestHome(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Missing required args: plot home STAR")
	}

	star := common.ClosestHomes(models.LocationID(args[0]))[0]
	src, _ := models.NewStar(args[0])
	dst, _ := models.NewStar(star)
	log("Nearest home base: %s (%.2fly away)", star, src.Position.Distance(dst.Position))
	return nil
}

func plotBridge(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("A system on the island is required")
	}

	// Load the systems in the island
	stars, err := common.RelayIsland(args[0], getFloat32(cmd, "max_hop"), getInt(cmd, "limit"))
	if err != nil {
		return err
	}

	// Load the relay network
	network := make(map[string]bool)
	net, err := common.FullNetwork()
	if err != nil {
		return err
	}
	for _, c := range net {
		network[c] = true
	}

	type bridge struct {
		start    string
		end      string
		hops     []string
		stations int
		hubs     int
	}

	options := make(map[string]*bridge)
	cfg := &common.PlotCfg{
		Hop:         7.5,
		Recalculate: true,
		UseHub:      true,
		UseStation:  true,
	}

	slices.Sort(stars)
	log("Island stars: %s", strings.Join(stars, ", "))
	if !getBool(cmd, "only_landing") {
		var mu sync.Mutex
		var eg errgroup.Group
		eg.SetLimit(20)
		// Find the nearest relay for each star on the island
		for n, s := range stars {
			eg.Go(func() error {
				log("%d/%d: %s...", n, len(stars), s)
				b := &bridge{
					start: s,
					hops:  []string{s},
				}
				relay, err := common.NearestRelay(s)
				if err != nil {
					return fmt.Errorf("Error finding relay from %q: %v", s, err)
				}
				b.end = relay
				path, err := common.PlotTrip(s, relay, cfg)
				if err != nil {
					return fmt.Errorf("No path possible from %q to %q: %v", s, relay, err)
				}
				for _, p := range path.Legs {
					if slices.Contains(stars, p.To) {
						return nil
					}
					b.hops = append(b.hops, p.To)
					if p.FromPosition.Distance(p.ToPosition) > 10 {
						b.hubs++
					} else if p.FromPosition.Distance(p.ToPosition) > 7.5 {
						b.stations++
					}
					if network[p.To] {
						break
					}
				}
				mu.Lock()
				defer mu.Unlock()
				options[s] = b
				return nil
			})
		}
		eg.Wait()

		var data [][]any
		for k, v := range options {
			data = append(data, []any{
				k, v.end, v.stations, v.hubs, strings.Join(v.hops, "->"),
			})
		}
		printTable([]string{"Island", "Network", "# DSRS", "# Hubs", "Path"}, data)
	}

	// If there isn't a possible bridge, identify the star nearest to the island.
	if len(options) == 0 {
		row := db.QueryRow(`
		  WITH star_list AS (
			  SELECT designation, position
			  FROM stars
			  WHERE designation = ANY($1::TEXT[])
		  )
		  SELECT n.designation AS nearest_star,
				 l.designation AS closest_to_list_star,
				 n.dist AS distance
		  FROM star_list l
		  CROSS JOIN LATERAL (
			  SELECT s.designation, s.position <-> l.position AS dist
			  FROM stars s
			  WHERE s.designation NOT IN (SELECT designation FROM star_list)
			  ORDER BY s.position <-> l.position ASC
			  LIMIT 1
		  ) n
		  ORDER BY n.dist ASC
		  LIMIT 1`, pq.Array(stars))
		var offshore, island string
		var dist float32
		if err := row.Scan(&offshore, &island, &dist); err != nil {
			return fmt.Errorf("Can't find nearest off-shore star: %v", err)
		}
		log("No bridge options available. Nearest offshore star is %s, %.2f LY from %s",
			offshore, dist, island)
	}

	return nil
}
