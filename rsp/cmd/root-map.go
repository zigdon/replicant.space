package cmd

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
	"github.com/zigdon/rsp/rest"
)

var mapCmd = &cobra.Command{
	Use:               "map [center_star_or_coords]",
	Short:             "Visualize stars and systems in 3D terminal space",
	ValidArgsFunction: completeStars,
	RunE:              runMapCmd,
}

var plotMapCmd = &cobra.Command{
	Use:               "map <src> <dst>",
	Short:             "Visualize a multi-hop trip in 3D terminal space",
	ValidArgsFunction: completeStars,
	RunE:              runPlotMapCmd,
}

func init() {
	rootCmd.AddCommand(mapCmd)
	mapCmd.Flags().Float32P("radius", "r", 500.0, "Viewing radius in light-years")
	mapCmd.Flags().StringP("center", "c", "GORUMIUN", "Center star or coordinates (X,Y,Z)")
	mapCmd.Flags().StringP("plane", "p", "3d", "Projection plane: 3d, xy, xz, yz")
	mapCmd.Flags().BoolP("static", "s", false, "Render static ASCII snapshot to stdout")
	mapCmd.Flags().Bool("life", false, "Filter stars with intelligent life")
	mapCmd.Flags().Bool("hubs", false, "Filter stars with hubs")
	mapCmd.Flags().Bool("explored", false, "Filter explored stars only")
	mapCmd.Flags().String("region", "", "Filter stars by region (e.g. solzone, alpha, beta, gamma)")
	mapCmd.Flags().Bool("regions", false, "Overlay region colors and tags")
	mapCmd.Flags().StringSliceP("devices", "d", []string{"heaven_vessel", "racing_vessel", "cargo_vessel"},
		"List of device types to overlay (comma-separated, e.g. autofactory,mining_drone)")
	mapCmd.Flags().Bool("device_only", false, "Filter map to only show stars with matching devices")
	mapCmd.Flags().BoolP("network", "N", false, "Overlay FTL relay network connections")
	mapCmd.Flags().Bool("network_only", false, "Filter map to only show stars with active relay network devices")
	mapCmd.Flags().IntP("width", "w", 100, "Width in columns for static map")
	mapCmd.Flags().IntP("height", "H", 35, "Height in rows for static map")

	// Also register under plot
	plotCmd.AddCommand(plotMapCmd)
	plotMapCmd.Flags().Float32P("max_hop", "m", 7.5, "Maximum allowed hop, in ly")
	plotMapCmd.Flags().BoolP("use_station", "u", false, "Allow using deep space relay stations")
	plotMapCmd.Flags().BoolP("static", "s", false, "Render static ASCII snapshot to stdout")
	plotMapCmd.Flags().StringSliceP("devices", "d", nil, "List of device types to overlay (comma-separated, e.g. autofactory,mining_drone)")
	plotMapCmd.Flags().BoolP("network", "N", false, "Overlay FTL relay network connections")
}

func parseCenterPosition(input string) (common.Vec3, string, error) {
	if input == "" {
		input = "GORUMIUN"
	}

	if strings.Contains(input, ",") || strings.Contains(input, ":") {
		pos, err := models.ParsePosition(input)
		if err != nil {
			return common.Vec3{}, input, err
		}
		return common.Vec3{X: pos.X, Y: pos.Y, Z: pos.Z}, input, nil
	}

	st, err := models.NewStar(input)
	if err != nil {
		return common.Vec3{}, input, fmt.Errorf("star %q not found: %w", input, err)
	}
	if st.Position == nil {
		return common.Vec3{}, input, fmt.Errorf("star %q has no coordinate position", input)
	}
	return common.Vec3{X: st.Position.X, Y: st.Position.Y, Z: st.Position.Z}, string(st.Designation), nil
}

func loadDeviceLocations(deviceTypes []string) map[string][]*common.DeviceLocationInfo {
	if db == nil || len(deviceTypes) == 0 {
		return nil
	}
	devRecords, err := db.QueryDevicesByTypes(deviceTypes)
	if err != nil {
		log("Error querying devices: %v", err)
		return nil
	}
	starDevices := make(map[string][]*common.DeviceLocationInfo)
	for _, d := range devRecords {
		star := models.LocationID(d.Location).Star()
		starDevices[star] = append(starDevices[star], &common.DeviceLocationInfo{
			Code:     d.Code,
			Type:     d.Type,
			Location: d.Location,
			Status:   d.Status,
		})
	}
	return starDevices
}

func loadNetworkGraph(stars []*models.Star) *common.NetworkGraph {
	if db == nil {
		return nil
	}
	devs, err := db.QueryRelayingNetworkDevices()
	if err != nil {
		log("Error querying network devices: %v", err)
		return nil
	}
	if len(devs) == 0 {
		return nil
	}

	starLookup := make(map[string]common.Vec3)
	for _, s := range stars {
		if s != nil && s.Position != nil {
			starLookup[string(s.Designation)] = common.Vec3{X: s.Position.X, Y: s.Position.Y, Z: s.Position.Z}
		}
	}

	var netDevs []*common.NetworkDevice
	for _, d := range devs {
		netDevs = append(netDevs, &common.NetworkDevice{
			Code:     d.Code,
			Type:     d.Type,
			Location: d.Location,
			Status:   d.Status,
			RangeLy:  d.RangeLy,
		})
	}

	return common.BuildNetworkGraph(netDevs, starLookup)
}

