package cmd

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/cache"
	"github.com/zigdon/rsp/models"
)

var plotCmd = &cobra.Command{
	Use:   "plot",
	Short: "Plan a multi-hop trip",
	RunE: plotTrip,
}

func init() {
	rootCmd.AddCommand(plotCmd)
	plotCmd.Flags().Float32P("max_hop", "m", 7.5, "Maximum allow hop, in ly")
	plotCmd.MarkFlagRequired("source")
	plotCmd.MarkFlagRequired("destination")
}

func plotTrip(cmd *cobra.Command, args []string) error {
	db, err := cache.Connect(false)
	if err != nil {
		return err
	}
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

	starSrc := &models.Star{Designation: src}
	if err := starSrc.Get(); err != nil {
		return err
	}
	starDst := &models.Star{Designation: dst}
	if err := starDst.Get(); err != nil {
		return err
	}
	
	sPos := models.NewPosition(
		starSrc.Position.X, starSrc.Position.Y, starSrc.Position.Z)
	dPos := models.NewPosition(
		starDst.Position.X, starDst.Position.Y, starDst.Position.Z)
	dist := sPos.Distance(dPos)
	waypoints := map[string]*models.JourneyLeg{
		src: {
			To: src,
			ToPosition: sPos,
			DistToDest: dist,
		},
	}

	queue := []string{src}
	for {
		// Sort by distance travelled + left to go
		slices.SortFunc(queue, func(a, b string) int {
			return cmp.Compare(
				waypoints[a].DistFromSrc + waypoints[a].DistToDest,
				waypoints[b].DistFromSrc + waypoints[b].DistToDest)
		})
		//fmt.Printf("starting iteration over queue: %v\n", queue)
		var nextQueue []string
		for _, s := range queue {
			//fmt.Printf("=== %s\n", s)
			// Get possible next steps
			stars, err := TripStepCandidate(db, s, dst, hop)
			if err != nil {
				return err
			}
			for _, next := range stars {
				next.DistFromSrc += waypoints[s].DistFromSrc
				ex, ok := waypoints[next.To]
				if !ok {
					// New waypoint, add it to the queue and move on
					waypoints[next.To] = next
					nextQueue = append(nextQueue, next.To)
					//fmt.Printf("  New waypoint: %s -> %s (behind: %.2f, ahead: %.2f)\n",
					//	next.From, next.To, next.DistFromSrc, next.DistToDest)
					continue
				}
				// Existing waypoint, if it's a shorter path to get there,
				// update it.
				if ex.DistFromSrc > next.DistFromSrc {
					// fmt.Printf("  Shorter path to %q, from %q (%.2f) rather than %q (%.2f)\n",
					//	next.To, next.From, next.DistFromSrc, ex.From, ex.DistFromSrc)
					ex.DistFromSrc = next.DistFromSrc
					ex.From = next.From
					ex.FromPosition = next.FromPosition
					waypoints[next.To] = ex
					continue
				}
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

		if closest == 0 {
			// Print it
			var lines []string
			for {
				lines = append(lines, fmt.Sprintf(
					" -> %s (%.2fly:%.2fly)", cur, waypoints[cur].DistFromSrc, waypoints[cur].DistToDest,
				))
				if cur == src { break }
				cur = waypoints[cur].From
			}
			slices.Reverse(lines)
			fmt.Printf("%s (%.2fly)\n", src, dist)
			for _, l := range lines {
				fmt.Println(l)
			}
			return nil
		}

		if len(nextQueue) == 0 { break }
		queue = nextQueue
	}

	return nil
}

func TripStepCandidate(db *cache.Cache, srcStar, dstStar string, radius float32) ([]*models.JourneyLeg, error) {
	// Get source coords
	src := &models.Star{Designation: srcStar}
	if err := src.Get(); err != nil {
		return nil, err
	}
	// Get dest coords
	dst := &models.Star{Designation: dstStar}
	if err := dst.Get(); err != nil {
		return nil, err
	}
	rows, err := db.DB.Query(
		`SELECT designation,
			position_x,
			position_y,
			position_z,
		    sqrt(
				power(position_x-?,2) +
				power(position_y-?,2) +
				power(position_z-?,2)) AS from_src,
		    sqrt(
				power(position_x-?,2) +
				power(position_y-?,2) +
				power(position_z-?,2)) AS from_dst
		FROM stars WHERE from_src <= ? AND from_src > 0.001`,
		src.Position.X,
		src.Position.Y,
		src.Position.Z,
		dst.Position.X,
		dst.Position.Y,
		dst.Position.Z,
		radius,
	)
	if err != nil {
		return nil, err
	}

	var res []*models.JourneyLeg
	for rows.Next() {
		var desg string
		var x, y, z, fSrc, fDst float32
		rows.Scan(&desg, &x, &y, &z, &fSrc, &fDst)
		res = append(res, &models.JourneyLeg{
				From: src.Designation,
				FromPosition: models.NewPosition(
					src.Position.X,
					src.Position.Y,
					src.Position.Z),
				To: desg,
				ToPosition: models.NewPosition(x, y, z),
				DistFromSrc: fSrc,
				DistToDest: fDst,
			},
		)
	}

	return res, nil
}

