package trimmed

import (
	"context"
	"math"

	"plainsolid/geom"
)

// ExtrudeCircularSegment sweeps the region bounded by one circular arc and
// its closing chord. The signed arc and height retain the selected region
// and extrusion direction. Each curved side owns an analytic cylinder.
func ExtrudeCircularSegment(ctx context.Context, frame geom.Frame, center geom.Vec2, radius, start, sweep, height float64) (Solid, error) {
	if ctx == nil || !uvFinite(center) || !finite(radius) || radius <= 0 || !finite(start) ||
		!finite(sweep) || sweep == 0 || math.Abs(sweep) >= 2*math.Pi || !finite(height) || height == 0 {
		return Solid{}, geom.ErrInvalid
	}
	start = math.Remainder(start, 2*math.Pi)
	if sweep < 0 {
		start += sweep
		sweep = -sweep
	}
	tol := max(radius, math.Abs(height)) * 1e-11
	if min(sweep, 2*math.Pi-sweep)*radius <= 8*tol {
		return Solid{}, geom.ErrUnrepresentable
	}
	f, err := geom.NewFrame(frame.ToWorld(geom.Vec3{X: center.X, Y: center.Y}), frame.XAxis(), frame.ZAxis())
	if err != nil {
		return Solid{}, err
	}
	b := cornerCylinderBuilder{ctx: ctx, frame: f, edgesByEnds: map[[2]int]int{}, verticesByPoint: map[geom.Vec3]int{}}
	n := max(2, int(math.Ceil(sweep/(math.Pi/2)))+1)
	points := make([]geom.Vec2, n+1)
	angles := make([]float64, n+1)
	curves := map[int]PCurve{}
	for i := range points {
		angles[i] = start + sweep*(float64(i)/float64(n))
		s, c := math.Sincos(angles[i])
		points[i] = geom.Vec2{X: radius * c, Y: radius * s}
		if i > 0 {
			curves[i-1] = EllipseUV{Cos: geom.Vec2{X: radius}, Sin: geom.Vec2{Y: radius}, From: angles[i-1], To: angles[i]}
		}
	}
	lo, hi := min(0, height), max(0, height)
	for i, z := range []float64{lo, hi} {
		cap, err := geom.NewFrame(f.ToWorld(geom.Vec3{Z: z}), f.XAxis(), f.ZAxis())
		if err != nil {
			return Solid{}, err
		}
		plane, err := geom.NewPlaneSurface(cap)
		if err != nil {
			return Solid{}, err
		}
		rim := make([]geom.Vec3, len(points))
		for j, p := range points {
			rim[j] = geom.Vec3{X: p.X, Y: p.Y, Z: z}
		}
		if err := b.face(plane, rim, points, curves, i == 0); err != nil {
			return Solid{}, err
		}
	}
	for i := range n {
		if err := b.cylinderBand(angles[i], angles[i+1], lo, hi, radius, false, points[i], points[i+1]); err != nil {
			return Solid{}, err
		}
	}
	if err := b.wall(points[n], points[0], lo, hi); err != nil {
		return Solid{}, err
	}
	return NewSolidContext(ctx, b.vertices, b.edges, b.faces, tol)
}
