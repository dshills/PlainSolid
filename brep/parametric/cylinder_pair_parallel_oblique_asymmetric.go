package parametric

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
)

// This adapter accepts only the already-proved exact similarity and strict
// transverse disks. The coordinate dispatcher retains all active axial ranges;
// no operation is performed on display meshes or rounded source endpoints.
func finiteObliqueAsymmetricTransverseSet(ctx context.Context, a, b geom.RawCylinder, tol float64, op BooleanOp, basis [3][3]*big.Rat, norm *big.Rat, x, y float64, limits [2][2]*big.Rat, forward [2]bool) (FiniteCylinderPairCommonResult, error) {
	var zero FiniteCylinderPairCommonResult
	q := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	for _, c := range []geom.RawCylinder{a, b} {
		for _, t := range []float64{c.Min, c.Max} {
			for k, o := range []float64{c.Origin.X, c.Origin.Y, c.Origin.Z} {
				p := new(big.Rat).Add(q(o), new(big.Rat).Mul(q([]float64{c.Axis.X, c.Axis.Y, c.Axis.Z}[k]), q(t)))
				if _, e := geom.RepresentedExactScalar(p); e != nil {
					return zero, e
				}
			}
		}
	}
	var local [2]geom.RawCylinder
	for i, c := range []geom.RawCylinder{a, b} {
		r, e := geom.RepresentedExactScalar(new(big.Rat).Quo(q(c.Radius), norm))
		if e != nil {
			return zero, e
		}
		lo, e := geom.RepresentedExactScalar(limits[i][0])
		if e != nil {
			return zero, e
		}
		hi, e := geom.RepresentedExactScalar(limits[i][1])
		if e != nil {
			return zero, e
		}
		local[i] = geom.RawCylinder{Axis: geom.Vec3{Z: 1}, Min: lo, Max: hi, Radius: r}
		if !forward[i] {
			local[i].Axis.Z = -1
			local[i].Min, local[i].Max = -hi, -lo
		}
	}
	local[1].Origin = geom.Vec3{X: x, Y: y}
	d2 := new(big.Rat).Add(new(big.Rat).Mul(q(x), q(x)), new(big.Rat).Mul(q(y), q(y)))
	sum := new(big.Rat).Add(q(local[0].Radius), q(local[1].Radius))
	diff := new(big.Rat).Sub(q(local[0].Radius), q(local[1].Radius))
	if d2.Cmp(new(big.Rat).Mul(sum, sum)) >= 0 || d2.Cmp(new(big.Rat).Mul(diff, diff)) <= 0 {
		return zero, geom.ErrDomain
	}
	// Round tolerance down, so mapping does not enlarge the caller's bound.
	tr := new(big.Rat).Quo(q(tol), norm)
	lt, _ := tr.Float64()
	if finite(lt) && q(lt).Cmp(tr) > 0 {
		lt = math.Nextafter(lt, math.Inf(-1))
	}
	if !(lt > 0) || !finite(lt) {
		return zero, geom.ErrUnrepresentable
	}
	out := FiniteCylinderPairCommonResult{Construction: CylinderCommonParallelAxes}
	if op == Union {
		r, e := FiniteCylinderPairUnion(ctx, local[0], local[1], lt)
		if e != nil {
			return zero, e
		}
		out.Solid, out.SideFaces, out.LowerCapFaces, out.UpperCapFaces = r.Solid, r.WallFaces, r.LowerCapFaces, r.UpperCapFaces
	} else if op == Difference {
		r, e := FiniteCylinderPairDifference(ctx, local[0], local[1], lt)
		if e != nil {
			return zero, e
		}
		if len(r.Solids[1].Faces()) != 0 {
			return zero, geom.ErrDomain
		}
		out.Solid = r.Solids[0]
		out.SideFaces = [2][]int{r.StockWallFaces[0], r.CutterWallFaces[0]} // Difference ownership is a flat source-cap list; the private carrier's lower slot carries that list unchanged.
		out.LowerCapFaces = [2][]int{r.StockCapFaces[0], r.CutterCapFaces[0]}
	} else {
		return zero, geom.ErrDomain
	}
	var m geom.Affine
	for j, v := range basis {
		for i, r := range v {
			f, e := geom.RepresentedExactScalar(r)
			if e != nil {
				return zero, e
			}
			m[i][j] = f
		}
	}
	m[0][3], m[1][3], m[2][3] = a.Origin.X, a.Origin.Y, a.Origin.Z
	s, e := out.Solid.Transformed(m)
	if e != nil {
		return zero, e
	}
	if e = ctx.Err(); e != nil {
		return zero, e
	}
	out.Solid = s
	return out, nil
}
