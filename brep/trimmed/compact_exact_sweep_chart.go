package trimmed

import (
	"math/big"

	"plainsolid/geom"
)

// The cap of a directed sweep uses a bilinear spline to represent (u,v,z)
// exactly. Recognize that parameter law from all the original controls,
// rather than treating a nearly flat spline as a plane.
func compactExactXYChart(s *geom.BSplineSurface) bool {
	if s == nil {
		return false
	}
	u0, u1, v0, v1 := s.Domain()
	d := s.Data()
	if d.DegreeU != 1 || d.DegreeV != 1 || d.CountU != 2 || d.CountV != 2 ||
		len(d.Poles) != 4 || len(d.KnotsU) != 4 || len(d.KnotsV) != 4 ||
		d.KnotsU[0] != u0 || d.KnotsU[3] != u1 || d.KnotsV[0] != v0 || d.KnotsV[3] != v1 {
		return false
	}
	z := d.Poles[0].Z
	want := [4]geom.Vec3{{X: u0, Y: v0, Z: z}, {X: u0, Y: v1, Z: z}, {X: u1, Y: v0, Z: z}, {X: u1, Y: v1, Z: z}}
	for i, p := range d.Poles {
		if p != want[i] || len(d.Weights) > 0 && d.Weights[i] != d.Weights[0] {
			return false
		}
	}
	return true
}

// Exact homogeneous U rows repeat their XY points and weights at both
// elevations. Thus the V direction is an actual affine line and opposite U
// chains have the same sampling law, even for a clockwise rational arc.
func compactExactVerticalChart(surface geom.Surface) bool {
	s, ok := surface.(*geom.ExactBezierSurface)
	if !ok || s.ExactPatch() == nil {
		return false
	}
	r, err := s.ExactPatch().ExportExactBezierRecipe()
	if err != nil || r.DegreeU < 1 || r.DegreeU > 2 || r.DegreeV != 1 || len(r.Controls) != (r.DegreeU+1)*2 {
		return false
	}
	var levels [2]*big.Rat
	for i := 0; i < len(r.Controls); i += 2 {
		a, b := r.Controls[i], r.Controls[i+1]
		if a[0] != b[0] || a[1] != b[1] || a[3] != b[3] {
			return false
		}
		w, ok := new(big.Rat).SetString(a[3])
		if !ok || w.Sign() <= 0 {
			return false
		}
		for j, c := range [2][4]string{a, b} {
			z, ok := new(big.Rat).SetString(c[2])
			if !ok {
				return false
			}
			z.Quo(z, w)
			if i == 0 {
				levels[j] = z
			} else if z.Cmp(levels[j]) != 0 {
				return false
			}
		}
	}
	return levels[0].Cmp(levels[1]) < 0
}
