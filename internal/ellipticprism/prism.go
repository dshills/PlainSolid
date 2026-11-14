// Package ellipticprism shares exact native ellipse construction between codecs.
package ellipticprism

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// Prism constructs the same immutable exact quarter geometry
// for the independently proved model-only and explicitly repaired dialects.
func Prism(ctx context.Context, frame geom.Frame, a, b, height, tolerance float64) (trimmed.Solid, [2][4]int, error) {
	parts, e := Parts(ctx, frame, a, b, height, tolerance)
	if e != nil {
		return trimmed.Solid{}, [2][4]int{}, e
	}
	return sewParts(ctx, parts, tolerance)
}

// Parts returns exact cap/cap/four-wall faces for already proved ellipse data.
func Parts(ctx context.Context, frame geom.Frame, a, b, height, tolerance float64) ([]trimmed.SewFace, error) {
	failure := func(e error) ([]trimmed.SewFace, error) { return nil, e }
	if ctx == nil || !finite(a) || !finite(b) || !finite(height) || !finite(tolerance) || a <= b || b <= 0 || height <= 0 || tolerance <= 0 {
		return failure(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return failure(e)
	}
	xyz := func(v geom.Vec3) [3]float64 { return [3]float64{v.X, v.Y, v.Z} }
	point := func(x, y, z float64) (geom.Vec3, error) {
		axes := [3][3]float64{xyz(frame.XAxis()), xyz(frame.YAxis()), xyz(frame.ZAxis())}
		origin := xyz(frame.Origin())
		v := [3]float64{}
		for i := range 3 {
			q := new(big.Rat).SetFloat64(origin[i])
			for j, c := range []float64{x, y, z} {
				q.Add(q, new(big.Rat).Mul(new(big.Rat).SetFloat64(c), new(big.Rat).SetFloat64(axes[j][i])))
			}
			var exact bool
			v[i], exact = q.Float64()
			if !exact || !finite(v[i]) {
				return geom.Vec3{}, geom.ErrUnrepresentable
			}
		}
		return geom.Vec3{X: v[0], Y: v[1], Z: v[2]}, nil
	}
	uvPoles := [4][]geom.Vec3{}
	worldPoles := [2][4][]geom.Vec3{}
	for q := 0; q < 4; q++ {
		for _, p := range [][2]float64{{1, 0}, {1, 1}, {0, 1}} {
			x, y := p[0], p[1]
			for i := 0; i < q; i++ {
				x, y = -y, x
			}
			x *= a
			y *= b
			uvPoles[q] = append(uvPoles[q], geom.Vec3{X: x, Y: y})
			for k, z := range []float64{0, height} {
				v, e := point(x, y, z)
				if e != nil {
					return failure(e)
				}
				worldPoles[k][q] = append(worldPoles[k][q], v)
			}
		}
	}
	caps := [2]geom.Frame{}
	for k, z := range []float64{0, height} {
		origin, e := point(0, 0, z)
		if e != nil {
			return failure(e)
		}
		caps[k], e = geom.NewFrame(origin, frame.XAxis(), frame.ZAxis())
		if e != nil {
			return failure(e)
		}
	}
	return partsControls(ctx, uvPoles, worldPoles, caps, tolerance)
}

// SewControls requires the caller to prove these complete ellipse quarters and
// cap frames describe one injective positive-height extrusion. It verifies
// continuous edge agreement and closes the exact native topology.
func SewControls(ctx context.Context, uvPoles [4][]geom.Vec3, worldPoles [2][4][]geom.Vec3, caps [2]geom.Frame, tolerance float64) (trimmed.Solid, [2][4]int, error) {
	parts, e := partsControls(ctx, uvPoles, worldPoles, caps, tolerance)
	if e != nil {
		return trimmed.Solid{}, [2][4]int{}, e
	}
	return sewParts(ctx, parts, tolerance)
}
func partsControls(ctx context.Context, uvPoles [4][]geom.Vec3, worldPoles [2][4][]geom.Vec3, caps [2]geom.Frame, tolerance float64) ([]trimmed.SewFace, error) {
	failure := func(e error) ([]trimmed.SewFace, error) { return nil, e }
	knots := []float64{0, 0, 0, 1, 1, 1}
	weights := []float64{1, 1, 2}
	parts := make([]trimmed.SewFace, 0, 6)
	for q := 0; q < 4; q++ {
		poles := []geom.Vec3{}
		for i := 0; i < 3; i++ {
			poles = append(poles, worldPoles[0][q][i], worldPoles[1][q][i])
		}
		s, e := geom.NewBSplineSurface(2, 1, 3, 2, poles, knots, []float64{0, 0, 1, 1}, []float64{1, 1, 1, 1, 2, 2})
		if e != nil {
			return failure(e)
		}
		part, e := trimmed.RectangularSplineFace(s, false)
		if e != nil {
			return failure(e)
		}
		parts = append(parts, part)
	}
	for k, f := range caps {
		plane, e := geom.NewPlaneSurface(f)
		if e != nil {
			return failure(e)
		}
		part := trimmed.SewFace{Face: trimmed.Face{Surface: plane, Reversed: k == 0}}
		for q := 0; q < 4; q++ {
			curve, e := geom.NewBSpline(2, worldPoles[k][q], knots, weights)
			if e != nil {
				return failure(e)
			}
			uv, e := geom.NewBSpline(2, uvPoles[q], knots, weights)
			if e != nil {
				return failure(e)
			}
			part.Vertices = append(part.Vertices, worldPoles[k][q][0])
			part.Edges = append(part.Edges, parametric.Edge{Start: q, End: (q + 1) % 4, Curve: curve, Range: parametric.Interval{Max: 1}})
			p0, p1 := uvPoles[q][0], uvPoles[q][2]
			part.Face.Outer = append(part.Face.Outer, trimmed.Use{Edge: q, UVStart: geom.Vec2{X: p0.X, Y: p0.Y}, UVEnd: geom.Vec2{X: p1.X, Y: p1.Y}, Curve: trimmed.SplineUV{Curve: uv, Range: trimmed.UVInterval{Max: 1}}})
		}
		parts = append(parts, part)
	}
	parts = append(append([]trimmed.SewFace{}, parts[4:]...), parts[:4]...)
	return parts, nil
}
func sewParts(ctx context.Context, parts []trimmed.SewFace, tolerance float64) (trimmed.Solid, [2][4]int, error) {
	failure := func(e error) (trimmed.Solid, [2][4]int, error) { return trimmed.Solid{}, [2][4]int{}, e }
	solid, history, e := trimmed.Sew(ctx, parts, trimmed.SewOptions{Tolerance: tolerance, MaxFaces: 6, MaxUses: 24, Proof: edgecheck.Options{}})
	if e != nil {
		return failure(fmt.Errorf("native exact ellipse prism sewing: %w", e))
	}
	var rim [2][4]int
	for k := 0; k < 2; k++ {
		for q := 0; q < 4; q++ {
			rim[k][q] = history.Edges[k][q]
		}
	}
	return solid, rim, nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
