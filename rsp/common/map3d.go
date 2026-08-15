package common

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/zigdon/rsp/models"
)

// Vec3 represents a 3D coordinate or vector.
type Vec3 struct {
	X float32
	Y float32
	Z float32
}

func NewVec3(x, y, z float32) Vec3 {
	return Vec3{X: x, Y: y, Z: z}
}

func (v Vec3) Add(other Vec3) Vec3 {
	return Vec3{X: v.X + other.X, Y: v.Y + other.Y, Z: v.Z + other.Z}
}

func (v Vec3) Sub(other Vec3) Vec3 {
	return Vec3{X: v.X - other.X, Y: v.Y - other.Y, Z: v.Z - other.Z}
}

func (v Vec3) Mul(s float32) Vec3 {
	return Vec3{X: v.X * s, Y: v.Y * s, Z: v.Z * s}
}

func (v Vec3) Length() float32 {
	return float32(math.Sqrt(float64(v.X*v.X + v.Y*v.Y + v.Z*v.Z)))
}

func (v Vec3) Distance(other Vec3) float32 {
	return v.Sub(other).Length()
}

func (v Vec3) String() string {
	return fmt.Sprintf("[%.2f, %.2f, %.2f]", v.X, v.Y, v.Z)
}

// RGB represents an 8-bit color.
type RGB struct {
	R, G, B uint8
}

func (c RGB) ANSI() string {
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.R, c.G, c.B)
}

func (c RGB) ANSIBg() string {
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", c.R, c.G, c.B)
}

func (c RGB) Dim(factor float32) RGB {
	if factor < 0 {
		factor = 0
	}
	if factor > 1 {
		factor = 1
	}
	return RGB{
		R: uint8(float32(c.R) * factor),
		G: uint8(float32(c.G) * factor),
		B: uint8(float32(c.B) * factor),
	}
}

const ANSIReset = "\x1b[0m"

// Spectral colors for star classification
var spectralColors = map[string]RGB{
	"O": {R: 85, G: 153, B: 255},  // Deep Blue
	"B": {R: 136, G: 204, B: 255}, // Light Blue / Cyan
	"A": {R: 240, G: 245, B: 255}, // Brilliant White
	"F": {R: 255, G: 244, B: 204}, // Cream / Yellow-White
	"G": {R: 255, G: 221, B: 68},  // Solar Yellow
	"K": {R: 255, G: 153, B: 51},  // Orange
	"M": {R: 255, G: 68, B: 68},   // Red Dwarf / Giant
}

func GetSpectralColor(spec string) RGB {
	if len(spec) > 0 {
		class := strings.ToUpper(string(spec[0]))
		if col, ok := spectralColors[class]; ok {
			return col
		}
	}
	return RGB{R: 170, G: 170, B: 170}
}

// ProjectionMode enum
type ProjectionMode string

const (
	Proj3DOrthographic ProjectionMode = "3d_ortho"
	Proj3DPerspective  ProjectionMode = "3d_perspective"
	ProjPlaneXY        ProjectionMode = "plane_xy"
	ProjPlaneXZ        ProjectionMode = "plane_xz"
	ProjPlaneYZ        ProjectionMode = "plane_yz"
)

// Camera3D encapsulates viewport geometry, rotation angles, and projection.
type Camera3D struct {
	Center      Vec3
	Radius      float32 // Viewport radius in light-years
	Pitch       float32 // Rotation around X (radians)
	Yaw         float32 // Rotation around Y (radians)
	Roll        float32 // Rotation around Z (radians)
	Mode        ProjectionMode
	Width       int     // Terminal character columns
	Height      int     // Terminal character rows
	AspectRatio float32 // Terminal character aspect ratio (default ~2.0: 1 char height = 2 char widths)
	FOV         float32 // Field of view angle in radians for perspective (e.g. 60 deg = ~1.047)
}

func NewCamera3D(w, h int) *Camera3D {
	return &Camera3D{
		Center:      Vec3{0, 0, 0},
		Radius:      25.0,
		Pitch:       0.35, // Slight top-down tilt
		Yaw:         0.45, // Angled perspective
		Roll:        0,
		Mode:        Proj3DOrthographic,
		Width:       w,
		Height:      h,
		AspectRatio: 2.0, // Character height is approx twice character width
		FOV:         1.0,
	}
}

