package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
	"github.com/zigdon/rsp/common"
	"github.com/zigdon/rsp/models"
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
	mapCmd.Flags().Float32P("radius", "r", 30.0, "Viewing radius in light-years")
	mapCmd.Flags().StringP("center", "c", "GORUMIUN", "Center star or coordinates (X,Y,Z)")
	mapCmd.Flags().StringP("plane", "p", "3d", "Projection plane: 3d, xy, xz, yz")
	mapCmd.Flags().BoolP("static", "s", false, "Render static ASCII snapshot to stdout")
	mapCmd.Flags().Bool("life", false, "Filter stars with intelligent life")
	mapCmd.Flags().Bool("hubs", false, "Filter stars with hubs")
	mapCmd.Flags().Bool("explored", false, "Filter explored stars only")
	mapCmd.Flags().IntP("width", "w", 100, "Width in columns for static map")
	mapCmd.Flags().IntP("height", "H", 35, "Height in rows for static map")

	// Also register under plot
	plotCmd.AddCommand(plotMapCmd)
	plotMapCmd.Flags().Float32P("max_hop", "m", 7.5, "Maximum allowed hop, in ly")
	plotMapCmd.Flags().BoolP("use_station", "u", false, "Allow using deep space relay stations")
	plotMapCmd.Flags().BoolP("static", "s", false, "Render static ASCII snapshot to stdout")
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

	if staticMode {
		cam := common.NewCamera3D(100, 35)
		cam.Center = center
		cam.Radius = radius
		output, mapped := common.RenderGalaxyMap(cam, stars, opts)
		fmt.Print(common.FormatMapHeader(cam, len(stars), len(mapped), nil))
		fmt.Println(output)
		fmt.Println(common.FormatMapLegend())
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
		opts.ShowLife = true
	}
	if getBool(cmd, "hubs") {
		opts.ShowHubs = true
	}
	if getBool(cmd, "explored") {
		opts.ShowExploredOnly = true
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
		fmt.Println(common.FormatMapLegend())
		return nil
	}

	return launchInteractiveMap(center, radius, stars, opts, centerName)
}

func launchInteractiveMap(center common.Vec3, radius float32, stars []*models.Star, opts *common.MapLayerOptions, initialTarget string) error {
	app := tview.NewApplication()

	cam := common.NewCamera3D(80, 24)
	cam.Center = center
	cam.Radius = radius

	var selectedIndex int
	var currentMapped []*common.StarMapPoint

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

	help.SetText(" [yellow]Arrows/hjkl[-] Rotate  [yellow]+/-[-] Zoom  [yellow]WASD[-] Pan  [yellow]p[-] Proj  [yellow]Tab[-] Target  [yellow]1-5[-] Layers  [yellow]r[-] Reset  [yellow]q[-] Quit")

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
		sb.WriteString(fmt.Sprintf("[white]Region:[-] [white]%s[-]\n", st.Region))

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
		}

		var hudSb strings.Builder
		hudSb.WriteString(fmt.Sprintf("[cyan::b]=== GALAXY 3D MAP ===[-::-]  Center: [yellow]%s[-]  Radius: [green]%.1fly[-]  Mode: [magenta]%s[-]  Stars: [white]%d visible[-] / %d total\n",
			cam.Center.String(), cam.Radius, cam.Mode, len(currentMapped), len(stars)))

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

			hudSb.WriteString(fmt.Sprintf("[white::b]Target:[-] [yellow::b]%s[-] ([white]%s[-]) | Class: [cyan]%s[-] | Planets: [white]%d[-] | Life: %s | Hub: %s | Pos: %s",
				st.Designation, name, st.SpectralType, st.EstimatedPlanets, lifeStr, hubStr, st.Position.String()))
		}

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

	mainFlex.AddItem(hud, 4, 0, false)
	mainFlex.AddItem(centerRow, 0, 1, true)
	mainFlex.AddItem(help, 1, 0, false)

	// Keyboard Controls
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		key := event.Key()
		r := event.Rune()

		switch {
		case key == tcell.KeyEscape || r == 'q' || r == 'Q':
			app.Stop()
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

		// Pan Center: W, A, S, D
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

		// Layer toggles: 1=Life, 2=Hubs, 3=Explored, 4=Grid, 5=Labels
		case r == '1':
			opts.ShowLife = !opts.ShowLife
			redraw()
			return nil
		case r == '2':
			opts.ShowHubs = !opts.ShowHubs
			redraw()
			return nil
		case r == '3':
			opts.ShowExploredOnly = !opts.ShowExploredOnly
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

		// Reset View
		case r == 'r' || r == 'R':
			cam.Center = center
			cam.Radius = radius
			cam.Pitch = 0.35
			cam.Yaw = 0.45
			cam.Mode = common.Proj3DOrthographic
			opts.SelectedStar = initialTarget
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
