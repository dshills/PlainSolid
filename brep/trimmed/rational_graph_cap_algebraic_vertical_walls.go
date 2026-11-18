//go:build research

package trimmed

import (
	"math/big"

	"plainsolid/geom"
)

// The zero-XY-Jacobian theorem suffices for vertical fibers but not for
// identifying the original wall at an oblique ray hit. Certify a narrower
// family: an exact sweep in Z over a simple projected curve. A strict
// enclosing axial domain for both caps excludes stacked wall charts and makes
// each interior wall point correspond to an actual lower-cap trim occurrence.
func (c AlgebraicGraphCapPair) proveVerticalLineWalls(boundary AlgebraicGraphShellBoundary, options RationalGraphShellSectionOptions, b *capClipBudget) (bool, error) {
	seen := make(map[int]bool)
	for _, f := range boundary.Faces {
		if f.StockFace < 0 || seen[f.StockFace] {
			continue
		}
		seen[f.StockFace] = true
		face := c.caps[0].recipe.stock.faces[f.StockFace]
		patch, e := exactFaceSupportPatch(face, options.Spans.Faces.Patches)
		if e != nil {
			return false, e
		}
		domain, e := faceUVBounds(face)
		if e != nil {
			return false, e
		}
		patch, e = patch.Trim(domain)
		if e != nil {
			return false, e
		}
		patchLimits := options.Spans.Faces.Patches
		if patchLimits.MaxOperations == 0 || patchLimits.MaxOperations > b.work {
			patchLimits.MaxOperations = b.work
		}
		if patchLimits.MaxCoefficientBits == 0 || patchLimits.MaxCoefficientBits > b.bits {
			patchLimits.MaxCoefficientBits = b.bits
		}
		patch, e = patch.WithLimits(patchLimits)
		if e != nil {
			return false, e
		}
		h, _, ok := patch.RationalBezierCoefficients(4)
		if !ok {
			return false, nil
		}
		if e := b.charge(len(h)*len(h[0])*160, 0); e != nil {
			return false, e
		}
		for _, row := range h {
			for _, q := range row {
				for _, r := range q {
					if r.Num().BitLen() > b.bits || r.Denom().BitLen() > b.bits {
						return false, geom.ErrConvergence
					}
				}
			}
		}
		planes := [2]polynomialPlaneEquation{c.caps[0].regions.plane, c.caps[1].regions.plane}
		if !spansVerticalGraphPlanes(h, false, planes) && !spansVerticalGraphPlanes(h, true, planes) {
			return false, nil
		}
	}
	return len(seen) > 0, nil
}

func spansVerticalGraphPlanes(h [][][4]*big.Rat, swap bool, planes [2]polynomialPlaneEquation) bool {
	n, m := len(h), len(h[0])
	at := func(i, j int) [4]*big.Rat { return h[i][j] }
	if swap {
		n, m = m, n
		at = func(i, j int) [4]*big.Rat { return h[j][i] }
	}
	if n < 2 || m < 2 || at(1, 0)[2].Sign() == 0 || at(0, 0)[3].Sign() <= 0 {
		return false
	}
	// The projective weight may vary with the transverse parameter. The
	// only axial dependence must be a constant Euclidean rise: Z+=kW*u.
	rise := new(big.Rat).Quo(at(1, 0)[2], at(0, 0)[3])
	for i := 1; i < n; i++ {
		for j := range m {
			q := at(i, j)
			z := new(big.Rat)
			if i == 1 {
				z.Mul(rise, at(0, j)[3])
			}
			if q[0].Sign() != 0 || q[1].Sign() != 0 || q[2].Cmp(z) != 0 || q[3].Sign() != 0 {
				return false
			}
		}
	}
	// A strictly monotone projected coordinate prevents self-overlap and
	// identifies one original transverse parameter at every wall point.
	injective := false
	for k := range 2 {
		d := make([]*big.Rat, 2*m-2)
		for i := range d {
			d[i] = new(big.Rat)
		}
		for i := 1; i < m; i++ {
			for j := range m {
				a, z := at(0, i), at(0, j)
				x := new(big.Rat).Sub(new(big.Rat).Mul(a[k], z[3]), new(big.Rat).Mul(a[3], z[k]))
				d[i+j-1].Add(d[i+j-1], new(big.Rat).Mul(x, big.NewRat(int64(i), 1)))
			}
		}
		injective = injective || strictGraphWallBernsteinSign(d) != 0
	}
	if !injective {
		return false
	}
	for _, plane := range planes {
		// The raw homogeneous residual is (N·P - offset*W).
		// Prove strict opposite signs across the entire two axial
		// edges, so every transverse location spans both caps.
		var end [2][]*big.Rat
		for i := range end {
			end[i] = make([]*big.Rat, m)
		}
		for j := range m {
			q := at(0, j)
			v := plane.dot([3]*big.Rat{q[0], q[1], q[2]})
			v.Sub(v, new(big.Rat).Mul(plane.offset, q[3]))
			end[0][j] = v
			end[1][j] = new(big.Rat).Add(v, new(big.Rat).Mul(at(1, j)[2], plane.normal[2]))
		}
		a, z := strictGraphWallBernsteinSign(end[0]), strictGraphWallBernsteinSign(end[1])
		if a == 0 || a != -z {
			return false
		}
	}
	return true
}

func strictGraphWallBernsteinSign(power []*big.Rat) int {
	d := len(power) - 1
	sign := power[0].Sign()
	if sign == 0 {
		return 0
	}
	for i := 1; i <= d; i++ {
		v := new(big.Rat)
		for j := 0; j <= i; j++ {
			v.Add(v, new(big.Rat).Mul(power[j], big.NewRat(polynomialChoose(i, j), polynomialChoose(d, j))))
		}
		if v.Sign() != sign {
			return 0
		}
	}
	return sign
}