func loadStarsFromDB(center common.Vec3, radius float32) ([]*models.Star, error) {
	if db == nil {
		return nil, fmt.Errorf("database cache is not connected")
	}

	records, err := db.QueryStarsInRadius(center.X, center.Y, center.Z, radius*1.5, 0)
	if err != nil {
		return nil, err
	}

	var stars []*models.Star
	for _, r := range records {
		stars = append(stars, &models.Star{
			Designation:      models.LocationID(r.Designation),
			Name:             r.Name,
			EntryPoint:       models.LocationID(r.EntryPoint),
			EstimatedPlanets: r.EstPlanets,
			SpectralType:     r.SpectralType,
			Explored:         r.Explored,
			HasLife:          r.HasLife,
			Position:         models.ParseCube(r.Position),
			HasHub:           r.HasHub,
			HasMyHub:         r.HasMyHub,
			Region:           r.Region,
		})
	}
	return stars, nil
}

func runPlotMapCmd(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("source and destination are required: plot map <src> <dst>")
	}
	src, dst := args[0], args[1]

	hop := getFloat32(cmd, "max_hop")
	if hop == 0 {
		hop = 7.5
	}
	useStation := getBool(cmd, "use_station")
	staticMode := getBool(cmd, "static")

	pCfg := &common.PlotCfg{
		Hop:        hop,
		UseStation: useStation,
		Partial:    true,
	}

	trip, err := common.PlotTrip(src, dst, pCfg)
	if err != nil {
		return fmt.Errorf("failed to plot trip: %w", err)
	}
	if trip == nil || len(trip.Legs) == 0 {
		return fmt.Errorf("no route found between %s and %s", src, dst)
	}

	// Calculate bounding center and radius for the route
	srcStar, err := models.NewStar(src)
	if err != nil {
		return err
	}
	dstStar, err := models.NewStar(dst)
	if err != nil {
		return err
	}

	// Populate positions on legs
	for _, leg := range trip.Legs {
		if leg.FromPosition == nil {
			if s, err := models.NewStar(leg.From); err == nil {
				leg.FromPosition = s.Position
			}
		}
		if leg.ToPosition == nil {
			if s, err := models.NewStar(leg.To); err == nil {
				leg.ToPosition = s.Position
			}
		}
	}

	midX := (srcStar.Position.X + dstStar.Position.X) / 2.0
	midY := (srcStar.Position.Y + dstStar.Position.Y) / 2.0
	midZ := (srcStar.Position.Z + dstStar.Position.Z) / 2.0
	center := common.Vec3{X: midX, Y: midY, Z: midZ}

	routeDist := srcStar.Position.Distance(dstStar.Position)
	radius := (routeDist / 2.0) * 1.4
	if radius < 15.0 {
		radius = 15.0
	}

	stars, err := loadStarsFromDB(center, radius)
	if err != nil {
		return err
	}

	opts := common.DefaultMapLayerOptions()
	opts.ShowRoute = true
	opts.RouteLegs = trip.Legs
	opts.SelectedStar = src
	opts.HighlightStar = dst

	deviceTypes := getStringSlice(cmd, "devices")
	if len(deviceTypes) > 0 {
		opts.DeviceTypes = deviceTypes
		opts.ShowDevices = true
		opts.StarDevices = loadDeviceLocations(deviceTypes)
	}
	opts.Network = loadNetworkGraph(stars)
	if getBool(cmd, "network") {
		opts.ShowNetwork = true
	}

	if staticMode {
		cam := common.NewCamera3D(100, 35)
		cam.Center = center
		cam.Radius = radius
		output, mapped := common.RenderGalaxyMap(cam, stars, opts)
		fmt.Print(common.FormatMapHeader(cam, len(stars), len(mapped), nil))
		fmt.Println(output)
		fmt.Println(common.FormatMapLegend(opts))
		fmt.Printf("\x1b[1;33mRoute:\x1b[0m %s -> %s (Total Distance: %.2fly, %d hops)\n",
			src, dst, routeDist, len(trip.Legs))
		return nil
	}

	return launchInteractiveMap(center, radius, stars, opts, src)
}

