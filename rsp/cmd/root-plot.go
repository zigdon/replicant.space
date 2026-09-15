package cmd

import (
	"cmp"
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

var networksCmd = &cobra.Command{
	Use:               "networks",
	Short:             "List the nearby FTL networks",
	ValidArgsFunction: completeStars,
	RunE:              neighbourNetworks,
}

var plotDistanceCmd = &cobra.Command{
	Use:               "distance",
	Short:             "Measure the distance between two points",
	ValidArgsFunction: completeStars,
	RunE:              plotDistance,
}

var plotRegionsCmd = &cobra.Command{
	Use:               "region",
	Aliases:           []string{"regions"},
	Short:             "Find the nearest regions to the specified stars",
	ValidArgsFunction: completeStars,
	RunE:              plotRegions,
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

var plotSpareHubCmd = &cobra.Command{
	Use:   "spares",
	Short: "Identify hubs that can be removed",
	RunE:  plotSpareHubs,
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
	plotCmd.AddCommand(networksCmd)
	plotCmd.AddCommand(plotDistanceCmd)
	plotCmd.AddCommand(plotRegionsCmd)
	plotCmd.AddCommand(plotSpareHubCmd)

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

	star, err := common.NearestRelay(args[0], nil)
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
	net, err := common.FullNetwork(nil)
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
				relay, err := common.NearestRelay(s, nil)
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

func plotRegions(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("At least one system is required")
	}
	var regs []string
	dists := make(map[string]map[string]float32)
	for _, dest := range args {
		rows, err := db.Query(`
			SELECT region, MIN(
			  position<->(
				SELECT position FROM stars WHERE designation = $1
			))
			FROM stars
			GROUP BY region
			ORDER BY min`, models.LocationID(dest).Star())
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var r string
			var d float32
			if err := rows.Scan(&r, &d); err != nil {
				return err
			}
			if r == "" {
				r = "none"
			}
			if !slices.Contains(regs, r) {
				regs = append(regs, r)
			}
			res, ok := dists[dest]
			if !ok {
				dists[dest] = make(map[string]float32)
				res = dists[dest]
			}
			res[r] = d
		}
	}

	slices.Sort(regs)
	var data [][]any
	for _, s := range args {
		var ds []any
		var min float32 = -1
		var col int
		for i, r := range regs {
			if min == -1 || dists[s][r] < min {
				min = dists[s][r]
				col = i
			}
			ds = append(ds, dists[s][r])
		}
		l := []any{s, regs[col]}
		l = append(l, ds...)
		data = append(data, l)
	}
	headers := []string{"Star", "Region"}
	headers = append(headers, regs...)
	printTable(headers, data)

	return nil
}

func plotSpareHubs(cmd *cobra.Command, args []string) error {
	net, err := common.FullNetwork(nil)
	if err != nil {
		return err
	}

	type rec struct {
		code        *models.CodeAlias
		location    models.LocationID
		inNet       bool
		spare       bool
		inHubRange  []string
		inDsrsRange []string
		inFrRange   []string
		missingFR   []string
		missingDSRS []string
		lost        []string
		src         models.LocationID
	}

	// Get all the relay devices
	var res []rec
	relays, err := db.QueryDevices(
		cache.QueryDevicesType("ftl_relay", "deep_space_relay_station", "system_hub"),
		cache.QueryDevicesStatus("relaying"),
	)
	if err != nil {
		return err
	}
	relayRange := make(map[string]float32)
	var hubs []*cache.QueryDevicesRes
	for _, r := range relays {
		switch r.Type {
		case "ftl_relay":
			relayRange[models.LocationID(r.Location).Star()] = 7.5
		case "deep_space_relay_station":
			relayRange[models.LocationID(r.Location).Star()] = 10
		case "system_hub":
			relayRange[models.LocationID(r.Location).Star()] = 15
			hubs = append(hubs, r)
		default:
			return fmt.Errorf("Unknown relay: %v", r)
		}
	}

	var eg errgroup.Group
	eg.SetLimit(10)
	var mu sync.Mutex
	for _, h := range hubs {
		if len(args) > 0 && !slices.ContainsFunc(args, func(a string) bool {
			return a == models.LocationID(h.Location).Star()
		}) {
			continue
		}
		eg.Go(func() error {
			dev, err := models.ParseOnly[models.Device](h.Data)
			if err != nil {
				return err
			}
			log("HUB %s @ %s", dev.Code, dev.Location)
			r := rec{
				code:     dev.Code,
				location: dev.Location,
				inNet:    slices.Contains(net, models.LocationID(h.Location).Star()),
			}
			star, err := models.NewStar(dev.Location.Star())
			if err != nil {
				return err
			}
			inRange, err := db.QueryStarsInRadius(star.Position.X, star.Position.Y, star.Position.Z, 7.5, 0)
			if err != nil {
				return err
			}
			for _, s := range inRange {
				r.inFrRange = append(r.inFrRange, s.Designation)
			}
			inRange, err = db.QueryStarsInRadius(star.Position.X, star.Position.Y, star.Position.Z, 10, 0)
			if err != nil {
				return err
			}
			for _, s := range inRange {
				if slices.Contains(r.inFrRange, s.Designation) {
					continue
				}
				r.inDsrsRange = append(r.inDsrsRange, s.Designation)
			}
			inRange, err = db.QueryStarsInRadius(star.Position.X, star.Position.Y, star.Position.Z, 15, 0)
			if err != nil {
				return err
			}
			for _, s := range inRange {
				if slices.Contains(r.inFrRange, s.Designation) {
					continue
				}
				if slices.Contains(r.inDsrsRange, s.Designation) {
					continue
				}
				r.inHubRange = append(r.inHubRange, s.Designation)
			}

			r.spare = true
			for _, t := range r.inHubRange {
				if _, ok := relayRange[t]; !ok {
					continue
				}
				home := common.ClosestHomes(models.LocationID(t))[0]
				r.src = models.LocationID(home)
				path, err := common.PlotTrip(home, t, &common.PlotCfg{
					Hop:        7.5,
					UseStation: true,
					Partial:    false,
					Banned:     []string{dev.Location.Star()},
				})
				if err != nil {
					log("Can't find alternate path to %s", t)
					r.spare = false
					r.lost = append(r.lost, t)
					continue
				}
				slices.Reverse(path.Legs)
				for _, l := range path.Legs {
					if _, ok := relayRange[l.To]; ok {
						break
					}
					if l.FromPosition.Distance(l.ToPosition) > 7.5 {
						if !slices.Contains(r.missingDSRS, l.To) {
							r.missingDSRS = append(r.missingDSRS, l.To)
						}
					} else {
						if !slices.Contains(r.missingFR, l.To) {
							r.missingFR = append(r.missingFR, l.To)
						}
					}
				}
			}

			slices.Sort(r.lost)
			slices.Sort(r.missingFR)
			slices.Sort(r.missingDSRS)
			slices.Sort(r.inFrRange)
			slices.Sort(r.inDsrsRange)
			slices.Sort(r.inHubRange)
			mu.Lock()
			defer mu.Unlock()
			res = append(res, r)
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	var final [][]any
	for _, r := range res {
		var data [][]any
		line := []any{
			r.code.Alias(), r.location.Star(), r.inNet, len(r.inFrRange), len(r.inDsrsRange), len(r.inHubRange), r.spare,
		}
		data = append(data, line)
		printTable([]string{"Alias", "Star", "In network", "FR range", "DSRS range", "Hub range", "Spare"}, data)
		data = data[:0]
		if len(r.lost) > 0 {
			data = append(data, []any{"Lost connection", wrap(strings.Join(r.lost, ", "), 70)})
		}
		if len(r.missingFR) > 0 {
			data = append(data, []any{"FTL Relay\n" + r.src.Star(), wrap(strings.Join(r.missingFR, ", "), 70)})
		}
		if len(r.missingDSRS) > 0 {
			data = append(data, []any{"DSRS\n" + r.src.Star(), wrap(strings.Join(r.missingDSRS, ", "), 70)})
		}
		line = append(line, len(r.lost), len(r.missingFR), len(r.missingDSRS))
		final = append(final, line)
		if len(data) > 0 {
			printTable([]string{"Missing relays", "Systems"}, data)
		}
	}
	printTable([]string{"Alias", "Star", "In network", "FR range", "DSRS range", "Hub range", "Spare", "Lost", "New FR", "New DSRS"}, final)

	return nil
}

func neighbourNetworks(cmd *cobra.Command, args []string) error {
	loc, err := models.NewStar(args[0])
	if err != nil {
		return err
	}
	nets, err := common.IdentifyNetworkBridge(loc.Designation)
	if err != nil {
		return err
	}
	var data [][]any
	for n, d := range nets {
		data = append(data, []any{n, d})
	}
	slices.SortFunc(data, func(a, b []any) int {
		return cmp.Compare(a[1].(float32), b[1].(float32))
	})
	printTable([]string{"Network", "Distance"}, data)
	return nil
}