// Transform projects a 3D world coordinate into camera space and 2D terminal character coordinates.
func (c *Camera3D) Transform(worldPos Vec3) (camPos Vec3, screenX, screenY int, visible bool) {
	rel := worldPos.Sub(c.Center)

	switch c.Mode {
	case ProjPlaneXY:
		camPos = Vec3{X: rel.X, Y: rel.Y, Z: rel.Z}
	case ProjPlaneXZ:
		camPos = Vec3{X: rel.X, Y: rel.Z, Z: -rel.Y}
	case ProjPlaneYZ:
		camPos = Vec3{X: rel.Y, Y: rel.Z, Z: -rel.X}
	default: // 3D with Euler angles
		// 1. Yaw around Y axis
		cosY := float32(math.Cos(float64(c.Yaw)))
		sinY := float32(math.Sin(float64(c.Yaw)))
		x1 := rel.X*cosY + rel.Z*sinY
		y1 := rel.Y
		z1 := -rel.X*sinY + rel.Z*cosY

		// 2. Pitch around X axis
		cosP := float32(math.Cos(float64(c.Pitch)))
		sinP := float32(math.Sin(float64(c.Pitch)))
		x2 := x1
		y2 := y1*cosP - z1*sinP
		z2 := y1*sinP + z1*cosP

		// 3. Roll around Z axis
		cosR := float32(math.Cos(float64(c.Roll)))
		sinR := float32(math.Sin(float64(c.Roll)))
		x3 := x2*cosR - y2*sinR
		y3 := x2*sinR + y2*cosR
		z3 := z2

		camPos = Vec3{X: x3, Y: y3, Z: z3}
	}

	halfW := float32(c.Width) / 2.0
	halfH := float32(c.Height) / 2.0

	// Scale factor: radius maps to half the smaller viewport dimension
	scale := (halfW) / c.Radius
	if c.Mode == Proj3DPerspective {
		// Perspective projection
		camDist := c.Radius * 1.8
		denom := camPos.Z + camDist
		if denom <= 0.1 {
			return camPos, 0, 0, false // Behind camera clipping
		}
		projFactor := camDist / denom
		screenX = int(math.Round(float64(halfW + (camPos.X * scale * projFactor))))
		screenY = int(math.Round(float64(halfH - (camPos.Y * scale * projFactor / c.AspectRatio))))
	} else {
		// Orthographic projection
		screenX = int(math.Round(float64(halfW + (camPos.X * scale))))
		screenY = int(math.Round(float64(halfH - (camPos.Y * scale / c.AspectRatio))))
	}

	visible = screenX >= 0 && screenX < c.Width && screenY >= 0 && screenY < c.Height
	return camPos, screenX, screenY, visible
}

// BrailleCanvas provides 2x4 subpixel drawing within terminal character cells.
type BrailleCanvas struct {
	Width  int // Cell columns
	Height int // Cell rows
	grid   [][]uint8
}

