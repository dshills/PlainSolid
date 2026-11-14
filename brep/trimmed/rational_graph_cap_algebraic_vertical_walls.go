package trimmed

import (
	"math/big"

	"plainsolid/geom"
)

// The zero-XY-Jacobian theorem suffices for vertical fibers but not for
// identifying the original wall at an oblique ray hit. Certify a narrower
// family: a polynomial sweep in Z over a simple projected curve. A strict
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
		h, _, ok := patch.RationalBezierCoefficients(4)
		if !ok {
			return false, nil
		}
		if e := b.charge(len(h)*len(h[0])*48, 0); e != nil {
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
	for i := range n {
		for j := range m {
			q := at(i, j)
			if (i != 0 || j != 0) && q[3].Sign() != 0 {
				return false
			}
			if i != 0 && (q[0].Sign() != 0 || q[1].Sign() != 0 || (i != 1 || j != 0) && q[2].Sign() != 0) {
				return false
			}
		}
	}
	// A nonconstant affine coordinate makes the projected parameter unique.
	affine := [2]bool{at(0, 1)[0].Sign() != 0, at(0, 1)[1].Sign() != 0}
	for j := 2; j < m; j++ {
		for k := range affine {
			affine[k] = affine[k] && at(0, j)[k].Sign() == 0
		}
	}
	if !affine[0] && !affine[1] {
		return false
	}
	for _, plane := range planes {
		// Bound W*(plane(P(0,v))) over v in [0,1] using its power
		// coefficients. The corresponding residual at u=1 only adds
		// the constant axial derivative. Both caps must lie strictly
		// between the two endpoints for the whole original chart.
		v := plane.dot([3]*big.Rat{at(0, 0)[0], at(0, 0)[1], at(0, 0)[2]})
		v.Sub(v, new(big.Rat).Mul(plane.offset, at(0, 0)[3]))
		lo, hi := new(big.Rat).Set(v), new(big.Rat).Set(v)
		for j := 1; j < m; j++ {
			q := at(0, j)
			a := plane.dot([3]*big.Rat{q[0], q[1], q[2]})
			if a.Sign() < 0 {
				lo.Add(lo, a)
			} else {
				hi.Add(hi, a)
			}
		}
		axial := new(big.Rat).Mul(at(1, 0)[2], plane.normal[2])
		if !(hi.Sign() < 0 && new(big.Rat).Add(lo, axial).Sign() > 0 ||
			lo.Sign() > 0 && new(big.Rat).Add(hi, axial).Sign() < 0) {
			return false
		}
	}
	return true
}