func runMapCmd(cmd *cobra.Command, args []string) error {
	centerInput := getString(cmd, "center")
	if len(args) > 0 {
		centerInput = args[0]
	}
	radius := getFloat32(cmd, "radius")
	if radius <= 0 {
		radius = 30.0
	}

	center, centerName, err := parseCenterPosition(centerInput)
	if err != nil {
		return err
	}

	stars, err := loadStarsFromDB(center, radius)
	if err != nil {
		return fmt.Errorf("failed to query stars: %w", err)
	}

	opts := common.DefaultMapLayerOptions()
	if getBool(cmd, "life") {
		opts.FilterLifeOnly = true
	}
	if getBool(cmd, "hubs") {
		opts.FilterHubsOnly = true
	}
	if getBool(cmd, "explored") {
		opts.FilterExploredOnly = true
	}
	if getBool(cmd, "regions") {
		opts.ShowRegions = true
	}
	if reg := getString(cmd, "region"); reg != "" {
		opts.FilterRegion = reg
	}
	deviceTypes := getStringSlice(cmd, "devices")
	if len(deviceTypes) > 0 {
		opts.DeviceTypes = deviceTypes
		opts.ShowDevices = true
		opts.StarDevices = loadDeviceLocations(deviceTypes)
		if getBool(cmd, "device_only") {
			opts.FilterDevicesOnly = true
		}
	}
	opts.Network = loadNetworkGraph(stars)
	if getBool(cmd, "network") {
		opts.ShowNetwork = true
	}
	if getBool(cmd, "network_only") {
		opts.ShowNetwork = true
		opts.FilterNetworkOnly = true
	}
	opts.SelectedStar = centerName

	staticMode := getBool(cmd, "static")
	plane := strings.ToLower(getString(cmd, "plane"))
	width := getInt(cmd, "width")
	height := getInt(cmd, "height")

	if staticMode {
		cam := common.NewCamera3D(width, height)
		cam.Center = center
		cam.Radius = radius
		switch plane {
		case "xy":
			cam.Mode = common.ProjPlaneXY
		case "xz":
			cam.Mode = common.ProjPlaneXZ
		case "yz":
			cam.Mode = common.ProjPlaneYZ
		default:
			cam.Mode = common.Proj3DOrthographic
		}

		output, mapped := common.RenderGalaxyMap(cam, stars, opts)
		fmt.Print(common.FormatMapHeader(cam, len(stars), len(mapped), nil))
		fmt.Println(output)
		fmt.Println(common.FormatMapLegend(opts))

		if opts.ShowDevices && len(opts.StarDevices) > 0 {
			fmt.Println("\n\x1b[1;33m=== DEVICE LOCATIONS ===\x1b[0m")
			var devStars []string
			for s := range opts.StarDevices {
				devStars = append(devStars, s)
			}
			slices.Sort(devStars)
			for _, s := range devStars {
				devs := opts.StarDevices[s]
				typeMap := make(map[string]int)
				for _, d := range devs {
					typeMap[d.Type]++
				}
				var typeStrs []string
				for t, c := range typeMap {
					typeStrs = append(typeStrs, fmt.Sprintf("%d × %s", c, t))
				}
				fmt.Printf("  \x1b[1;36m%-16s\x1b[0m (%d devices): %s\n", s, len(devs), strings.Join(typeStrs, ", "))
			}
		}

		if opts.ShowNetwork && opts.Network != nil && len(opts.Network.Nodes) > 0 {
			fmt.Printf("\n\x1b[1;36m=== FTL RELAY NETWORK (%d nodes, %d links, %d subnets) ===\x1b[0m\n",
				len(opts.Network.Nodes), len(opts.Network.Links), len(opts.Network.Subnets))
			var subnetIDs []int
			for id := range opts.Network.Subnets {
				subnetIDs = append(subnetIDs, id)
			}
			slices.Sort(subnetIDs)
			for _, id := range subnetIDs {
				nodes := opts.Network.Subnets[id]
				var starNames []string
				for _, n := range nodes {
					starNames = append(starNames, n.Star)
				}
				slices.Sort(starNames)
				if len(starNames) > 8 {
					starNames = append(starNames[:8], fmt.Sprintf("... +%d more", len(starNames)-8))
				}
				fmt.Printf("  \x1b[1;33mSubnet #%d\x1b[0m (%d stars): %s\n", id, len(nodes), strings.Join(starNames, ", "))
			}
		}
		return nil
	}

	return launchInteractiveMap(center, radius, stars, opts, centerName)
}