// Braille dot bitmask layout:
// dot 1 (0x01)  dot 4 (0x08)
// dot 2 (0x02)  dot 5 (0x10)
// dot 3 (0x04)  dot 6 (0x20)
// dot 7 (0x40)  dot 8 (0x80)
var brailleDotMasks = [4][2]uint8{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

func NewBrailleCanvas(w, h int) *BrailleCanvas {
	grid := make([][]uint8, h)
	for y := range grid {
		grid[y] = make([]uint8, w)
	}
	return &BrailleCanvas{
		Width:  w,
		Height: h,
		grid:   grid,
	}
}

func (bc *BrailleCanvas) PixelWidth() int {
	return bc.Width * 2
}

func (bc *BrailleCanvas) PixelHeight() int {
	return bc.Height * 4
}

func (bc *BrailleCanvas) SetPixel(px, py int) {
	if px < 0 || px >= bc.PixelWidth() || py < 0 || py >= bc.PixelHeight() {
		return
	}
	cellX := px / 2
	cellY := py / 4
	dotX := px % 2
	dotY := py % 4
	bc.grid[cellY][cellX] |= brailleDotMasks[dotY][dotX]
}

func (bc *BrailleCanvas) DrawLine(x0, y0, x1, y1 int) {
	dx := int(math.Abs(float64(x1 - x0)))
	dy := int(math.Abs(float64(y1 - y0)))
	sx := 1
	if x0 > x1 {
		sx = -1
	}
	sy := 1
	if y0 > y1 {
		sy = -1
	}
	err := dx - dy

	for {
		bc.SetPixel(x0, y0)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

func (bc *BrailleCanvas) DrawLine3D(cam *Camera3D, p0, p1 Vec3) {
	_, sx0, sy0, v0 := cam.Transform(p0)
	_, sx1, sy1, v1 := cam.Transform(p1)
	if !v0 && !v1 {
		return
	}
	// Subpixel coordinates (2x width, 4x height)
	px0 := sx0 * 2
	py0 := sy0 * 4
	px1 := sx1 * 2
	py1 := sy1 * 4
	bc.DrawLine(px0, py0, px1, py1)
}

func (bc *BrailleCanvas) RuneAt(x, y int) rune {
	if x < 0 || x >= bc.Width || y < 0 || y >= bc.Height {
		return ' '
	}
	mask := bc.grid[y][x]
	if mask == 0 {
		return ' '
	}
	return rune(0x2800 + uint32(mask))
}

// StarMapPoint represents a star mapped to screen space.
type StarMapPoint struct {
	Star      *models.Star
	WorldPos  Vec3
	CamPos    Vec3
	ScreenX   int
	ScreenY   int
	Visible   bool
	Glyph     rune
	Color     RGB
	IsHub     bool
	IsMyHub   bool
	HasLife   bool
	IsRoute   bool
	RouteStep int
}

// MapLayerOptions configures visual layers in the renderer.
type MapLayerOptions struct {
	ShowLife         bool
	ShowHubs         bool
	ShowExploredOnly bool
	ShowLabels       bool
	ShowGrid         bool
	ShowAxes         bool
	ShowRoute        bool
	HighlightStar    string
	SelectedStar     string
	RouteLegs        []*models.JourneyLeg
}

func DefaultMapLayerOptions() *MapLayerOptions {
	return &MapLayerOptions{
		ShowLife:         true,
		ShowHubs:         true,
		ShowExploredOnly: false,
		ShowLabels:       true,
		ShowGrid:         true,
		ShowAxes:         true,
		ShowRoute:        true,
	}
}

// CellLayer holds character and ANSI styling for each viewport cell.
type CellLayer struct {
	Rune     rune
	FgColor  RGB
	HasColor bool
	IsBold   bool
	DepthZ   float32
	Star     *StarMapPoint
}

// prepareGalaxyGrid computes the 3D projection, Braille canvas, and cell layers.
func prepareGalaxyGrid(cam *Camera3D, stars []*models.Star, opts *MapLayerOptions) ([][]CellLayer, []*StarMapPoint) {
	if opts == nil {
		opts = DefaultMapLayerOptions()
	}

	w, h := cam.Width, cam.Height
	canvas := NewBrailleCanvas(w, h)
	cells := make([][]CellLayer, h)
	for y := range cells {
		cells[y] = make([]CellLayer, w)
		for x := range cells[y] {
			cells[y][x] = CellLayer{Rune: ' ', DepthZ: 99999}
		}
	}

	// 1. Draw coordinate grid / axes if enabled
	if opts.ShowGrid {
		drawGalacticGrid(cam, canvas, opts)
	}

	// 2. Draw route legs with Braille lines if enabled
	var routeStars = make(map[string]int)
	if opts.ShowRoute && len(opts.RouteLegs) > 0 {
		for i, leg := range opts.RouteLegs {
			if leg.FromPosition != nil && leg.ToPosition != nil {
				p0 := Vec3{X: leg.FromPosition.X, Y: leg.FromPosition.Y, Z: leg.FromPosition.Z}
				p1 := Vec3{X: leg.ToPosition.X, Y: leg.ToPosition.Y, Z: leg.ToPosition.Z}
				canvas.DrawLine3D(cam, p0, p1)
				routeStars[leg.From] = i + 1
				routeStars[leg.To] = i + 2
			}
		}
	}

	// 3. Project all stars and determine visibility
	var mappedStars []*StarMapPoint
	for _, st := range stars {
		if opts.ShowExploredOnly && !st.Explored {
			continue
		}
		if st.Position == nil {
			continue
		}

		wPos := Vec3{X: st.Position.X, Y: st.Position.Y, Z: st.Position.Z}
		camPos, sx, sy, vis := cam.Transform(wPos)

		baseCol := GetSpectralColor(st.SpectralType)

		// Depth brightness scaling (dim stars in background)
		depthRatio := (camPos.Z + cam.Radius) / (cam.Radius * 2.0)
		dimFactor := float32(0.5 + 0.5*(1.0-depthRatio))
		if dimFactor < 0.3 {
			dimFactor = 0.3
		}
		if dimFactor > 1.0 {
			dimFactor = 1.0
		}
		starCol := baseCol.Dim(dimFactor)

		// Choose glyph based on attributes and depth
		var glyph rune
		if st.HasMyHub {
			glyph = '◆'
			starCol = RGB{R: 255, G: 85, B: 255} // Bright Magenta
		} else if st.HasLife {
			glyph = '✦'
			starCol = RGB{R: 85, G: 255, B: 85} // Bright Green
		} else if st.HasHub {
			glyph = '◇'
			starCol = RGB{R: 85, G: 255, B: 255} // Cyan
		} else if step, isR := routeStars[string(st.Designation)]; isR {
			glyph = '◉'
			starCol = RGB{R: 255, G: 230, B: 100}
			_ = step
		} else {
			// Depth glyphs
			if camPos.Z < -cam.Radius*0.3 {
				glyph = '★' // Close
			} else if camPos.Z < cam.Radius*0.2 {
				glyph = '*' // Mid
			} else {
				glyph = '·' // Far
			}
		}

		step, isRoute := routeStars[string(st.Designation)]
		mp := &StarMapPoint{
			Star:      st,
			WorldPos:  wPos,
			CamPos:    camPos,
			ScreenX:   sx,
			ScreenY:   sy,
			Visible:   vis,
			Glyph:     glyph,
			Color:     starCol,
			IsHub:     st.HasHub,
			IsMyHub:   st.HasMyHub,
			HasLife:   st.HasLife,
			IsRoute:   isRoute,
			RouteStep: step,
		}

		if vis {
			mappedStars = append(mappedStars, mp)
		}
	}

	// 4. Sort mapped stars by depth (painter's algorithm: far to near)
	slices.SortFunc(mappedStars, func(a, b *StarMapPoint) int {
		return cmp.Compare(b.CamPos.Z, a.CamPos.Z)
	})

	// 5. Transfer Braille canvas background to cell buffer
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r := canvas.RuneAt(x, y)
			if r != ' ' {
				cells[y][x] = CellLayer{
					Rune:     r,
					FgColor:  RGB{R: 60, G: 90, B: 120}, // Subdued grid/line color
					HasColor: true,
					DepthZ:   50000,
				}
			}
		}
	}

	// 6. Draw Star Glyphs over canvas
	for _, mp := range mappedStars {
		x, y := mp.ScreenX, mp.ScreenY
		if x >= 0 && x < w && y >= 0 && y < h {
			cells[y][x] = CellLayer{
				Rune:     mp.Glyph,
				FgColor:  mp.Color,
				HasColor: true,
				IsBold:   mp.IsMyHub || mp.HasLife || mp.IsRoute,
				DepthZ:   mp.CamPos.Z,
				Star:     mp,
			}
		}
	}

	// 7. Overlay Star Labels if enabled
	if opts.ShowLabels {
		for _, mp := range mappedStars {
			// Show names for prominent stars (Hubs, Life, Route, Selected, or closest stars)
			isProminent := mp.IsMyHub || mp.HasLife || mp.IsRoute ||
				string(mp.Star.Designation) == opts.SelectedStar ||
				string(mp.Star.Designation) == opts.HighlightStar ||
				mp.CamPos.Z < -cam.Radius*0.2

			if isProminent {
				displayName := mp.Star.Name
				if displayName == "" {
					displayName = string(mp.Star.Designation)
				}
				if len(displayName) > 12 {
					displayName = displayName[:12]
				}

				labelStartX := mp.ScreenX + 2
				labelY := mp.ScreenY
				if labelY >= 0 && labelY < h {
					for i, ch := range displayName {
						lx := labelStartX + i
						if lx < w && lx >= 0 {
							// Don't overwrite another star glyph
							if cells[labelY][lx].Star == nil {
								labelCol := mp.Color.Dim(0.85)
								cells[labelY][lx] = CellLayer{
									Rune:     ch,
									FgColor:  labelCol,
									HasColor: true,
									DepthZ:   mp.CamPos.Z,
								}
							}
						}
					}
				}
			}
		}
	}

	// 8. Draw Cursor / Selected Star Target box
	if opts.SelectedStar != "" {
		for _, mp := range mappedStars {
			if string(mp.Star.Designation) == opts.SelectedStar {
				sx, sy := mp.ScreenX, mp.ScreenY
				if sx > 0 && sx < w-1 && sy >= 0 && sy < h {
					cells[sy][sx-1] = CellLayer{Rune: '[', FgColor: RGB{R: 255, G: 255, B: 0}, HasColor: true, IsBold: true}
					cells[sy][sx+1] = CellLayer{Rune: ']', FgColor: RGB{R: 255, G: 255, B: 0}, HasColor: true, IsBold: true}
				}
			}
		}
	}

	return cells, mappedStars
}

// RenderGalaxyMap renders a complete ANSI string view of the galaxy for terminal output.
func RenderGalaxyMap(cam *Camera3D, stars []*models.Star, opts *MapLayerOptions) (string, []*StarMapPoint) {
	cells, mappedStars := prepareGalaxyGrid(cam, stars, opts)
	w, h := cam.Width, cam.Height

	var sb strings.Builder
	for y := 0; y < h; y++ {
		var curColor RGB
		var hasCurColor bool
		var curBold bool

		for x := 0; x < w; x++ {
			c := cells[y][x]
			if c.HasColor {
				if !hasCurColor || curColor != c.FgColor || curBold != c.IsBold {
					if c.IsBold {
						sb.WriteString("\x1b[1m")
					} else if curBold {
						sb.WriteString("\x1b[22m")
					}
					sb.WriteString(c.FgColor.ANSI())
					curColor = c.FgColor
					hasCurColor = true
					curBold = c.IsBold
				}
				sb.WriteRune(c.Rune)
			} else {
				if hasCurColor || curBold {
					sb.WriteString(ANSIReset)
					hasCurColor = false
					curBold = false
				}
				sb.WriteRune(c.Rune)
			}
		}
		if hasCurColor || curBold {
			sb.WriteString(ANSIReset)
		}
		if y < h-1 {
			sb.WriteByte('\n')
		}
	}

	return sb.String(), mappedStars
}

// RenderGalaxyMapTview renders the galaxy map formatted with tview color tags and escaped brackets.
func RenderGalaxyMapTview(cam *Camera3D, stars []*models.Star, opts *MapLayerOptions) (string, []*StarMapPoint) {
	cells, mappedStars := prepareGalaxyGrid(cam, stars, opts)
	w, h := cam.Width, cam.Height

	var sb strings.Builder
	for y := 0; y < h; y++ {
		var curColor RGB
		var hasCurColor bool
		var curBold bool

		for x := 0; x < w; x++ {
			c := cells[y][x]
			if c.HasColor {
				if !hasCurColor || curColor != c.FgColor || curBold != c.IsBold {
					if hasCurColor || curBold {
						sb.WriteString("[-::-]")
					}
					if c.IsBold {
						sb.WriteString(fmt.Sprintf("[#%02x%02x%02x::b]", c.FgColor.R, c.FgColor.G, c.FgColor.B))
					} else {
						sb.WriteString(fmt.Sprintf("[#%02x%02x%02x]", c.FgColor.R, c.FgColor.G, c.FgColor.B))
					}
					curColor = c.FgColor
					hasCurColor = true
					curBold = c.IsBold
				}
				if c.Rune == '[' {
					sb.WriteString("[[")
				} else if c.Rune == ']' {
					sb.WriteString("[]")
				} else {
					sb.WriteRune(c.Rune)
				}
			} else {
				if hasCurColor || curBold {
					sb.WriteString("[-::-]")
					hasCurColor = false
					curBold = false
				}
				if c.Rune == '[' {
					sb.WriteString("[[")
				} else if c.Rune == ']' {
					sb.WriteString("[]")
				} else {
					sb.WriteRune(c.Rune)
				}
			}
		}
		if hasCurColor || curBold {
			sb.WriteString("[-::-]")
		}
		if y < h-1 {
			sb.WriteByte('\n')
		}
	}

	return sb.String(), mappedStars
}

// StripANSI removes ANSI escape codes from a string.
func StripANSI(str string) string {
	var sb strings.Builder
	inEsc := false
	for _, r := range str {
		if r == 0x1b {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == 'm' {
				inEsc = false
			}
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// drawGalacticGrid draws reference axes and bounding plane rings.
func drawGalacticGrid(cam *Camera3D, bc *BrailleCanvas, opts *MapLayerOptions) {
	r := cam.Radius

	// Draw coordinate axes from center:
	// X axis: Green (East/West)
	// Y axis: Red (North/South / Galactic Up)
	// Z axis: Blue (Depth)
	if opts.ShowAxes {
		bc.DrawLine3D(cam, cam.Center.Sub(Vec3{X: r * 0.8}), cam.Center.Add(Vec3{X: r * 0.8}))
		bc.DrawLine3D(cam, cam.Center.Sub(Vec3{Y: r * 0.8}), cam.Center.Add(Vec3{Y: r * 0.8}))
		bc.DrawLine3D(cam, cam.Center.Sub(Vec3{Z: r * 0.8}), cam.Center.Add(Vec3{Z: r * 0.8}))
	}

	// Draw reference circle on the X-Z galactic plane
	const segments = 32
	var prev Vec3
	for i := 0; i <= segments; i++ {
		theta := float64(i) * 2.0 * math.Pi / float64(segments)
		pt := cam.Center.Add(Vec3{
			X: float32(math.Cos(theta)) * r * 0.7,
			Y: 0,
			Z: float32(math.Sin(theta)) * r * 0.7,
		})
		if i > 0 {
			bc.DrawLine3D(cam, prev, pt)
		}
		prev = pt
	}
}

// FormatMapHeader returns a styled HUD header with coordinates, zoom, and statistics.
func FormatMapHeader(cam *Camera3D, totalStars, visibleCount int, selectedStar *StarMapPoint) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\x1b[1;36m[GALAXY 3D MAP]\x1b[0m  Center: \x1b[33m%s\x1b[0m  Radius: \x1b[32m%.1fly\x1b[0m  Mode: \x1b[35m%s\x1b[0m  Stars: \x1b[37m%d visible\x1b[0m / %d total\n",
		cam.Center.String(), cam.Radius, cam.Mode, visibleCount, totalStars))

	if selectedStar != nil && selectedStar.Star != nil {
		st := selectedStar.Star
		name := st.Name
		if name == "" {
			name = "-"
		}
		lifeStr := "No"
		if st.HasLife {
			lifeStr = "\x1b[1;32mYES\x1b[0m"
		}
		hubStr := "No"
		if st.HasMyHub {
			hubStr = "\x1b[1;35mMY HUB\x1b[0m"
		} else if st.HasHub {
			hubStr = "\x1b[1;36mSystem Hub\x1b[0m"
		}

		sb.WriteString(fmt.Sprintf("\x1b[1;37mTarget:\x1b[0m \x1b[1;33m%s\x1b[0m (%s) | Spec: \x1b[36m%s\x1b[0m | Planets: \x1b[37m%d\x1b[0m | Life: %s | Hub: %s | Pos: %s\n",
			st.Designation, name, st.SpectralType, st.EstimatedPlanets, lifeStr, hubStr, st.Position.String()))
	}
	return sb.String()
}

// FormatMapLegend returns a color-coded legend explaining glyphs and spectral colors.
func FormatMapLegend() string {
	return "\x1b[90mLegend: \x1b[1;35m◆\x1b[0m My Hub  \x1b[1;32m✦\x1b[0m Life  \x1b[1;36m◇\x1b[0m Hub  \x1b[1;33m◉\x1b[0m Route  | Classes: \x1b[38;2;85;153;255mO\x1b[0m \x1b[38;2;136;204;255mB\x1b[0m \x1b[38;2;240;245;255mA\x1b[0m \x1b[38;2;255;244;204mF\x1b[0m \x1b[38;2;255;221;68mG\x1b[0m \x1b[38;2;255;153;51mK\x1b[0m \x1b[38;2;255;68;68mM\x1b[0m\x1b[0m"
}
