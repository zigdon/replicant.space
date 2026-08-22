package common

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

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

// Known region color palette
var knownRegionColors = map[string]RGB{
	"":        {R: 100, G: 100, B: 100}, // Dark Grey
	"solzone": {R: 0, G: 229, B: 255},   // Bright Cyan
	"alpha":   {R: 118, G: 255, B: 3},   // Neon Green
	"beta":    {R: 255, G: 145, B: 0},   // Warm Amber / Orange
	"gamma":   {R: 224, G: 64, B: 251},  // Vibrant Violet
}

func HSVtoRGB(h, s, v float64) RGB {
	c := v * s
	x := c * (1.0 - math.Abs(math.Mod(h/60.0, 2.0)-1.0))
	m := v - c
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return RGB{
		R: uint8(math.Round((r + m) * 255)),
		G: uint8(math.Round((g + m) * 255)),
		B: uint8(math.Round((b + m) * 255)),
	}
}

func GetRegionColor(region string) RGB {
	if region == "" {
		return RGB{R: 160, G: 160, B: 160}
	}
	rLower := strings.ToLower(strings.TrimSpace(region))
	if col, ok := knownRegionColors[rLower]; ok {
		return col
	}
	// Deterministic color generation based on FNV-1a hash
	h := uint32(2166136261)
	for i := 0; i < len(rLower); i++ {
		h ^= uint32(rLower[i])
		h *= 16777619
	}
	hue := float64(h % 360)
	return HSVtoRGB(hue, 0.85, 1.0)
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

// DeviceLocationInfo stores summary info of a device at a location.
type DeviceLocationInfo struct {
	Code     string
	Type     string
	Location string
	Status   string
}

// NetworkDevice represents a single relaying device in a star system.
type NetworkDevice struct {
	Code     string
	Type     string
	Location string
	Status   string
	RangeLy  float32
}

// NetworkLink represents an FTL connection between two network nodes.
type NetworkLink struct {
	FromStar      string
	ToStar        string
	FromPos       Vec3
	ToPos         Vec3
	Distance      float32
	IsFromReach   bool // True if From can reach To (dist <= Range(From))
	IsToReach     bool // True if To can reach From (dist <= Range(To))
	Bidirectional bool // True if both can reach each other
}

// NetworkNode represents a star system hosting one or more relaying network devices.
type NetworkNode struct {
	Star        string
	Position    Vec3
	MaxRange    float32
	Devices     []*NetworkDevice
	SubnetID    int
	Connections []*NetworkLink
}

// NetworkGraph represents the entire FTL communication grid.
type NetworkGraph struct {
	Nodes   map[string]*NetworkNode // Key: star designation
	Links   []*NetworkLink
	Subnets map[int][]*NetworkNode // Subnet ID -> list of nodes
}

// BuildNetworkGraph constructs an FTL communication graph from relaying devices and star positions.
// Ranges: ftl_relay=7.5ly, deep_space_relay_station=10.0ly, system_hub=15.0ly.
// Connections are asymmetric: link exists if distance <= max(Range(A), Range(B)).
func BuildNetworkGraph(devices []*NetworkDevice, starLookup map[string]Vec3) *NetworkGraph {
	g := &NetworkGraph{
		Nodes:   make(map[string]*NetworkNode),
		Subnets: make(map[int][]*NetworkNode),
	}

	// 1. Group devices by star designation
	for _, d := range devices {
		if d == nil {
			continue
		}
		starName := strings.ToUpper(strings.TrimSpace(models.LocationID(d.Location).Star()))
		pos, hasPos := starLookup[starName]
		if !hasPos {
			continue
		}

		node, exists := g.Nodes[starName]
		if !exists {
			node = &NetworkNode{
				Star:     starName,
				Position: pos,
				MaxRange: d.RangeLy,
			}
			g.Nodes[starName] = node
		}
		node.Devices = append(node.Devices, d)
		if d.RangeLy > node.MaxRange {
			node.MaxRange = d.RangeLy
		}
	}

	// 2. Discover links between all pairs of nodes
	nodeList := make([]*NetworkNode, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		nodeList = append(nodeList, n)
	}

	for i := 0; i < len(nodeList); i++ {
		for j := i + 1; j < len(nodeList); j++ {
			n1 := nodeList[i]
			n2 := nodeList[j]

			dist := n1.Position.Distance(n2.Position)
			can1Reach2 := dist <= n1.MaxRange
			can2Reach1 := dist <= n2.MaxRange

			if can1Reach2 || can2Reach1 {
				link := &NetworkLink{
					FromStar:      n1.Star,
					ToStar:        n2.Star,
					FromPos:       n1.Position,
					ToPos:         n2.Position,
					Distance:      dist,
					IsFromReach:   can1Reach2,
					IsToReach:     can2Reach1,
					Bidirectional: can1Reach2 && can2Reach1,
				}
				g.Links = append(g.Links, link)
				n1.Connections = append(n1.Connections, link)
				n2.Connections = append(n2.Connections, link)
			}
		}
	}

	// 3. Compute Connected Components (Subnets) via BFS
	visited := make(map[string]bool)
	subnetCounter := 1

	for _, n := range nodeList {
		if visited[n.Star] {
			continue
		}

		var component []*NetworkNode
		queue := []*NetworkNode{n}
		visited[n.Star] = true

		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			curr.SubnetID = subnetCounter
			component = append(component, curr)

			for _, link := range curr.Connections {
				neighborStar := link.ToStar
				if neighborStar == curr.Star {
					neighborStar = link.FromStar
				}
				if !visited[neighborStar] {
					visited[neighborStar] = true
					if neighborNode, ok := g.Nodes[neighborStar]; ok {
						queue = append(queue, neighborNode)
					}
				}
			}
		}

		g.Subnets[subnetCounter] = component
		subnetCounter++
	}

	return g
}