// searchMapTarget resolves a query string to a Star system via system name, device code, or device alias.
func searchMapTarget(query string, loadedStars []*models.Star) (*models.Star, string, error) {
	if query == "" {
		return nil, "", fmt.Errorf("empty search query")
	}

	q := strings.TrimSpace(query)
	qLower := strings.ToLower(q)

	// 1. Exact match across loaded stars
	for _, st := range loadedStars {
		if st == nil {
			continue
		}
		if strings.EqualFold(string(st.Designation), q) || (st.Name != "" && strings.EqualFold(st.Name, q)) {
			return st, fmt.Sprintf("Found system %s (%s)", st.Designation, st.Name), nil
		}
	}

	// 2. Prefix match across loaded stars
	for _, st := range loadedStars {
		if st == nil {
			continue
		}
		if strings.HasPrefix(strings.ToLower(string(st.Designation)), qLower) || (st.Name != "" && strings.HasPrefix(strings.ToLower(st.Name), qLower)) {
			return st, fmt.Sprintf("Found system %s (%s)", st.Designation, st.Name), nil
		}
	}

	// 3. Device alias or device code resolution (e.g. hv-1, r-1, af-2, sh-1, d-xxx)
	dealiased := q
	if db != nil {
		dealiased = db.Dealias(q)
	}

	if db != nil && db.DB != nil {
		var devCode, devType, devLoc string
		row := db.DB.QueryRow(`
			SELECT code, type, location 
			FROM json_devices 
			WHERE LOWER(code) = $1 OR LOWER(code) = $2 
			LIMIT 1`, strings.ToLower(q), strings.ToLower(dealiased))
		if err := row.Scan(&devCode, &devType, &devLoc); err == nil && devLoc != "" {
			starName := models.LocationID(devLoc).Star()
			st, err := models.NewStar(starName)
			if err == nil && st != nil {
				aliasName := q
				if db != nil {
					if a := db.HasAlias(devCode); a != "" {
						aliasName = a
					}
				}
				return st, fmt.Sprintf("Found device %s (%s) at %s (%s)", aliasName, devType, devLoc, starName), nil
			}
		}
	}

	// 4. Replicant resolution (e.g. r-1, r-2)
	if strings.HasPrefix(qLower, "r-") || strings.HasPrefix(qLower, "replicant-") {
		ca := models.NewCodeAlias(q)
		if rep, err := rest.Replicant(ca); err == nil && rep != nil {
			loc := rep.CurrentLocation
			if loc == "" {
				loc = rep.Location
			}
			if loc != "" {
				starName := models.LocationID(loc).Star()
				st, err := models.NewStar(starName)
				if err == nil && st != nil {
					return st, fmt.Sprintf("Found replicant %s at %s (%s)", q, loc, starName), nil
				}
			}
		}
	}

	// 5. Look up star directly in database
	st, err := models.NewStar(q)
	if err == nil && st != nil && st.Position != nil {
		return st, fmt.Sprintf("Found system %s (%s)", st.Designation, st.Name), nil
	}

	// 6. Substring match across loaded stars
	for _, st := range loadedStars {
		if st == nil {
			continue
		}
		if strings.Contains(strings.ToLower(string(st.Designation)), qLower) || (st.Name != "" && strings.Contains(strings.ToLower(st.Name), qLower)) {
			return st, fmt.Sprintf("Found system %s (%s)", st.Designation, st.Name), nil
		}
	}

	return nil, "", fmt.Errorf("no system, device, or alias found matching %q", query)
}

