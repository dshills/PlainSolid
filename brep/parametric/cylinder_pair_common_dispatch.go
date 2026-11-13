package parametric

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/geom"
)

type CylinderCommonConstruction uint8

const (
	CylinderCommonEmpty CylinderCommonConstruction = iota
	CylinderCommonIntersectingAxes
	CylinderCommonSkewAxes
	CylinderCommonPerpendicularCap
	CylinderCommonPerpendicularSlab
	CylinderCommonPerpendicularQuadrant
)

// FiniteCylinderPairCommonResult is the native intersection of two supported
// RawCylinder constructions. Face ancestry follows the input order. The
// retained side and end faces can have different numbers of chart patches.
type FiniteCylinderPairCommonResult struct {
	Solid                                   Solid
	Construction                            CylinderCommonConstruction
	SideFaces, LowerCapFaces, UpperCapFaces [2][]int
}

// FiniteCylinderPairCommon selects a native construction for two finite
// cylinders. It handles unclipped nonparallel crossing axes, exactly placed
// skew axes, and selected equal-radius perpendicular cap arrangements. All
// input raw axes and cap limits retain their original meaning. Unsupported
// arrangements fail without returning partial solids.
func FiniteCylinderPairCommon(ctx context.Context, a, b geom.RawCylinder, tolerance float64) (FiniteCylinderPairCommonResult, error) {
	var empty FiniteCylinderPairCommonResult
	if ctx == nil || !finite(tolerance) || tolerance <= 0 {
		return empty, geom.ErrInvalid
	}
	if _, e := geom.NewCylinderPairCommon(a, b); e != nil {
		return empty, e
	}
	m, e := geom.NewObliqueSkewCylinderMap(ctx, a.Origin, a.Axis, a.Radius, b.Origin, b.Axis, b.Radius, geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		return empty, e
	}
	if m.ProjectionRelation <= 0 {
		return empty, nil
	}
	if m.SeparationSign != 0 {
		r, e := ObliqueSkewCylinderCommon(ctx, a, b, tolerance)
		if e != nil {
			return empty, e
		}
		return FiniteCylinderPairCommonResult{Solid: r.Solid, Construction: CylinderCommonSkewAxes, SideFaces: r.CylinderFaces}, nil
	}
	var center geom.Vec3
	coord := [3]*float64{&center.X, &center.Y, &center.Z}
	for i, c := range m.ClosestFeet[0] {
		f, e := geom.RepresentedExactScalar(c)
		if e != nil {
			return empty, e
		}
		*coord[i] = f
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	var windows [2][2]float64
	for i, c := range [2]geom.RawCylinder{a, b} {
		for j, x := range [2]float64{c.Min, c.Max} {
			f, e := geom.RepresentedExactScalar(new(big.Rat).Sub(q(x), m.ClosestRawParameters[i]))
			if e != nil {
				return empty, e
			}
			windows[i][j] = f
		}
	}
	proof, e := ObliqueCylinderCommon(ctx, center, a.Axis, b.Axis, a.Radius, b.Radius, windows[0], windows[1], tolerance)
	if e == nil {
		return FiniteCylinderPairCommonResult{Solid: proof.Solid, Construction: CylinderCommonIntersectingAxes, SideFaces: proof.CylinderFaces}, nil
	}
	if !errors.Is(e, geom.ErrConvergence) {
		return empty, e
	}
	if a.Radius != b.Radius {
		return empty, e
	}
	return finitePerpendicularCapCommon(ctx, a, b, center, windows, tolerance)
}

// The clipped family currently uses exact +X/+Z unit axes. The
// general free-axis construction above does not need this restriction.
func finitePerpendicularCapCommon(ctx context.Context, a, b geom.RawCylinder, center geom.Vec3, ends [2][2]float64, tolerance float64) (FiniteCylinderPairCommonResult, error) {
	var empty FiniteCylinderPairCommonResult
	if a.Axis != (geom.Vec3{X: 1}) || b.Axis != (geom.Vec3{Z: 1}) {
		return empty, geom.ErrDomain
	}
	r := a.Radius
	if ends[0][0] >= r || ends[0][1] <= -r || ends[1][0] >= r || ends[1][1] <= -r {
		return empty, nil
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	guard := new(big.Rat).Add(q(r), new(big.Rat).Mul(big.NewRat(8, 1), q(tolerance)))
	before := func(x float64) bool { return q(x).Cmp(new(big.Rat).Neg(guard)) < 0 }
	after := func(x float64) bool { return q(x).Cmp(guard) > 0 }
	clear := func(w [2]float64) bool { return before(w[0]) && after(w[1]) }
	interior := func(x float64) bool { return x > -r && x < r }
	if clear(ends[1]) {
		z := min(-ends[1][0], ends[1][1])
		l, u := ends[0][0], ends[0][1]
		if interior(l) && interior(u) {
			out, e := PerpendicularCylinderSlab(ctx, center, r, l, u, z, tolerance, geom.ExactCurvePlaneEventOptions{})
			if e != nil {
				return empty, e
			}
			return FiniteCylinderPairCommonResult{out.Solid, CylinderCommonPerpendicularSlab, [2][]int{out.XAxisFaces, out.ZAxisFaces}, [2][]int{out.LowerCapFaces, nil}, [2][]int{out.UpperCapFaces, nil}}, nil
		}
		if before(l) && interior(u) {
			if u == 0 {
				out, e := PerpendicularCylinderEquatorHalf(ctx, center, r, -l, z, tolerance, false)
				if e != nil {
					return empty, e
				}
				return FiniteCylinderPairCommonResult{out.Solid, CylinderCommonPerpendicularCap, [2][]int{out.XAxisFaces, out.ZAxisFaces}, [2][]int{}, [2][]int{{out.CapFace}, nil}}, nil
			}
			out, e := PerpendicularCylinderAlgebraicCap(ctx, center, r, u, -l, z, tolerance, geom.ExactCurvePlaneEventOptions{})
			if e != nil {
				return empty, e
			}
			return FiniteCylinderPairCommonResult{out.Solid, CylinderCommonPerpendicularCap, [2][]int{out.XAxisFaces, out.ZAxisFaces}, [2][]int{}, [2][]int{{out.CapFace}, nil}}, nil
		}
		if after(u) && interior(l) {
			// Reflect the X coordinate after constructing the complementary
			// upper-cap chart. The transform keeps the body orientation.
			m := geom.Affine{{-1, 0, 0, center.X}, {0, 1, 0, center.Y}, {0, 0, 1, center.Z}}
			local, e := finitePerpendicularCapCommon(ctx, a, b, geom.Vec3{}, [2][2]float64{{-u, -l}, ends[1]}, tolerance)
			if e != nil {
				return empty, e
			}
			local.Solid, e = local.Solid.Transformed(m)
			if e != nil {
				return empty, e
			}
			local.LowerCapFaces, local.UpperCapFaces = local.UpperCapFaces, local.LowerCapFaces
			return local, nil
		}
	}
	if clear(ends[0]) {
		swapped, e := finitePerpendicularCapCommon(ctx, geom.RawCylinder{Axis: geom.Vec3{X: 1}, Radius: r}, geom.RawCylinder{Axis: geom.Vec3{Z: 1}, Radius: r}, geom.Vec3{}, [2][2]float64{ends[1], ends[0]}, tolerance)
		if e != nil {
			return empty, e
		}
		m := geom.Affine{{0, 0, 1, center.X}, {0, 1, 0, center.Y}, {1, 0, 0, center.Z}}
		swapped.Solid, e = swapped.Solid.Transformed(m)
		if e != nil {
			return empty, e
		}
		for _, g := range []*[2][]int{&swapped.SideFaces, &swapped.LowerCapFaces, &swapped.UpperCapFaces} {
			g[0], g[1] = g[1], g[0]
		}
		return swapped, nil
	}
	x, ok := centerCapHalf(ends[0], before, after)
	z, ok2 := centerCapHalf(ends[1], before, after)
	if ok && ok2 {
		out, e := PerpendicularCylinderQuadrant(ctx, center, r, x.length, z.length, tolerance, x.positive, z.positive)
		if e != nil {
			return empty, e
		}
		result := FiniteCylinderPairCommonResult{Solid: out.Solid, Construction: CylinderCommonPerpendicularQuadrant, SideFaces: [2][]int{{out.XAxisFace}, {out.ZAxisFace}}}
		for i, p := range []bool{x.positive, z.positive} {
			id := [2]int{out.XCapFace, out.ZCapFace}[i]
			if p {
				result.LowerCapFaces[i] = []int{id}
			} else {
				result.UpperCapFaces[i] = []int{id}
			}
		}
		return result, nil
	}
	return empty, geom.ErrConvergence
}

type cylinderCenterHalf struct {
	length   float64
	positive bool
}

func centerCapHalf(w [2]float64, before, after func(float64) bool) (cylinderCenterHalf, bool) {
	if w[0] == 0 && after(w[1]) {
		return cylinderCenterHalf{w[1], true}, true
	}
	if w[1] == 0 && before(w[0]) {
		return cylinderCenterHalf{-w[0], false}, true
	}
	return cylinderCenterHalf{}, false
}
