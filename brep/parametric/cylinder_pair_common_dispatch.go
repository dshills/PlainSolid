package parametric

import (
	"context"
	"errors"
	"math"
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
	CylinderCommonParallelAxes
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
// cylinders. It handles coordinate-parallel axes, unclipped nonparallel
// crossing or exactly placed skew axes, and selected perpendicular caps on
// signed coordinate axes of arbitrary nonzero scale. All
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
	if rawAxesParallel(a.Axis, b.Axis) {
		return finiteParallelCylinderCommon(ctx, a, b, tolerance)
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

func rawAxesParallel(a, b geom.Vec3) bool {
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	x, y := [3]float64{a.X, a.Y, a.Z}, [3]float64{b.X, b.Y, b.Z}
	for i := range 3 {
		j, k := (i+1)%3, (i+2)%3
		if new(big.Rat).Mul(q(x[j]), q(y[k])).Cmp(new(big.Rat).Mul(q(x[k]), q(y[j]))) != 0 {
			return false
		}
	}
	return true
}

func finiteParallelCylinderCommon(ctx context.Context, a, b geom.RawCylinder, tolerance float64) (FiniteCylinderPairCommonResult, error) {
	var empty FiniteCylinderPairCommonResult
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	axis := 0
	if a.Axis.Y != 0 {
		axis = 1
	}
	if a.Axis.Z != 0 {
		axis = 2
	}
	axisValues := func(v geom.Vec3) [3]float64 { return [3]float64{v.X, v.Y, v.Z} }
	for _, s := range [2]geom.RawCylinder{a, b} {
		for j, v := range axisValues(s.Axis) {
			if j != axis && v != 0 {
				return empty, geom.ErrDomain
			}
		}
	}
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	minmax := [2][2]*big.Rat{}
	ownerLower := [2]bool{}
	for i, s := range [2]geom.RawCylinder{a, b} {
		base, scale := q(axisValues(s.Origin)[axis]), q(axisValues(s.Axis)[axis])
		lo := new(big.Rat).Add(base, new(big.Rat).Mul(scale, q(s.Min)))
		hi := new(big.Rat).Add(base, new(big.Rat).Mul(scale, q(s.Max)))
		ownerLower[i] = scale.Sign() > 0
		if !ownerLower[i] {
			lo, hi = hi, lo
		}
		minmax[i] = [2]*big.Rat{lo, hi}
	}
	lo, hi := minmax[0][0], minmax[0][1]
	if minmax[1][0].Cmp(lo) > 0 {
		lo = minmax[1][0]
	}
	if minmax[1][1].Cmp(hi) < 0 {
		hi = minmax[1][1]
	}
	if lo.Cmp(hi) >= 0 {
		return empty, nil
	}
	j, k := (axis+1)%3, (axis+2)%3
	coordA, coordB := axisValues(a.Origin), axisValues(b.Origin)
	dx := new(big.Rat).Sub(q(coordB[j]), q(coordA[j]))
	dy := new(big.Rat).Sub(q(coordB[k]), q(coordA[k]))
	sq := func(x *big.Rat) *big.Rat { return new(big.Rat).Mul(x, x) }
	d2 := new(big.Rat).Add(sq(dx), sq(dy))
	if d2.Cmp(sq(new(big.Rat).Add(q(a.Radius), q(b.Radius)))) >= 0 {
		return empty, nil
	}
	start, e := geom.RepresentedExactScalar(lo)
	if e != nil {
		return empty, e
	}
	if _, e := geom.RepresentedExactScalar(hi); e != nil {
		return empty, e
	}
	height, e := geom.RepresentedExactScalar(new(big.Rat).Sub(hi, lo))
	if e != nil {
		return empty, e
	}
	x, e := geom.RepresentedExactScalar(dx)
	if e != nil {
		return empty, e
	}
	y, e := geom.RepresentedExactScalar(dy)
	if e != nil {
		return empty, e
	}
	coordA[axis] = start
	vec := func(c [3]float64) geom.Vec3 { return geom.Vec3{X: c[0], Y: c[1], Z: c[2]} }
	var ex, ez [3]float64
	ex[j], ez[axis] = 1, 1
	f, e := geom.NewFrame(vec(coordA), vec(ex), vec(ez))
	if e != nil {
		return empty, e
	}
	s, e := CombineParallelCylinders(f, DiskProfile{Radius: a.Radius}, DiskProfile{Center: geom.Vec2{X: x, Y: y}, Radius: b.Radius}, height, Intersection, tolerance)
	if e != nil {
		return empty, e
	}
	out := FiniteCylinderPairCommonResult{Solid: s, Construction: CylinderCommonParallelAxes}
	for id, face := range s.Faces() {
		if cylinder, ok := face.Surface.(geom.CylinderSurface); ok {
			f0, r := cylinder.Parameters()
			center := axisValues(f0.Origin())
			for i, c := range [2]geom.RawCylinder{a, b} {
				p := axisValues(c.Origin)
				if center[j] == p[j] && center[k] == p[k] && r == c.Radius {
					out.SideFaces[i] = append(out.SideFaces[i], id)
				}
			}
			continue
		}
		for i, m := range minmax {
			if !face.Reversed && m[0].Cmp(lo) == 0 || face.Reversed && m[1].Cmp(hi) == 0 {
				if ownerLower[i] != face.Reversed {
					out.LowerCapFaces[i] = append(out.LowerCapFaces[i], id)
				} else {
					out.UpperCapFaces[i] = append(out.UpperCapFaces[i], id)
				}
			}
		}
	}
	if e = ctx.Err(); e != nil {
		return empty, e
	}
	return out, nil
}

// The clipped charts are built with +X and +Z at unit scale. An exact
// signed permutation places them on the original cylinder axes; converting
// cap parameters to physical lengths must not snap a cap to a nearby plane.
func finitePerpendicularCapCommon(ctx context.Context, a, b geom.RawCylinder, center geom.Vec3, ends [2][2]float64, tolerance float64) (FiniteCylinderPairCommonResult, error) {
	var empty FiniteCylinderPairCommonResult
	var basis [2]geom.Vec3
	for i, cylinder := range [2]geom.RawCylinder{a, b} {
		axis := cylinder.Axis
		var scale float64
		switch {
		case axis.Y == 0 && axis.Z == 0:
			scale, basis[i].X = math.Abs(axis.X), math.Copysign(1, axis.X)
		case axis.X == 0 && axis.Z == 0:
			scale, basis[i].Y = math.Abs(axis.Y), math.Copysign(1, axis.Y)
		case axis.X == 0 && axis.Y == 0:
			scale, basis[i].Z = math.Abs(axis.Z), math.Copysign(1, axis.Z)
		default:
			return empty, geom.ErrDomain
		}
		for j, cap := range ends[i] {
			q := new(big.Rat).Mul(new(big.Rat).SetFloat64(scale), new(big.Rat).SetFloat64(cap))
			length, e := geom.RepresentedExactScalar(q)
			if e != nil {
				return empty, e
			}
			ends[i][j] = length
		}
	}
	x, z := basis[0], basis[1]
	if x.Dot(z) != 0 {
		return empty, geom.ErrDomain
	}
	if x == (geom.Vec3{X: 1}) && z == (geom.Vec3{Z: 1}) {
		return finiteCanonicalPerpendicularCapCommon(ctx, center, a.Radius, ends, tolerance)
	}
	out, e := finiteCanonicalPerpendicularCapCommon(ctx, geom.Vec3{}, a.Radius, ends, tolerance)
	if e != nil || out.Construction == CylinderCommonEmpty {
		return out, e
	}
	y := z.Cross(x)
	out.Solid, e = out.Solid.Transformed(geom.Affine{
		{x.X, y.X, z.X, center.X}, {x.Y, y.Y, z.Y, center.Y}, {x.Z, y.Z, z.Z, center.Z},
	})
	if e != nil {
		return empty, e
	}
	return out, nil
}

func finiteCanonicalPerpendicularCapCommon(ctx context.Context, center geom.Vec3, r float64, ends [2][2]float64, tolerance float64) (FiniteCylinderPairCommonResult, error) {
	var empty FiniteCylinderPairCommonResult
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
			local, e := finiteCanonicalPerpendicularCapCommon(ctx, geom.Vec3{}, r, [2][2]float64{{-u, -l}, ends[1]}, tolerance)
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
		swapped, e := finiteCanonicalPerpendicularCapCommon(ctx, geom.Vec3{}, r, [2][2]float64{ends[1], ends[0]}, tolerance)
		if e != nil || swapped.Construction == CylinderCommonEmpty {
			return swapped, e
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