// TravellingDevice represents a device currently in transit between locations.
type TravellingDevice struct {
	Code            string
	Alias           string
	Type            string
	Origin          string
	OriginPos       Vec3
	Destination     string
	DestinationPos  Vec3
	ProgressPercent float32
	Eta             string
	Status          string
	Departed        time.Time
	Arrives         time.Time
	EstimatedPos    Vec3
	RouteLegs       []*models.JourneyLeg
	ActiveLegIndex  int
}

// TravelFilterOptions specifies criteria to filter travelling devices.
type TravelFilterOptions struct {
	Devices     []string // Specific device codes or aliases
	DeviceTypes []string // Specific device types or prefixes
	SourceStars []string // Specific source/origin star designations
	DestStars   []string // Specific destination star designations
}

func (f *TravelFilterOptions) Matches(d *models.Device) bool {
	if f == nil {
		return true
	}
	if len(f.Devices) > 0 {
		match := false
		code := strings.ToLower(d.Code.String())
		alias := strings.ToLower(d.Code.Alias())
		for _, dev := range f.Devices {
			dev = strings.ToLower(strings.TrimSpace(dev))
			if dev != "" && (code == dev || alias == dev || strings.EqualFold(d.Code.String(), dev) || strings.EqualFold(d.Code.Alias(), dev)) {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	if len(f.DeviceTypes) > 0 {
		match := false
		dType := strings.ToLower(d.Type)
		for _, t := range f.DeviceTypes {
			t = strings.ToLower(strings.TrimSpace(t))
			if t != "" && (dType == t || strings.HasPrefix(dType, t) || strings.EqualFold(d.Type, t)) {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	if len(f.SourceStars) > 0 {
		match := false
		origStar := ""
		if d.Travel != nil && d.Travel.Origin != "" {
			origStar = strings.ToUpper(strings.TrimSpace(d.Travel.Origin.Star()))
		} else if d.Location != "" {
			origStar = strings.ToUpper(strings.TrimSpace(d.Location.Star()))
		}
		for _, s := range f.SourceStars {
			s = strings.ToUpper(strings.TrimSpace(models.LocationID(s).Star()))
			if s != "" && origStar == s {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	if len(f.DestStars) > 0 {
		match := false
		destStar := ""
		if d.Travel != nil && d.Travel.Destination != "" {
			destStar = strings.ToUpper(strings.TrimSpace(d.Travel.Destination.Star()))
		}
		for _, s := range f.DestStars {
			s = strings.ToUpper(strings.TrimSpace(models.LocationID(s).Star()))
			if s != "" && destStar == s {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	return true
}

// BuildTravellingDevices parses and filters travelling devices, computing route legs and estimated 3D positions.
func BuildTravellingDevices(devices []*models.Device, starLookup map[string]Vec3, filter *TravelFilterOptions) []*TravellingDevice {
	var result []*TravellingDevice
	now := time.Now()

	for _, d := range devices {
		if d == nil || d.Travel == nil {
			continue
		}
		if d.Travel.Destination == "" && len(d.Travel.Route) == 0 {
			continue
		}
		if filter != nil && !filter.Matches(d) {
			continue
		}

		origStar := strings.ToUpper(strings.TrimSpace(d.Travel.Origin.Star()))
		if origStar == "" && d.Location != "" {
			origStar = strings.ToUpper(strings.TrimSpace(d.Location.Star()))
		}
		destStar := strings.ToUpper(strings.TrimSpace(d.Travel.Destination.Star()))

		origPos, hasOrigPos := starLookup[origStar]
		destPos, hasDestPos := starLookup[destStar]

		var routeLegs []*models.JourneyLeg
		if len(d.Travel.Route) > 0 {
			for i, rLeg := range d.Travel.Route {
				fromStar := strings.ToUpper(strings.TrimSpace(rLeg.From.Star()))
				toStar := strings.ToUpper(strings.TrimSpace(rLeg.To.Star()))
				fPos, hasF := starLookup[fromStar]
				tPos, hasT := starLookup[toStar]
				var fromModelPos, toModelPos *models.Position
				if hasF {
					fromModelPos = &models.Position{X: fPos.X, Y: fPos.Y, Z: fPos.Z}
				}
				if hasT {
					toModelPos = &models.Position{X: tPos.X, Y: tPos.Y, Z: tPos.Z}
				}
				dist := rLeg.DistanceLy
				if dist == 0 && hasF && hasT {
					dist = fPos.Distance(tPos)
				}
				jl := &models.JourneyLeg{
					From:         fromStar,
					FromPosition: fromModelPos,
					To:           toStar,
					ToPosition:   toModelPos,
					DistFromSrc:  dist,
					Step:         i + 1,
				}
				routeLegs = append(routeLegs, jl)
			}
		} else if origStar != "" && destStar != "" && origStar != destStar {
			var fromModelPos, toModelPos *models.Position
			if hasOrigPos {
				fromModelPos = &models.Position{X: origPos.X, Y: origPos.Y, Z: origPos.Z}
			}
			if hasDestPos {
				toModelPos = &models.Position{X: destPos.X, Y: destPos.Y, Z: destPos.Z}
			}
			routeLegs = append(routeLegs, &models.JourneyLeg{
				From:         origStar,
				FromPosition: fromModelPos,
				To:           destStar,
				ToPosition:   toModelPos,
				DistFromSrc:  origPos.Distance(destPos),
				Step:         1,
			})
		}

		var f float32
		if d.Travel.ProgressPercent > 0 {
			if d.Travel.ProgressPercent > 1.0 {
				f = d.Travel.ProgressPercent / 100.0
			} else {
				f = d.Travel.ProgressPercent
			}
		}
		var depTime, arrTime time.Time
		if d.Travel.Departed != nil {
			depTime = d.Travel.Departed.Time()
		}
		if d.Travel.Arrives != nil {
			arrTime = d.Travel.Arrives.Time()
		}
		if f == 0 && !depTime.IsZero() && !arrTime.IsZero() && arrTime.After(depTime) {
			if now.After(arrTime) {
				f = 1.0
			} else if now.Before(depTime) {
				f = 0.0
			} else {
				f = float32(now.Sub(depTime)) / float32(arrTime.Sub(depTime))
			}
		}
		if f < 0 {
			f = 0
		}
		if f > 1 {
			f = 1
		}

		estPos := origPos
		activeLegIdx := 0

		if len(routeLegs) > 0 {
			var waypoints []Vec3
			for i, leg := range routeLegs {
				if leg.FromPosition != nil {
					pFrom := Vec3{X: leg.FromPosition.X, Y: leg.FromPosition.Y, Z: leg.FromPosition.Z}
					if i == 0 || len(waypoints) == 0 {
						waypoints = append(waypoints, pFrom)
					}
				}
				if leg.ToPosition != nil {
					pTo := Vec3{X: leg.ToPosition.X, Y: leg.ToPosition.Y, Z: leg.ToPosition.Z}
					waypoints = append(waypoints, pTo)
				}
			}

			if len(waypoints) >= 2 {
				var totalDist float32
				for i := 0; i < len(waypoints)-1; i++ {
					totalDist += waypoints[i].Distance(waypoints[i+1])
				}

				if totalDist > 0 {
					targetDist := f * totalDist
					var cumDist float32
					estPos = waypoints[len(waypoints)-1]
					activeLegIdx = len(waypoints) - 2

					for i := 0; i < len(waypoints)-1; i++ {
						segLen := waypoints[i].Distance(waypoints[i+1])
						if segLen > 0 && cumDist+segLen >= targetDist {
							segFraction := (targetDist - cumDist) / segLen
							if segFraction < 0 {
								segFraction = 0
							}
							if segFraction > 1 {
								segFraction = 1
							}
							estPos = waypoints[i].Add(waypoints[i+1].Sub(waypoints[i]).Mul(segFraction))
							activeLegIdx = i
							break
						}
						cumDist += segLen
					}
				} else {
					estPos = waypoints[0]
				}
			} else if hasOrigPos && hasDestPos {
				estPos = origPos.Add(destPos.Sub(origPos).Mul(f))
			} else if hasOrigPos {
				estPos = origPos
			} else if hasDestPos {
				estPos = destPos
			}
		} else if hasOrigPos && hasDestPos {
			estPos = origPos.Add(destPos.Sub(origPos).Mul(f))
		}

		codeStr := d.Code.String()
		aliasStr := d.Code.Alias()
		if codeStr == "" {
			codeStr = aliasStr
		}
		if aliasStr == "" {
			aliasStr = codeStr
		}

		etaStr := ""
		if d.Travel.Eta != nil {
			etaStr = d.Travel.Eta.String()
		}

		td := &TravellingDevice{
			Code:            codeStr,
			Alias:           aliasStr,
			Type:            d.Type,
			Origin:          origStar,
			OriginPos:       origPos,
			Destination:     destStar,
			DestinationPos:  destPos,
			ProgressPercent: f * 100.0,
			Eta:             etaStr,
			Status:          d.Status,
			Departed:        depTime,
			Arrives:         arrTime,
			EstimatedPos:    estPos,
			RouteLegs:       routeLegs,
			ActiveLegIndex:  activeLegIdx,
		}

		result = append(result, td)
	}

	return result
}

// TravellingDeviceMapPoint represents a travelling device projected onto screen space.
type TravellingDeviceMapPoint struct {
	Device   *TravellingDevice
	WorldPos Vec3
	CamPos   Vec3
	ScreenX  int
	ScreenY  int
	Visible  bool
	Glyph    rune
	Color    RGB
}

// StarMapPoint represents a star mapped to screen space.
type StarMapPoint struct {
	Star        *models.Star
	WorldPos    Vec3
	CamPos      Vec3
	ScreenX     int
	ScreenY     int
	Visible     bool
	Glyph       rune
	Color       RGB
	IsHub       bool
	IsMyHub     bool
	HasLife     bool
	IsRoute     bool
	RouteStep   int
	Devices     []*DeviceLocationInfo
	NetworkNode *NetworkNode
	Travelling  []*TravellingDevice
}

// MapLayerOptions configures visual layers in the renderer.
type MapLayerOptions struct {
	FilterLifeOnly       bool
	FilterHubsOnly       bool
	FilterExploredOnly   bool
	FilterRegion         string
	FilterDevicesOnly    bool
	FilterNetworkOnly    bool
	FilterTravelOnly     bool
	ShowRegions          bool
	ShowDevices          bool
	ShowNetwork          bool
	ShowTravel           bool
	ShowLabels           bool
	ShowGrid             bool
	ShowAxes             bool
	ShowRoute            bool
	HighlightStar        string
	SelectedStar         string
	SelectedTravelDevice string
	RouteLegs            []*models.JourneyLeg
	DeviceTypes          []string
	StarDevices          map[string][]*DeviceLocationInfo
	Network              *NetworkGraph
	TravellingDevices    []*TravellingDevice
	TravelFilter         *TravelFilterOptions
	MappedTravelling     []*TravellingDeviceMapPoint
}

func DefaultMapLayerOptions() *MapLayerOptions {
	return &MapLayerOptions{
		FilterLifeOnly:     false,
		FilterHubsOnly:     false,
		FilterExploredOnly: false,
		FilterRegion:       "",
		FilterDevicesOnly:  false,
		FilterNetworkOnly:  false,
		FilterTravelOnly:   false,
		ShowRegions:        false,
		ShowDevices:        false,
		ShowNetwork:        false,
		ShowTravel:         false,
		ShowLabels:         true,
		ShowGrid:           true,
		ShowAxes:           true,
		ShowRoute:          true,
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
	Travel   *TravellingDeviceMapPoint
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

	// 2. Draw route legs, network links, and travel routes with Braille lines if enabled
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
	if opts.ShowNetwork && opts.Network != nil && len(opts.Network.Links) > 0 {
		for _, link := range opts.Network.Links {
			canvas.DrawLine3D(cam, link.FromPos, link.ToPos)
		}
	}
	if opts.ShowTravel && len(opts.TravellingDevices) > 0 {
		for _, td := range opts.TravellingDevices {
			if len(td.RouteLegs) > 0 {
				for _, leg := range td.RouteLegs {
					if leg.FromPosition != nil && leg.ToPosition != nil {
						p0 := Vec3{X: leg.FromPosition.X, Y: leg.FromPosition.Y, Z: leg.FromPosition.Z}
						p1 := Vec3{X: leg.ToPosition.X, Y: leg.ToPosition.Y, Z: leg.ToPosition.Z}
						canvas.DrawLine3D(cam, p0, p1)
					}
				}
			} else if td.Origin != "" && td.Destination != "" {
				canvas.DrawLine3D(cam, td.OriginPos, td.DestinationPos)
			}
		}
	}

	// Index travelling devices by star for StarMapPoint
	var starTravelling map[string][]*TravellingDevice
	var travelStars map[string]bool
	if len(opts.TravellingDevices) > 0 {
		starTravelling = make(map[string][]*TravellingDevice)
		travelStars = make(map[string]bool)
		for _, td := range opts.TravellingDevices {
			if td.Origin != "" {
				starTravelling[td.Origin] = append(starTravelling[td.Origin], td)
				travelStars[td.Origin] = true
			}
			if td.Destination != "" {
				starTravelling[td.Destination] = append(starTravelling[td.Destination], td)
				travelStars[td.Destination] = true
			}
			for _, leg := range td.RouteLegs {
				travelStars[leg.From] = true
				travelStars[leg.To] = true
			}
		}
	}

	// 3. Project all stars and determine visibility
	var mappedStars []*StarMapPoint
	for _, st := range stars {
		var starDevs []*DeviceLocationInfo
		if opts.StarDevices != nil {
			starDevs = opts.StarDevices[string(st.Designation)]
		}

		var netNode *NetworkNode
		if opts.Network != nil {
			netNode = opts.Network.Nodes[string(st.Designation)]
		}

		if opts.FilterExploredOnly && !st.Explored {
			continue
		}
		if opts.FilterLifeOnly && !st.HasLife {
			continue
		}
		if opts.FilterHubsOnly && (!st.HasHub && !st.HasMyHub) {
			continue
		}
		if opts.FilterRegion != "" && !strings.EqualFold(st.Region, opts.FilterRegion) {
			continue
		}
		if opts.FilterDevicesOnly && len(starDevs) == 0 {
			continue
		}
		if opts.FilterNetworkOnly && netNode == nil {
			continue
		}
		if opts.FilterTravelOnly && travelStars != nil && !travelStars[string(st.Designation)] {
			continue
		}
		if st.Position == nil {
			continue
		}

		wPos := Vec3{X: st.Position.X, Y: st.Position.Y, Z: st.Position.Z}
		camPos, sx, sy, vis := cam.Transform(wPos)

		var baseCol RGB
		if opts.ShowRegions {
			baseCol = GetRegionColor(st.Region)
		} else {
			baseCol = GetSpectralColor(st.SpectralType)
		}

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

		// Choose glyph based on attributes, network, devices, and depth
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
		} else if opts.ShowNetwork && netNode != nil {
			glyph = '◈'                         // Diamond glyph for active relay network node
			starCol = RGB{R: 0, G: 229, B: 255} // Neon Cyan
		} else if opts.ShowDevices && len(starDevs) > 0 {
			glyph = '⬢'                         // Solid Hexagon for device host
			starCol = RGB{R: 255, G: 215, B: 0} // Gold/Amber
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

		var trDevs []*TravellingDevice
		if starTravelling != nil {
			trDevs = starTravelling[string(st.Designation)]
		}

		step, isRoute := routeStars[string(st.Designation)]
		mp := &StarMapPoint{
			Star:        st,
			WorldPos:    wPos,
			CamPos:      camPos,
			ScreenX:     sx,
			ScreenY:     sy,
			Visible:     vis,
			Glyph:       glyph,
			Color:       starCol,
			IsHub:       st.HasHub,
			IsMyHub:     st.HasMyHub,
			HasLife:     st.HasLife,
			IsRoute:     isRoute,
			RouteStep:   step,
			Devices:     starDevs,
			NetworkNode: netNode,
			Travelling:  trDevs,
		}

		if vis {
			mappedStars = append(mappedStars, mp)
		}
	}

	// 4. Sort mapped stars by depth (painter's algorithm: far to near)
	slices.SortFunc(mappedStars, func(a, b *StarMapPoint) int {
		return cmp.Compare(b.CamPos.Z, a.CamPos.Z)
	})

	// Project travelling device positions
	var mappedTravelling []*TravellingDeviceMapPoint
	if opts.ShowTravel && len(opts.TravellingDevices) > 0 {
		for _, td := range opts.TravellingDevices {
			camPos, sx, sy, vis := cam.Transform(td.EstimatedPos)
			mp := &TravellingDeviceMapPoint{
				Device:   td,
				WorldPos: td.EstimatedPos,
				CamPos:   camPos,
				ScreenX:  sx,
				ScreenY:  sy,
				Visible:  vis,
				Glyph:    '▲',
				Color:    RGB{R: 255, G: 190, B: 30}, // Bright Amber/Gold
			}
			if vis {
				mappedTravelling = append(mappedTravelling, mp)
			}
		}
	}
	opts.MappedTravelling = mappedTravelling

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
			hasDevices := len(mp.Devices) > 0
			inNet := opts.ShowNetwork && mp.NetworkNode != nil
			cells[y][x] = CellLayer{
				Rune:     mp.Glyph,
				FgColor:  mp.Color,
				HasColor: true,
				IsBold:   mp.IsMyHub || mp.HasLife || mp.IsRoute || (opts.ShowDevices && hasDevices) || inNet,
				DepthZ:   mp.CamPos.Z,
				Star:     mp,
			}
		}
	}

	// 6b. Draw Travelling Device Markers over canvas
	if opts.ShowTravel && len(mappedTravelling) > 0 {
		for _, mp := range mappedTravelling {
			x, y := mp.ScreenX, mp.ScreenY
			if x >= 0 && x < w && y >= 0 && y < h {
				cells[y][x] = CellLayer{
					Rune:     mp.Glyph,
					FgColor:  mp.Color,
					HasColor: true,
					IsBold:   true,
					DepthZ:   mp.CamPos.Z,
					Travel:   mp,
				}
			}
		}
	}

	// 7. Overlay Star Labels if enabled
	if opts.ShowLabels {
		for _, mp := range mappedStars {
			hasDevices := len(mp.Devices) > 0
			inNet := opts.ShowNetwork && mp.NetworkNode != nil
			// Show names for prominent stars (Hubs, Life, Route, Device Hosts, Network Nodes, Selected, or closest stars)
			isProminent := mp.IsMyHub || mp.HasLife || mp.IsRoute ||
				(opts.ShowDevices && hasDevices) ||
				inNet ||
				string(mp.Star.Designation) == opts.SelectedStar ||
				string(mp.Star.Designation) == opts.HighlightStar ||
				mp.CamPos.Z < -cam.Radius*0.2

			if isProminent {
				displayName := mp.Star.Name
				if displayName == "" {
					displayName = string(mp.Star.Designation)
				}
				if opts.ShowRegions && mp.Star.Region != "" {
					displayName = fmt.Sprintf("%s (%s)", displayName, mp.Star.Region)
				}
				if opts.ShowDevices && hasDevices {
					displayName = fmt.Sprintf("%s [%d dev]", displayName, len(mp.Devices))
				}
				if inNet {
					displayName = fmt.Sprintf("%s [Net#%d]", displayName, mp.NetworkNode.SubnetID)
				}
				if len(displayName) > 18 {
					displayName = displayName[:18]
				}

				labelStartX := mp.ScreenX + 2
				labelY := mp.ScreenY
				if labelY >= 0 && labelY < h {
					for i, ch := range displayName {
						lx := labelStartX + i
						if lx < w && lx >= 0 {
							// Don't overwrite another star glyph or travel marker
							if cells[labelY][lx].Star == nil && cells[labelY][lx].Travel == nil {
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

		// Overlay Travelling Device Labels
		if opts.ShowTravel && len(mappedTravelling) > 0 {
			for _, mp := range mappedTravelling {
				label := fmt.Sprintf("%s (%.0f%%)", mp.Device.Alias, mp.Device.ProgressPercent)
				if len(label) > 16 {
					label = label[:16]
				}
				labelStartX := mp.ScreenX + 2
				labelY := mp.ScreenY
				if labelY >= 0 && labelY < h {
					for i, ch := range label {
						lx := labelStartX + i
						if lx < w && lx >= 0 {
							if cells[labelY][lx].Star == nil && cells[labelY][lx].Travel == nil {
								cells[labelY][lx] = CellLayer{
									Rune:     ch,
									FgColor:  mp.Color.Dim(0.85),
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

	// 8. Draw Cursor / Selected Star or Travel Device Target box
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
	if opts.SelectedTravelDevice != "" {
		for _, mp := range mappedTravelling {
			if strings.EqualFold(mp.Device.Code, opts.SelectedTravelDevice) || strings.EqualFold(mp.Device.Alias, opts.SelectedTravelDevice) {
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

// FormatMapLegend returns a color-coded legend explaining glyphs, spectral colors, or region colors.
func FormatMapLegend(opts *MapLayerOptions) string {
	var parts []string
	parts = append(parts, "\x1b[1;35m◆\x1b[0m My Hub", "\x1b[1;32m✦\x1b[0m Life", "\x1b[1;36m◇\x1b[0m Hub", "\x1b[1;33m◉\x1b[0m Route")
	if opts != nil && (opts.ShowDevices || len(opts.StarDevices) > 0) {
		parts = append(parts, "\x1b[1;33m⬢\x1b[0m Device")
	}
	if opts != nil && (opts.ShowNetwork || opts.Network != nil) {
		parts = append(parts, "\x1b[1;36m◈\x1b[0m Relay Net")
	}
	if opts != nil && (opts.ShowTravel || len(opts.TravellingDevices) > 0) {
		parts = append(parts, "\x1b[1;33m▲\x1b[0m Travelling")
	}
	base := "\x1b[90mLegend: \x1b[0m" + strings.Join(parts, "  ")
	if opts != nil && opts.ShowRegions {
		return base + "  | \x1b[1;37mRegions:\x1b[0m \x1b[38;2;0;229;255mSolzone\x1b[0m \x1b[38;2;118;255;3mAlpha\x1b[0m \x1b[38;2;255;145;0mBeta\x1b[0m \x1b[38;2;224;64;251mGamma\x1b[0m\x1b[0m"
	}
	return base + "  | Classes: \x1b[38;2;85;153;255mO\x1b[0m \x1b[38;2;136;204;255mB\x1b[0m \x1b[38;2;240;245;255mA\x1b[0m \x1b[38;2;255;244;204mF\x1b[0m \x1b[38;2;255;221;68mG\x1b[0m \x1b[38;2;255;153;51mK\x1b[0m \x1b[38;2;255;68;68mM\x1b[0m\x1b[0m"
}
