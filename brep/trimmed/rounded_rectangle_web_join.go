package trimmed

import (
	"context"
	"math"

	"plainsolid/geom"
)

// RoundedRectangleWebJoin retains the bottom cap as face 0 and the exposed
// foot cap as face 1. WebSides gives the lower-Y and higher-Y web faces.
type RoundedRectangleWebJoin struct {
	Solid
	WebTop   int
	WebSides [2]int
}

// JoinBottomChamferedRoundedRectangleWeb joins a full-width rectangular web
// at the high-Y side of bottom-chamfered stock. The web covers both rounded
// corners on that side, begins within the straight wall band, and extends
// above the stock. The original corner arcs bound two exposed underside
// faces at the tool's original entry plane.
func JoinBottomChamferedRoundedRectangleWeb(ctx context.Context, frame geom.Frame, profile RoundedRectangle,
	height, chamfer, webY, webBottom, webTop, tolerance float64) (RoundedRectangleWebJoin, error) {
	if ctx == nil || !finite(webY) || !finite(webBottom) || !finite(webTop) || !finite(height) || !finite(tolerance) || !prismCardinal(frame) {
		return RoundedRectangleWebJoin{}, geom.ErrInvalid
	}
	pad := 8 * tolerance
	radius := profile.Radii[0]
	if tolerance <= 0 || !finite(pad) || webBottom-chamfer <= pad || height-webBottom <= pad || webTop-height <= pad ||
		webY-(profile.Min.Y+radius) <= pad || profile.Max.Y-radius-webY <= pad {
		return RoundedRectangleWebJoin{}, geom.ErrDomain
	}
	stock, err := BottomChamferedRoundedRectangle(ctx, frame, profile, webBottom, chamfer, tolerance)
	if err != nil {
		return RoundedRectangleWebJoin{}, err
	}
	ring, arcs, err := roundedExtrusionProfile(profile)
	if err != nil {
		return RoundedRectangleWebJoin{}, err
	}
	b := cornerCylinderBuilder{ctx: ctx, frame: frame, vertices: stock.Vertices(), edges: stock.Edges(), faces: stock.Faces(),
		verticesByPoint: map[geom.Vec3]int{}, edgesByEnds: map[[2]int]int{}}
	for i, p := range b.vertices {
		b.verticesByPoint[p] = i
	}
	for i, e := range b.edges {
		b.edgesByEnds[[2]int{min(e.Start, e.End), max(e.Start, e.End)}] = i
	}

	left, right := profile.Min.X, profile.Max.X
	rear := profile.Max.Y
	rearRight, rearLeft := geom.Vec2{X: right, Y: rear}, geom.Vec2{X: left, Y: rear}
	frontRight, frontLeft := geom.Vec2{X: right, Y: webY}, geom.Vec2{X: left, Y: webY}
	// The exposed foot at the original stock height replaces the interior
	// cap used while constructing the lower walls and chamfer.
	foot := []geom.Vec2{ring[0], ring[1], ring[2], ring[3], frontRight, frontLeft}
	if err := addRoundedWebCap(&b, foot, []roundedWebArc{{0, arcs[0], false}, {2, arcs[2], false}}, height, false); err != nil {
		return RoundedRectangleWebJoin{}, err
	}
	b.faces[1] = b.faces[len(b.faces)-1]
	b.faces = b.faces[:len(b.faces)-1]

	for _, corner := range []struct {
		ring       []geom.Vec2
		sourceEdge int
	}{{[]geom.Vec2{ring[4], rearRight, ring[5]}, 4}, {[]geom.Vec2{ring[6], rearLeft, ring[7]}, 6}} {
		if err := addRoundedWebCap(&b, corner.ring, []roundedWebArc{{2, arcs[corner.sourceEdge], true}}, webBottom, true); err != nil {
			return RoundedRectangleWebJoin{}, err
		}
	}
	// The exposed lower corners and front edge of the foot continue
	// unchanged to the original top. The right, rear and left sides
	// acquire the rectangular web's corner fills over the overlap band.
	for i := range 3 {
		if arcs[i].circle.Radius == 0 {
			if err := b.wall(ring[i], ring[i+1], webBottom, height); err != nil {
				return RoundedRectangleWebJoin{}, err
			}
			continue
		}
		surface, uv, err := arcs[i].wall(frame, webBottom, height)
		if err != nil {
			return RoundedRectangleWebJoin{}, err
		}
		points := roundedWebBandPoints([]geom.Vec2{ring[i], ring[i+1]}, []geom.Vec2{ring[i], ring[i+1]}, webBottom, height)
		if err := b.face(surface, points, uv[:], nil, false); err != nil {
			return RoundedRectangleWebJoin{}, err
		}
	}
	for _, side := range []struct{ bottom, top []geom.Vec2 }{
		{[]geom.Vec2{ring[3], ring[4], rearRight}, []geom.Vec2{ring[3], frontRight, rearRight}},
		{[]geom.Vec2{rearRight, ring[5], ring[6], rearLeft}, []geom.Vec2{rearRight, rearLeft}},
		{[]geom.Vec2{rearLeft, ring[7], ring[0]}, []geom.Vec2{rearLeft, frontLeft, ring[0]}},
	} {
		if err := addRoundedWebWall(&b, side.bottom, side.top, webBottom, height, false); err != nil {
			return RoundedRectangleWebJoin{}, err
		}
	}
	web := []geom.Vec2{frontLeft, frontRight, rearRight, rearLeft}
	result := RoundedRectangleWebJoin{WebTop: len(b.faces)}
	if err := addRoundedWebCap(&b, web, nil, webTop, false); err != nil {
		return RoundedRectangleWebJoin{}, err
	}
	for i, p := range web {
		if i%2 == 0 {
			result.WebSides[i/2] = len(b.faces)
		}
		if i == 0 {
			// Both planar bore caps use the same signed cardinal chart.
			// Reverse the front face to give it its outward orientation.
			side := []geom.Vec2{frontRight, frontLeft}
			if err := addRoundedWebWall(&b, side, side, height, webTop, true); err != nil {
				return RoundedRectangleWebJoin{}, err
			}
			continue
		}
		if err := b.wall(p, web[(i+1)%len(web)], height, webTop); err != nil {
			return RoundedRectangleWebJoin{}, err
		}
	}
	result.Solid, err = NewSolidContext(ctx, b.vertices, b.edges, b.faces, tolerance)
	if err != nil {
		return RoundedRectangleWebJoin{}, err
	}
	return result, nil
}

