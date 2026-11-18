//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// mixedExactPlaneProposal recognizes exact planes with represented
// provenance. The homogeneous identity checks the full surface; corner
// samples alone would miss saddle and bowed patches.
func mixedExactPlaneProposal(ctx context.Context, patch *geom.SurfacePatch, face int, budget *int) (MixedPlanarFragmentProposal, bool, error) {
	var zero MixedPlanarFragmentProposal
	if err := ctx.Err(); err != nil {
		return zero, false, err
	}
	// The fixed allowance covers coefficient conversion, three bounded
	// exact evaluations, and all homogeneous verification comparisons.
	const work = 12000
	if *budget <= work {
		return zero, false, geom.ErrConvergence
	}
	*budget -= work
	patch, err := patch.WithLimits(geom.SurfacePatchOptions{MaxOperations: 6144, MaxCoefficientBits: 4096})
	if err != nil {
		return zero, false, err
	}
	h, _, ok := patch.RationalBezierCoefficients(4)
	if !ok || len(h) == 0 || len(h[0]) == 0 || h[0][0][3].Sign() <= 0 {
		return zero, false, geom.ErrUnrepresentable
	}
	for axis := range 3 {
		c := new(big.Rat).Quo(h[0][0][axis], h[0][0][3])
		matches := true
		for _, row := range h {
			for _, entry := range row {
				if new(big.Rat).Mul(entry[3], c).Cmp(entry[axis]) != 0 {
					matches = false
					break
				}
			}
			if !matches {
				break
			}
		}
		if !matches {
			continue
		}
		v, exact := c.Float64()
		if !exact || !finite(v) {
			return zero, false, geom.ErrUnrepresentable
		}
		return MixedPlanarFragmentProposal{Face: face, Plane: mixedCoordinatePlane(axis, v),
			ProjectionAxes: [2]int{(axis + 1) % 3, (axis + 2) % 3}}, true, nil
	}
	// A tilted plane is admissible only when three noncollinear
	// original points are exactly representable. The proposed plane
	// is then checked against every homogeneous coefficient.
	hits := [3][3]*big.Rat{}
	d := patch.Domain()
	params := [3][2]float64{{d.UMin, d.VMin}, {d.UMax, d.VMin}, {d.UMin, d.VMax}}
	for i, uv := range params {
		p, e := patch.EvaluateRational(ctx, ratInset(uv[0]), ratInset(uv[1]),
			geom.SurfacePatchEvaluationOptions{MaxOperations: 1600, MaxCoefficientBits: 4096})
		if e != nil {
			return zero, false, e
		}
		hits[i] = p.Point
	}
	var n [3]*big.Rat
	axis := -1
	for i := range n {
		j, k := (i+1)%3, (i+2)%3
		v := func(p, index int) *big.Rat { return new(big.Rat).Sub(hits[p][index], hits[0][index]) }
		n[i] = new(big.Rat).Sub(new(big.Rat).Mul(v(1, j), v(2, k)), new(big.Rat).Mul(v(1, k), v(2, j)))
		if n[i].Sign() != 0 && (axis < 0 || new(big.Rat).Abs(n[i]).Cmp(new(big.Rat).Abs(n[axis])) > 0) {
			axis = i
		}
	}
	if axis == -1 {
		return zero, false, nil
	}
	dot := func(p [3]*big.Rat) *big.Rat {
		q := new(big.Rat)
		for i, a := range n {
			q.Add(q, new(big.Rat).Mul(a, p[i]))
		}
		return q
	}
	offset := dot(hits[0])
	for _, row := range h {
		for _, c := range row {
			if dot([3]*big.Rat{c[0], c[1], c[2]}).Cmp(new(big.Rat).Mul(offset, c[3])) != 0 {
				return zero, false, nil
			}
		}
	}
	var points [3]geom.Vec3
	for i, x := range hits {
		var m [3]float64
		for k, a := range x {
			v, exact := a.Float64()
			if !exact || !finite(v) {
				return zero, false, geom.ErrUnrepresentable
			}
			m[k] = v
		}
		points[i] = geom.Vec3{X: m[0], Y: m[1], Z: m[2]}
	}
	return MixedPlanarFragmentProposal{Face: face, Plane: points, ProjectionAxes: [2]int{(axis + 1) % 3, (axis + 2) % 3}}, true, nil
}
