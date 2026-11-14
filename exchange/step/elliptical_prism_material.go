package step

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// EllipticalPrismMaterial is an explicitly reconstructed native solid. Four
// rational side charts replace one periodic source face; four exact rational
// quarter edges partition each original full rim. No source approximation is
// promoted as exact: Shell retains the enforced periodic repair evidence.
type EllipticalPrismMaterial struct {
	Solid               trimmed.Solid
	Shell               RepairedEllipticalPrismShell
	NativeFaceSourceIDs [6]int64  // lower cap, upper cap, four sides
	NativeRimEdges      [2][4]int // lower, upper, increasing source ellipse angle
	SourceRimEdgeIDs    [2]int64
}

// PromoteRepairedEllipticalPrism authenticates the retained source again, then
// constructs an embedded ellipse prism using rational quarters (weights1,1,2).
// Every world control point must be losslessly representable. Exact sewing
// proves continuous model/UV agreement; positive radii and strictly increasing
// height establish the disjoint quadrant walls and bounded cap disks. Tolerance
// is native validation tolerance, not permission to round geometric data.
func PromoteRepairedEllipticalPrism(ctx context.Context, source *File, l Limits, o EllipticalWireRepairOptions, tolerance float64) (EllipticalPrismMaterial, error) {
	fail := func(e error) (EllipticalPrismMaterial, error) { return EllipticalPrismMaterial{}, e }
	if ctx == nil || !finiteNumber(tolerance) || tolerance <= 0 || !ellipseOwnedSourceBound(source, 121) {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	raw, e := mappedPlanarSourceBytes(source)
	if e != nil {
		return fail(e)
	}
	shell, e := RepairRoundedEllipticalPrismShell(bytes.NewReader(raw), l, o)
	if e != nil {
		return fail(e)
	}
	frame, a, b := shell.Side.Profile.Parameters()
	height := shell.Side.Height
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
			if !exact || !finiteNumber(v[i]) {
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
					return fail(e)
				}
				worldPoles[k][q] = append(worldPoles[k][q], v)
			}
		}
	}
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
			return fail(e)
		}
		part, e := trimmed.RectangularSplineFace(s, false)
		if e != nil {
			return fail(e)
		}
		parts = append(parts, part)
	}
	for k, z := range []float64{0, height} {
		origin, e := point(0, 0, z)
		if e != nil {
			return fail(e)
		}
		f, e := geom.NewFrame(origin, frame.XAxis(), frame.ZAxis())
		if e != nil {
			return fail(e)
		}
		plane, e := geom.NewPlaneSurface(f)
		if e != nil {
			return fail(e)
		}
		part := trimmed.SewFace{Face: trimmed.Face{Surface: plane, Reversed: k == 0}}
		for q := 0; q < 4; q++ {
			curve, e := geom.NewBSpline(2, worldPoles[k][q], knots, weights)
			if e != nil {
				return fail(e)
			}
			uv, e := geom.NewBSpline(2, uvPoles[q], knots, weights)
			if e != nil {
				return fail(e)
			}
			part.Vertices = append(part.Vertices, worldPoles[k][q][0])
			part.Edges = append(part.Edges, parametric.Edge{Start: q, End: (q + 1) % 4, Curve: curve, Range: parametric.Interval{Max: 1}})
			p0, p1 := uvPoles[q][0], uvPoles[q][2]
			part.Face.Outer = append(part.Face.Outer, trimmed.Use{Edge: q, UVStart: geom.Vec2{X: p0.X, Y: p0.Y}, UVEnd: geom.Vec2{X: p1.X, Y: p1.Y}, Curve: trimmed.SplineUV{Curve: uv, Range: trimmed.UVInterval{Max: 1}}})
		}
		parts = append(parts, part)
	}
	parts = append(append([]trimmed.SewFace{}, parts[4:]...), parts[:4]...)
	solid, history, e := trimmed.Sew(ctx, parts, trimmed.SewOptions{Tolerance: tolerance, MaxFaces: 6, MaxUses: 24, Proof: edgecheck.Options{}})
	if e != nil {
		return fail(fmt.Errorf("step: exact ellipse prism sewing: %w", e))
	}
	out := EllipticalPrismMaterial{Solid: solid, Shell: shell, SourceRimEdgeIDs: [2]int64{shell.SourceEdgeIDs[2], shell.SourceEdgeIDs[1]}}
	for i := 0; i < 4; i++ {
		out.NativeFaceSourceIDs[i+2] = shell.SourceFaceIDs[0]
	}
	out.NativeFaceSourceIDs[0] = shell.SourceFaceIDs[1]
	out.NativeFaceSourceIDs[1] = shell.SourceFaceIDs[2]
	for k := 0; k < 2; k++ {
		for q := 0; q < 4; q++ {
			out.NativeRimEdges[k][q] = history.Edges[k][q]
		}
	}
	return out, nil
}
