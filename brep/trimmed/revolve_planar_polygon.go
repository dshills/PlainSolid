package trimmed

import (
	"context"
	"fmt"
	"math/big"

	"plainsolid/geom"
)

// PlanarPolygonRevolution describes an exactly resolved coordinate-axis
// revolution. Meridian is in the returned frame's radial/axial coordinates
// and retains the caller's vertex order. Radius is nonnegative.
type PlanarPolygonRevolution struct {
	Frame    geom.Frame
	Meridian []geom.Vec2
}

// PreparePlanarPolygonRevolution resolves a model-space planar polygon and a
// directed axis line. The axis must be coordinate-aligned, lie in the polygon's
// coordinate plane, and leave the entire polygon on one side. The polygon can
// touch the axis. Axis direction magnitude is ignored. Local coordinates must
// be exactly representable and map back to the input vertices without rounding.
// This resolves placement; RevolvePolygon checks the full meridian topology.
func PreparePlanarPolygonRevolution(ctx context.Context, points []geom.Vec3, axisPoint, axisDirection geom.Vec3) (PlanarPolygonRevolution, error) {
	var empty PlanarPolygonRevolution
	if ctx == nil || len(points) < 3 || len(points) > 256 || !axisPoint.IsFinite() || !axisDirection.IsFinite() {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	a := -1
	as := 1.
	for i, x := range [3]float64{axisDirection.X, axisDirection.Y, axisDirection.Z} {
		if x == 0 {
			continue
		}
		if a >= 0 {
			return empty, fmt.Errorf("trimmed: revolution axis is not coordinate-aligned: %w", geom.ErrDomain)
		}
		a = i
		if x < 0 {
			as = -1
		}
	}
	if a < 0 {
		return empty, geom.ErrInvalid
	}
	origin := [3]float64{axisPoint.X, axisPoint.Y, axisPoint.Z}
	first := [3]float64{points[0].X, points[0].Y, points[0].Z}
	constant := [3]bool{true, true, true}
	for _, p := range points {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		if !p.IsFinite() {
			return empty, geom.ErrInvalid
		}
		for i, x := range [3]float64{p.X, p.Y, p.Z} {
			constant[i] = constant[i] && x == first[i]
		}
	}
	if constant[a] {
		return empty, fmt.Errorf("trimmed: revolution axis is transverse to the meridian: %w", geom.ErrDomain)
	}
	radial := -1
	for i := range constant {
		if i == a {
			continue
		}
		if constant[i] {
			if first[i] != origin[i] {
				return empty, fmt.Errorf("trimmed: revolution axis does not lie in the meridian plane: %w", geom.ErrDomain)
			}
		} else if radial >= 0 {
			return empty, fmt.Errorf("trimmed: meridian is not in a coordinate plane containing the axis: %w", geom.ErrDomain)
		} else {
			radial = i
		}
	}
	if radial < 0 {
		return empty, geom.ErrDegenerate
	}
	rs := 0.
	for _, p := range points {
		d := [3]float64{p.X, p.Y, p.Z}[radial]
		if d != origin[radial] {
			sign := 1.
			if d < origin[radial] {
				sign = -1
			}
			if rs != 0 && sign != rs {
				return empty, fmt.Errorf("trimmed: profile crosses the revolution axis: %w", geom.ErrDomain)
			}
			rs = sign
		}
	}
	if rs == 0 {
		return empty, geom.ErrDegenerate
	}
	x, z := [3]float64{}, [3]float64{}
	x[radial], z[a] = rs, as
	frame, e := geom.NewFrame(axisPoint, geom.Vec3{X: x[0], Y: x[1], Z: x[2]}, geom.Vec3{X: z[0], Y: z[1], Z: z[2]})
	if e != nil {
		return empty, e
	}
	result := PlanarPolygonRevolution{Frame: frame, Meridian: make([]geom.Vec2, len(points))}
	for i, p := range points {
		values := [3]float64{p.X, p.Y, p.Z}
		scalar := func(index int, sign float64) (float64, error) {
			v := new(big.Rat).Sub(ratInset(values[index]), ratInset(origin[index]))
			return geom.RepresentedExactScalar(v.Mul(v, ratInset(sign)))
		}
		r, e := scalar(radial, rs)
		if e != nil {
			return empty, e
		}
		h, e := scalar(a, as)
		if e != nil {
			return empty, e
		}
		if frame.ToWorld(geom.Vec3{X: r, Z: h}) != p {
			return empty, geom.ErrUnrepresentable
		}
		result.Meridian[i] = geom.Vec2{X: r, Y: h}
	}
	return result, nil
}

// RevolvePlanarPolygon resolves the model-space sketch and constructs its
// signed analytic revolution about the directed coordinate-axis line.
func RevolvePlanarPolygon(ctx context.Context, points []geom.Vec3, axisPoint, axisDirection geom.Vec3, angle float64) (Solid, error) {
	p, e := PreparePlanarPolygonRevolution(ctx, points, axisPoint, axisDirection)
	if e != nil {
		return Solid{}, e
	}
	return RevolvePolygon(ctx, p.Frame, p.Meridian, angle)
}
