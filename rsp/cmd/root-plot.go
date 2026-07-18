package cmd

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var plotCmd = &cobra.Command{
	Use:   "plot",
	Short: "Plan a multi-hop trip",
	RunE:  plotTrip,
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

var vectorStarsCmd = &cobra.Command{
	Use:   "sector",
	Short: "List the stars in a specific vector of the galaxy",
	RunE:  vectorStars,
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

	plotCmd.AddCommand(vectorStarsCmd)
	vectorStarsCmd.Flags().StringP("source", "s", "SOL", "Vector source, STAR or x,y,z")
	vectorStarsCmd.Flags().IntP("cone", "c", 1, "Radius of the cone, as a percentage of each vector element")
	vectorStarsCmd.Flags().IntP("margin", "m", 5, "Depth of the sector, as a percentage of the distance from origin")
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

func vectorStars(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("Missing required args: plot vector <star or location>")
	}

	cone, _ := cmd.Flags().GetInt("cone")
	margin, _ := cmd.Flags().GetInt("margin")

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
	r, _ := cmd.Flags().GetFloat32("radius")
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
		if _, err := db.DB.Exec(`UPDATE stars SET has_my_hub=1 WHERE designation = $1`, star); err != nil {
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

func plotTrip(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("Missing required args: plot <source> <dest>")
	}
	src := args[0]
	dst := args[1]
	hop, _ := cmd.Flags().GetFloat32("max_hop")

	// Pathfinding:
	// Keep a list of waypoints
	//   - previous hop to get there
	//   - distance travelled from origin to get there
	//   - remaining distance to destination (as-crow-spaceflies)
	// Loop over waypoints, sorted by lowest travelled + remaining distance
	// For each waypoint, get the possible next steps
	// Ignore repeats (unless this is a shorter way to get to them)
	// Repeat until destination is found

	starSrc, err := models.NewStar(src)
	if err != nil {
		return err
	}
	sPos := starSrc.Position

	var dPos *models.Position
	if strings.Contains(dst, ",") {
		pos, err := models.ParsePosition(dst)
		if err != nil {
			return err
		}
		log("Plotting to arbitrary position %s", pos)
		nearest, dist, err := db.FindNearestStar(pos.X, pos.Y, pos.Z)
		if err != nil {
			return fmt.Errorf("Can't find nearest star: %v", err)
		}
		log("Nearest star: %s (%.2fly away)", nearest, dist)
		nStar, err := models.NewStar(nearest)
		if err != nil {
			return err
		}
		dPos = nStar.Position
	} else {
		starDst, err := models.NewStar(dst)
		if err != nil {
			return err
		}
		dPos = starDst.Position
	}

	origDist := sPos.Distance(dPos)
	j := &models.Journey{
		Source: src,
		Dest:   dst,
		MaxHop: hop,
	}
	recalc, _ := cmd.Flags().GetBool("recalculate")
	if err := j.Get(); !recalc && err == nil {
		log("Loading cached route from %s:", j.Calculated.Format(time.Stamp))
		log("%s (%.2fly)", src, origDist)
		for _, jl := range j.Legs {
			log(" -> %s (%.2fly:%.2fly)", jl.To, jl.DistFromSrc, jl.DistToDest)
		}
		return nil
	}

	log("Total distance: %.2fly", origDist)
	waypoints := map[string]*models.JourneyLeg{
		src: {
			To:         src,
			ToPosition: sPos,
			DistToDest: origDist,
		},
	}

	queue := []string{src}
	debug := func(tmpl string, args ...any) {
		if d, _ := cmd.Flags().GetBool("debug"); !d {
			return
		}
		log(tmpl, args...)
	}
	var best *models.JourneyLeg
	ts := time.Now()
	var cnt int
	for {
		// Sort by distance travelled + left to go
		slices.SortFunc(queue, func(a, b string) int {
			return cmp.Compare(
				waypoints[a].DistFromSrc+waypoints[a].DistToDest,
				waypoints[b].DistFromSrc+waypoints[b].DistToDest)
		})
		debug("starting iteration over queue: %v", queue)
		var nextQueue []string
		for _, s := range queue {
			cnt++
			if time.Since(ts) > time.Second {
				log("... Examined %d stars, %d in the queue, current best %v", cnt, len(queue), best)
				ts = time.Now()
			}

			debug("=== %s", s)
			// Get possible next steps
			qStar, err := models.NewStar(s)
			if err != nil {
				return fmt.Errorf("Can't get star %q: %v", s, err)
			}
			stars, err := TripStepCandidate(s, qStar.Position, dPos, hop)
			if err != nil {
				return fmt.Errorf("No candidates found from %v to %v: %v", s, dst, err)
			}
			debug("%d candidates found", len(stars))
			for _, next := range stars {
				if best == nil || next.DistToDest < best.DistToDest {
					best = next
				}
				debug("  - %v", next)
				next.DistFromSrc += waypoints[s].DistFromSrc
				next.Step = waypoints[s].Step + 1
				debug("      total distance from src: %.2f", next.DistFromSrc)
				ex, ok := waypoints[next.To]
				if !ok {
					// New waypoint, add it to the queue and move on
					waypoints[next.To] = next
					nextQueue = append(nextQueue, next.To)
					debug("      New waypoint: %s -> %s (behind: %.2f, ahead: %.2f)",
						next.From, next.To, next.DistFromSrc, next.DistToDest)
					continue
				}
				// Existing waypoint, if it's a shorter path to get there, update it.
				if ex.DistFromSrc > next.DistFromSrc {
					debug("      Shorter path to %q, from %q (%.2f) rather than %q (%.2f)",
						next.To, next.From, next.DistFromSrc, ex.From, ex.DistFromSrc)
					ex.DistFromSrc = next.DistFromSrc
					ex.From = next.From
					ex.FromPosition = next.FromPosition
					ex.Step = next.Step
					waypoints[next.To] = ex
					continue
				}
				debug("      Discarding longer leg")
			}
		}

		// Find the current best route
		cur := src
		closest := waypoints[src].DistToDest
		for k, v := range waypoints {
			if v.DistToDest < closest {
				cur = k
				closest = v.DistToDest
			}
		}

		if waypoints[cur].To == dst {
			// Print it
			var lines []string
			for {
				j.Legs = append(j.Legs, waypoints[cur])
				lines = append(lines, fmt.Sprintf(
					" -> %s (%.2fly:%.2fly)", cur, waypoints[cur].DistFromSrc, waypoints[cur].DistToDest,
				))
				if cur == src {
					break
				}
				cur = waypoints[cur].From
			}
			slices.Reverse(lines)
			log("%s (%.2fly)", src, origDist)
			for _, l := range lines {
				log(l)
			}
			return j.Cache()
		}

		if len(nextQueue) == 0 {
			break
		}
		queue = nextQueue
	}
	log("Failed to find route, closest is %v", best)

	return nil
}

func TripStepCandidate(start string, src, dst *models.Position, radius float32) ([]*models.JourneyLeg, error) {
	rows, err := db.DB.Query(`
		SELECT designation, position_x, position_y, position_z, from_src, from_dst
		FROM (
			SELECT designation, position_x, position_y, position_z,
				sqrt(
					power(position_x - $1, 2) + 
					power(position_y - $2, 2) + 
					power(position_z - $3, 2)
				) AS from_src,
				sqrt(
					power(position_x - $4, 2) +
					power(position_y - $5, 2) + 
					power(position_z - $6, 2)
				) AS from_dst
			FROM stars
		) sub
		WHERE from_src <= $7 AND from_src > 0.001;`,
		src.X, src.Y, src.Z,
		dst.X, dst.Y, dst.Z,
		radius,
	)
	if err != nil {
		return nil, err
	}

	var res []*models.JourneyLeg
	var errs []error
	for rows.Next() {
		var desg string
		var x, y, z, fSrc, fDst float32
		errs = append(errs, rows.Scan(&desg, &x, &y, &z, &fSrc, &fDst))
		res = append(res, &models.JourneyLeg{
			From: start,
			FromPosition: models.NewPosition(
				src.X,
				src.Y,
				src.Z),
			To:          desg,
			ToPosition:  models.NewPosition(x, y, z),
			DistFromSrc: fSrc,
			DistToDest:  fDst,
		},
		)
	}
	errs = append(errs, rows.Err())

	return res, errors.Join(errs...)
}