type roundedWebArc struct {
	index   int
	source  extrusionQuarterArc
	reverse bool
}

func addRoundedWebCap(b *cornerCylinderBuilder, ring []geom.Vec2, arcs []roundedWebArc, z float64, reversed bool) error {
	frame, err := geom.NewFrame(b.frame.ToWorld(geom.Vec3{Z: z}), b.frame.XAxis(), b.frame.ZAxis())
	if err != nil {
		return err
	}
	plane, err := geom.NewPlaneSurface(frame)
	if err != nil {
		return err
	}
	world := make([]geom.Vec3, len(ring))
	for i, p := range ring {
		world[i] = geom.Vec3{X: p.X, Y: p.Y, Z: z}
	}
	curves := make(map[int]PCurve, len(arcs))
	for _, a := range arcs {
		q, radius := a.source.quadrant, a.source.circle.Radius
		sine := radius
		if a.reverse {
			q, sine = 3-q, -sine
		}
		curves[a.index], err = newAnchoredCircularUV(EllipseUV{Center: a.source.circle.Center,
			Cos: geom.Vec2{X: radius}, Sin: geom.Vec2{Y: sine}, From: float64(q) * math.Pi / 2, To: float64(q+1) * math.Pi / 2},
			ring[a.index], ring[(a.index+1)%len(ring)])
		if err != nil {
			return err
		}
	}
	return b.face(plane, world, ring, curves, reversed)
}

func roundedWebBandPoints(bottom, top []geom.Vec2, lo, hi float64) []geom.Vec3 {
	points := make([]geom.Vec3, 0, len(bottom)+len(top))
	for _, p := range bottom {
		points = append(points, geom.Vec3{X: p.X, Y: p.Y, Z: lo})
	}
	for i := len(top) - 1; i >= 0; i-- {
		points = append(points, geom.Vec3{X: top[i].X, Y: top[i].Y, Z: hi})
	}
	return points
}

func addRoundedWebWall(b *cornerCylinderBuilder, bottom, top []geom.Vec2, lo, hi float64, reversed bool) error {
	points := roundedWebBandPoints(bottom, top, lo, hi)
	origin := b.frame.ToWorld(points[0])
	tangent := b.frame.ToWorld(points[1]).Sub(origin)
	frame, err := geom.NewFrame(origin, tangent, tangent.Cross(b.frame.ZAxis()))
	if err != nil {
		return err
	}
	plane, err := geom.NewPlaneSurface(frame)
	if err != nil {
		return err
	}
	uv := make([]geom.Vec2, len(points))
	for i, p := range points {
		q := frame.ToLocal(b.frame.ToWorld(p))
		uv[i] = geom.Vec2{X: q.X, Y: q.Y}
	}
	return b.face(plane, points, uv, nil, reversed)
}