func launchInteractiveMap(center common.Vec3, radius float32, stars []*models.Star, opts *common.MapLayerOptions, initialTarget string) error {
	app := tview.NewApplication()
	app.EnableMouse(true)

	cam := common.NewCamera3D(80, 24)
	cam.Center = center
	cam.Radius = radius

	var selectedIndex int
	var currentMapped []*common.StarMapPoint
	var isSearching bool
	var searchStatusMsg string

	mapView := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(false).
		SetWrap(false)

	sidebar := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true)

	hud := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false)

	help := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false)

	help.SetText(" [yellow]Arrows/hjkl[-] Rotate  [yellow]+/-[-] Zoom  [yellow]WASD/EC[-] Pan (X/Y/Z)  [yellow]/[-] Search  [yellow]Click[-] Select  [yellow]Tab[-] Target  [yellow]1-8[-] Filters  [yellow]r[-] Reset  [yellow]q[-] Quit")

	searchInput := tview.NewInputField().
		SetLabel(" [yellow::b]Search (System / Device / Alias):[-::-] ").
		SetFieldWidth(40).
		SetFieldBackgroundColor(tcell.ColorDarkSlateGray).
		SetFieldTextColor(tcell.ColorWhite)

	updateSidebar := func(target *common.StarMapPoint) {
		sidebar.Clear()
		if target == nil || target.Star == nil {
			sidebar.SetText("[cyan::b]=== TARGET DETAILS ===[-::-]\n\n[gray]No star selected.[-]\nUse [yellow]Tab[-] to cycle stars.")
			return
		}

		st := target.Star
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("[cyan::b]=== %s ===[-::-]\n\n", st.Designation))
		if st.Name != "" {
			sb.WriteString(fmt.Sprintf("[white]Name:[-] [yellow]%s[-]\n", st.Name))
		}
		sb.WriteString(fmt.Sprintf("[white]Spectral Class:[-] [cyan]%s[-]\n", st.SpectralType))
		sb.WriteString(fmt.Sprintf("[white]Est. Planets:[-] [white]%d[-]\n", st.EstimatedPlanets))
		regCol := common.GetRegionColor(st.Region)
		sb.WriteString(fmt.Sprintf("[white]Region:[-] [#%02x%02x%02x::b]%s[-::-]\n", regCol.R, regCol.G, regCol.B, st.Region))

		lifeStr := "[gray]No[-]"
		if st.HasLife {
			lifeStr = "[green::b]YES (Intelligent)[-::-]"
		}
		sb.WriteString(fmt.Sprintf("[white]Life:[-] %s\n", lifeStr))

		hubStr := "[gray]None[-]"
		if st.HasMyHub {
			hubStr = "[magenta::b]PLAYER HUB[-::-]"
		} else if st.HasHub {
			hubStr = "[cyan::b]System Hub[-::-]"
		}
		sb.WriteString(fmt.Sprintf("[white]Hub Status:[-] %s\n", hubStr))

		exploredStr := "[gray]No[-]"
		if st.Explored {
			exploredStr = "[green]Explored[-]"
		}
		sb.WriteString(fmt.Sprintf("[white]Explored:[-] %s\n\n", exploredStr))

		if st.Position != nil {
			sb.WriteString(fmt.Sprintf("[white]Coordinates:[-]\n  X: [yellow]%.2f[-]\n  Y: [yellow]%.2f[-]\n  Z: [yellow]%.2f[-]\n\n",
				st.Position.X, st.Position.Y, st.Position.Z))
			distFromCenter := cam.Center.Distance(common.Vec3{X: st.Position.X, Y: st.Position.Y, Z: st.Position.Z})
			sb.WriteString(fmt.Sprintf("[white]Dist to Center:[-] [yellow]%.2fly[-]\n", distFromCenter))
		}

		if len(target.Devices) > 0 {
			sb.WriteString(fmt.Sprintf("\n[yellow::b]=== DEVICES AT STAR (%d) ===[-::-]\n", len(target.Devices)))
			typeCounts := make(map[string]int)
			for _, dev := range target.Devices {
				typeCounts[dev.Type]++
			}
			for dtype, cnt := range typeCounts {
				sb.WriteString(fmt.Sprintf("• [cyan]%s[-]: [white]%d[-] devices\n", dtype, cnt))
			}
			sb.WriteString("\n[gray]Device List:[-]\n")
			for i, dev := range target.Devices {
				if i >= 6 {
					sb.WriteString(fmt.Sprintf("  [gray]... and %d more[-]\n", len(target.Devices)-6))
					break
				}
				statusStr := dev.Status
				if statusStr == "" {
					statusStr = "-"
				}
				sb.WriteString(fmt.Sprintf("  [yellow]%s[-] ([white]%s[-])\n    Loc: [gray]%s[-]  Stat: [gray]%s[-]\n",
					dev.Code, dev.Type, dev.Location, statusStr))
			}
		}

		if target.NetworkNode != nil {
			netNode := target.NetworkNode
			subnetSize := 1
			if opts.Network != nil && opts.Network.Subnets != nil {
				subnetSize = len(opts.Network.Subnets[netNode.SubnetID])
			}
			sb.WriteString(fmt.Sprintf("\n[cyan::b]=== FTL NETWORK NODE ===[-::-]\n"))
			sb.WriteString(fmt.Sprintf("[white]Subnet:[-] [yellow]#%d[-] ([green]%d stars connected[-])\n", netNode.SubnetID, subnetSize))
			sb.WriteString(fmt.Sprintf("[white]Max Reach:[-] [yellow]%.1fly[-]\n", netNode.MaxRange))
			sb.WriteString(fmt.Sprintf("[white]Relaying Devices (%d):[-]\n", len(netNode.Devices)))
			for _, d := range netNode.Devices {
				sb.WriteString(fmt.Sprintf("  • [cyan]%s[-] ([gray]%s[-], reach: [yellow]%.1fly[-])\n", d.Type, d.Code, d.RangeLy))
			}
			sb.WriteString(fmt.Sprintf("[white]Direct Links (%d):[-]\n", len(netNode.Connections)))
			for i, link := range netNode.Connections {
				if i >= 6 {
					sb.WriteString(fmt.Sprintf("  [gray]... and %d more[-]\n", len(netNode.Connections)-6))
					break
				}
				otherStar := link.ToStar
				reachMode := "[green]bi-dir[-]"
				if otherStar == netNode.Star {
					otherStar = link.FromStar
					if !link.IsToReach {
						reachMode = "[yellow]inbound-only[-]"
					} else if !link.IsFromReach {
						reachMode = "[cyan]outbound-only[-]"
					}
				} else {
					if !link.IsFromReach {
						reachMode = "[yellow]inbound-only[-]"
					} else if !link.IsToReach {
						reachMode = "[cyan]outbound-only[-]"
					}
				}
				sb.WriteString(fmt.Sprintf("  • [yellow]%s[-] ([white]%.1fly[-], %s)\n", otherStar, link.Distance, reachMode))
			}
		}

		sidebar.SetText(sb.String())
	}

	redraw := func() {
		_, _, w, h := mapView.GetInnerRect()
		if w <= 10 || h <= 5 {
			w, h = 80, 24
		}
		cam.Width = w
		cam.Height = h

		output, mapped := common.RenderGalaxyMapTview(cam, stars, opts)
		currentMapped = mapped

		var selectedPoint *common.StarMapPoint
		if len(currentMapped) > 0 {
			if selectedIndex >= len(currentMapped) {
				selectedIndex = len(currentMapped) - 1
			}
			if selectedIndex < 0 {
				selectedIndex = 0
			}
			selectedPoint = currentMapped[selectedIndex]
			opts.SelectedStar = string(selectedPoint.Star.Designation)
		} else {
			opts.SelectedStar = ""
		}

		var filterBadges []string
		if opts.FilterLifeOnly {
			filterBadges = append(filterBadges, "[#55ff55::b][1:Life:ON][-::-]")
		} else {
			filterBadges = append(filterBadges, "[gray][1:Life][-]")
		}
		if opts.FilterHubsOnly {
			filterBadges = append(filterBadges, "[#ff55ff::b][2:Hubs:ON][-::-]")
		} else {
			filterBadges = append(filterBadges, "[gray][2:Hubs][-]")
		}
		if opts.FilterExploredOnly {
			filterBadges = append(filterBadges, "[#55ffff::b][3:Explored:ON][-::-]")
		} else {
			filterBadges = append(filterBadges, "[gray][3:Explored][-]")
		}
		if opts.ShowGrid {
			filterBadges = append(filterBadges, "[yellow][4:Grid][-]")
		} else {
			filterBadges = append(filterBadges, "[gray][4:Grid:OFF][-]")
		}
		if opts.ShowLabels {
			filterBadges = append(filterBadges, "[yellow][5:Labels][-]")
		} else {
			filterBadges = append(filterBadges, "[gray][5:Labels:OFF][-]")
		}
		if opts.ShowRegions {
			filterBadges = append(filterBadges, "[#00e5ff::b][6:Regions:ON][-::-]")
		} else {
			filterBadges = append(filterBadges, "[gray][6:Regions][-]")
		}
		if opts.ShowDevices {
			filterBadges = append(filterBadges, "[#ffff55::b][7:Devices:ON][-::-]")
		} else {
			filterBadges = append(filterBadges, "[gray][7:Devices][-]")
		}
		if opts.ShowNetwork {
			filterBadges = append(filterBadges, "[#00e5ff::b][8:Network:ON][-::-]")
		} else {
			filterBadges = append(filterBadges, "[gray][8:Network][-]")
		}

		var devSummary string
		if opts.StarDevices != nil && len(opts.StarDevices) > 0 {
			var totalDevs int
			for _, devs := range opts.StarDevices {
				totalDevs += len(devs)
			}
			devSummary = fmt.Sprintf(" | Devs: [yellow]%d[-] in %d systems", totalDevs, len(opts.StarDevices))
		}

		var netSummary string
		if opts.Network != nil && len(opts.Network.Nodes) > 0 {
			netSummary = fmt.Sprintf(" | Net: [cyan]%d nodes, %d links[-]", len(opts.Network.Nodes), len(opts.Network.Links))
		}

		var hudSb strings.Builder
		hudSb.WriteString(fmt.Sprintf("[cyan::b]=== GALAXY 3D MAP ===[-::-]  Center: [yellow]%s[-]  Radius: [green]%.1fly[-]  Mode: [magenta]%s[-]  Stars: [white]%d visible[-] / %d total%s%s\n",
			cam.Center.String(), cam.Radius, cam.Mode, len(currentMapped), len(stars), devSummary, netSummary))

		if selectedPoint != nil && selectedPoint.Star != nil {
			st := selectedPoint.Star
			name := st.Name
			if name == "" {
				name = "-"
			}
			lifeStr := "[gray]No[-]"
			if st.HasLife {
				lifeStr = "[green::b]YES (Intelligent)[-::-]"
			}
			hubStr := "[gray]None[-]"
			if st.HasMyHub {
				hubStr = "[magenta::b]PLAYER HUB[-::-]"
			} else if st.HasHub {
				hubStr = "[cyan::b]System Hub[-::-]"
			}

			devInfoStr := ""
			if len(selectedPoint.Devices) > 0 {
				devInfoStr = fmt.Sprintf(" | [yellow]Devices: %d[-]", len(selectedPoint.Devices))
			}
			if selectedPoint.NetworkNode != nil {
				devInfoStr += fmt.Sprintf(" | [cyan]Net#%d:%.1fly[-]", selectedPoint.NetworkNode.SubnetID, selectedPoint.NetworkNode.MaxRange)
			}

			hudSb.WriteString(fmt.Sprintf("[white::b]Target:[-] [yellow::b]%s[-] ([white]%s[-]) | Class: [cyan]%s[-] | Planets: [white]%d[-] | Life: %s | Hub: %s%s | Pos: %s\n",
				st.Designation, name, st.SpectralType, st.EstimatedPlanets, lifeStr, hubStr, devInfoStr, st.Position.String()))
		} else {
			hudSb.WriteString("[gray]Target: None matching current filters[-]\n")
		}

		if searchStatusMsg != "" {
			hudSb.WriteString(fmt.Sprintf("%s\n", searchStatusMsg))
		}
		hudSb.WriteString(fmt.Sprintf("Filters: %s", strings.Join(filterBadges, " ")))

		hud.SetText(hudSb.String())
		mapView.SetText(output)
		updateSidebar(selectedPoint)
	}

	// Layout arrangement
	mainFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	hud.SetBorder(true).SetTitle(" Stellar Cartography ")
	help.SetBorder(false)
	mapView.SetBorder(true).SetTitle(" 3D Viewport ")
	sidebar.SetBorder(true).SetTitle(" Star Inspector ")

	centerRow := tview.NewFlex().SetDirection(tview.FlexColumn)
	centerRow.AddItem(mapView, 0, 3, true)
	centerRow.AddItem(sidebar, 28, 1, false)

	mainFlex.AddItem(hud, 6, 0, false)
	mainFlex.AddItem(centerRow, 0, 1, true)
	mainFlex.AddItem(help, 1, 0, false)

	// Mouse Selection
	mapView.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action == tview.MouseLeftClick || action == tview.MouseLeftDoubleClick {
			mx, my := event.Position()
			vx, vy, vw, vh := mapView.GetInnerRect()
			if mx >= vx && mx < vx+vw && my >= vy && my < vy+vh {
				rx := mx - vx
				ry := my - vy

				var bestStar *common.StarMapPoint
				var bestDist float32 = 999999
				var bestIdx int = -1

				for i, mp := range currentMapped {
					if mp == nil || !mp.Visible {
						continue
					}
					// 1. Direct hit on star glyph
					if mp.ScreenX == rx && mp.ScreenY == ry {
						bestStar = mp
						bestIdx = i
						break
					}
					// 2. Hit on label text (to the right of glyph)
					labelLen := len(mp.Star.Name)
					if labelLen == 0 {
						labelLen = len(string(mp.Star.Designation))
					}
					if labelLen > 18 {
						labelLen = 18
					}
					if mp.ScreenY == ry && rx >= mp.ScreenX && rx <= mp.ScreenX+2+labelLen {
						bestStar = mp
						bestIdx = i
						break
					}
					// 3. Proximity hit within ~2 cells
					dx := float32(mp.ScreenX - rx)
					dy := float32((mp.ScreenY - ry) * 2)
					dist := dx*dx + dy*dy
					if dist < 8.0 && dist < bestDist {
						bestDist = dist
						bestStar = mp
						bestIdx = i
					}
				}

				if bestStar != nil {
					selectedIndex = bestIdx
					opts.SelectedStar = string(bestStar.Star.Designation)
					searchStatusMsg = fmt.Sprintf("[green]Selected: %s (%s)[-]", bestStar.Star.Designation, bestStar.Star.Name)
					redraw()
					return action, nil
				}
			}
		}
		return action, event
	})

	// Search Input Handling
	searchInput.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			query := strings.TrimSpace(searchInput.GetText())
			if query != "" {
				matched, info, err := searchMapTarget(query, stars)
				if err != nil {
					searchStatusMsg = fmt.Sprintf("[red::b]✗ %v[-::-]", err)
				} else if matched != nil {
					foundInSlice := false
					for _, s := range stars {
						if s != nil && s.Designation == matched.Designation {
							foundInSlice = true
							break
						}
					}
					if !foundInSlice {
						stars = append(stars, matched)
					}

					if matched.Position != nil {
						cam.Center = common.Vec3{X: matched.Position.X, Y: matched.Position.Y, Z: matched.Position.Z}
					}
					opts.SelectedStar = string(matched.Designation)
					initialTarget = string(matched.Designation)
					searchStatusMsg = fmt.Sprintf("[green::b]✓ %s[-::-]", info)
				}
			}
		}
		isSearching = false
		mainFlex.RemoveItem(searchInput)
		mainFlex.AddItem(help, 1, 0, false)
		app.SetFocus(mapView)
		redraw()
	})

	// Keyboard Controls
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		key := event.Key()
		r := event.Rune()

		if isSearching {
			if key == tcell.KeyEscape {
				isSearching = false
				mainFlex.RemoveItem(searchInput)
				mainFlex.AddItem(help, 1, 0, false)
				app.SetFocus(mapView)
				redraw()
				return nil
			}
			return event
		}

		switch {
		case key == tcell.KeyEscape || r == 'q' || r == 'Q':
			app.Stop()
			return nil

		// Search Hotkey
		case r == '/':
			isSearching = true
			searchStatusMsg = ""
			searchInput.SetText("")
			mainFlex.RemoveItem(help)
			mainFlex.AddItem(searchInput, 1, 0, true)
			app.SetFocus(searchInput)
			return nil

		// Rotation: Yaw and Pitch
		case key == tcell.KeyLeft || r == 'h':
			cam.Yaw -= 0.15
			redraw()
			return nil
		case key == tcell.KeyRight || r == 'l':
			cam.Yaw += 0.15
			redraw()
			return nil
		case key == tcell.KeyUp || r == 'k':
			cam.Pitch += 0.15
			redraw()
			return nil
		case key == tcell.KeyDown || r == 'j':
			cam.Pitch -= 0.15
			redraw()
			return nil

		// Zoom: Radius adjustment
		case r == '+' || r == '=':
			if cam.Radius > 3.0 {
				cam.Radius *= 0.85
			}
			redraw()
			return nil
		case r == '-' || r == '_':
			if cam.Radius < 1500.0 {
				cam.Radius *= 1.18
			}
			redraw()
			return nil

		// Pan Center: W/S (Y), A/D (X), E/C/Z (Z)
		case r == 'w' || r == 'W':
			cam.Center.Y += cam.Radius * 0.1
			redraw()
			return nil
		case r == 's' || r == 'S':
			cam.Center.Y -= cam.Radius * 0.1
			redraw()
			return nil
		case r == 'a' || r == 'A':
			cam.Center.X -= cam.Radius * 0.1
			redraw()
			return nil
		case r == 'd' || r == 'D':
			cam.Center.X += cam.Radius * 0.1
			redraw()
			return nil
		case r == 'e' || r == 'E' || key == tcell.KeyPgUp:
			cam.Center.Z += cam.Radius * 0.1
			redraw()
			return nil
		case r == 'c' || r == 'C' || r == 'z' || r == 'Z' || key == tcell.KeyPgDn:
			cam.Center.Z -= cam.Radius * 0.1
			redraw()
			return nil

		// Cycle Projections: 3D Ortho -> 3D Perspective -> XY -> XZ -> YZ
		case r == 'p' || r == 'P':
			switch cam.Mode {
			case common.Proj3DOrthographic:
				cam.Mode = common.Proj3DPerspective
			case common.Proj3DPerspective:
				cam.Mode = common.ProjPlaneXY
			case common.ProjPlaneXY:
				cam.Mode = common.ProjPlaneXZ
			case common.ProjPlaneXZ:
				cam.Mode = common.ProjPlaneYZ
			case common.ProjPlaneYZ:
				cam.Mode = common.Proj3DOrthographic
			}
			redraw()
			return nil

		// Cycle Selected Target Star
		case key == tcell.KeyTab:
			if len(currentMapped) > 0 {
				selectedIndex = (selectedIndex + 1) % len(currentMapped)
				opts.SelectedStar = string(currentMapped[selectedIndex].Star.Designation)
				redraw()
			}
			return nil
		case key == tcell.KeyBacktab:
			if len(currentMapped) > 0 {
				selectedIndex = (selectedIndex - 1 + len(currentMapped)) % len(currentMapped)
				opts.SelectedStar = string(currentMapped[selectedIndex].Star.Designation)
				redraw()
			}
			return nil

		// Layer toggles: 1=Life, 2=Hubs, 3=Explored, 4=Grid, 5=Labels, 6=Regions, 7=Devices, 8=Network
		case r == '1':
			opts.FilterLifeOnly = !opts.FilterLifeOnly
			selectedIndex = 0
			redraw()
			return nil
		case r == '2':
			opts.FilterHubsOnly = !opts.FilterHubsOnly
			selectedIndex = 0
			redraw()
			return nil
		case r == '3':
			opts.FilterExploredOnly = !opts.FilterExploredOnly
			selectedIndex = 0
			redraw()
			return nil
		case r == '4':
			opts.ShowGrid = !opts.ShowGrid
			redraw()
			return nil
		case r == '5':
			opts.ShowLabels = !opts.ShowLabels
			redraw()
			return nil
		case r == '6':
			opts.ShowRegions = !opts.ShowRegions
			redraw()
			return nil
		case r == '7':
			opts.ShowDevices = !opts.ShowDevices
			redraw()
			return nil
		case r == '8' || r == 'n' || r == 'N':
			opts.ShowNetwork = !opts.ShowNetwork
			redraw()
			return nil

		// Reset View
		case r == 'r' || r == 'R':
			cam.Center = center
			cam.Radius = radius
			cam.Pitch = 0.35
			cam.Yaw = 0.45
			cam.Mode = common.Proj3DOrthographic
			opts.FilterLifeOnly = false
			opts.FilterHubsOnly = false
			opts.FilterExploredOnly = false
			opts.FilterRegion = ""
			opts.FilterDevicesOnly = false
			opts.FilterNetworkOnly = false
			opts.ShowRegions = false
			opts.ShowDevices = (len(opts.DeviceTypes) > 0)
			opts.ShowNetwork = (opts.Network != nil && len(opts.Network.Nodes) > 0)
			opts.ShowGrid = true
			opts.ShowLabels = true
			opts.SelectedStar = initialTarget
			searchStatusMsg = ""
			selectedIndex = 0
			redraw()
			return nil
		}

		return event
	})

	mapView.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		redraw()
		return x, y, width, height
	})

	if err := app.SetRoot(mainFlex, true).Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running 3D Map viewer: %v\n", err)
		return err
	}
	return nil
}
