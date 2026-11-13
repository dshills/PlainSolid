package trimmed

import (
	"errors"
	"math"
	"math/big"

	"plainsolid/geom"
	"plainsolid/intersect"
)

// The wire is already simple by complete pair coverage and embedding of both
// shells. For this proof of its original UV orientation, enclose the Green
// integral ∮(u-u_ref)dv. On one subinterval evaluate Δv in exact endpoint
// boxes, then bound the omitted integral by max|u-u_start| times the variation
// bound on v. Neither enclosure sample nor chord stands in for the curve.
func (w *shellImprintWork) closedPathWinding(face, op int, path shellImprintPath, s AlgebraicShellImprint) (int, error) {
	type piece struct {
		segment   intersect.TubeSegment
		domain    [2]float64
		variation *big.Rat
		reversed  bool
	}
	var curves []piece
	for _, use := range path.uses {
		j := s.Section[use.SectionPiece]
		b := w.source.branches[j.Branch]
		if b.Faces[op] != face || w.exact.MaxOperations < 1 {
			return 0, geom.ErrConvergence
		}
		x := b.Segments[j.Index]
		signs, e := x.ParameterTraversalSigns(w.ctx, geom.SurfacePatchOptions{
			MaxOperations: w.exact.MaxOperations, MaxCoefficientBits: w.exact.MaxCoefficientBits})
		w.exact.MaxOperations -= signs.ExactOperations
		if e != nil {
			return 0, e
		}
		axis := 2*op + 1
		if !signs.HasRange[axis] {
			return 0, geom.ErrConvergence
		}
		d := signs.Ranges[axis]
		if !finite(d.Min) || !finite(d.Max) {
			return 0, geom.ErrConvergence
		}
		_, domain, reverse, e := x.ExactFreeDomain()
		if e != nil {
			return 0, e
		}
		lo, e := geom.RepresentedExactScalar(domain.Lower)
		if e != nil {
			return 0, e
		}
		hi, e := geom.RepresentedExactScalar(domain.Upper)
		if e != nil {
			return 0, e
		}
		variation := ratInset(max(math.Abs(d.Min), math.Abs(d.Max)))
		curves = append(curves, piece{x, [2]float64{lo, hi}, variation, use.Reversed != reverse})
	}
	original := [2]Solid{w.source.a, w.source.b}[op].faces[face]
	domain, e := faceUVBounds(original)
	if e != nil {
		return 0, e
	}
	uRef := ratInset(domain.UMin)
	for n := 1; n <= 64 && len(curves)*n <= 16384; n *= 2 {
		lo, hi := new(big.Rat), new(big.Rat)
		for _, c := range curves {
			if e := w.ctx.Err(); e != nil {
				return 0, e
			}
			if c.variation.Sign() == 0 {
				continue
			}
			axisBox := func(x [2]geom.UVBox) geom.UVBox { return x[op] }
			evaluate := func(x float64) (geom.UVBox, error) {
				if w.exact.MaxOperations < 1 {
					return geom.UVBox{}, geom.ErrConvergence
				}
				q, e := c.segment.EncloseAtFree(x, geom.SurfaceTubeEvaluationOptions{
					MaxOperations: w.exact.MaxOperations, MaxCoefficientBits: w.exact.MaxCoefficientBits})
				w.exact.MaxOperations -= q.ExactOperations
				return axisBox([2]geom.UVBox{q.ParametersA, q.ParametersB}), e
			}
			a := c.domain[0]
			start, e := evaluate(a)
			if e != nil {
				return 0, e
			}
			for i := range n {
				if w.exact.MaxOperations < 1 {
					return 0, geom.ErrConvergence
				}
				// A contraction budget may leave a valid outward box. The
				// sign test uses only that box, never its convergence flag.
				t := float64(i+1) / float64(n)
				b := (1-t)*c.domain[0] + t*c.domain[1]
				if i == n-1 {
					b = c.domain[1]
				}
				if b <= a || b > c.domain[1] {
					return 0, geom.ErrConvergence
				}
				enclosure, e := c.segment.EncloseFreeInterval(w.ctx, a, b, geom.SurfaceTubeIntervalOptions{
					MaxIterations: 4, MaxOperations: w.exact.MaxOperations, MaxCoefficientBits: w.exact.MaxCoefficientBits})
				w.exact.MaxOperations -= enclosure.ExactOperations
				if !enclosure.HasBounds || e != nil && !errors.Is(e, geom.ErrConvergence) {
					if e == nil {
						e = geom.ErrConvergence
					}
					return 0, e
				}
				box := axisBox([2]geom.UVBox{enclosure.ParametersA, enclosure.ParametersB})
				finish, e := evaluate(b)
				if e != nil {
					return 0, e
				}
				ul := new(big.Rat).Sub(ratInset(start.Min.X), uRef)
				uh := new(big.Rat).Sub(ratInset(start.Max.X), uRef)
				dvl := new(big.Rat).Sub(ratInset(finish.Min.Y), ratInset(start.Max.Y))
				dvh := new(big.Rat).Sub(ratInset(finish.Max.Y), ratInset(start.Min.Y))
				z := [4]*big.Rat{new(big.Rat).Mul(ul, dvl), new(big.Rat).Mul(ul, dvh), new(big.Rat).Mul(uh, dvl), new(big.Rat).Mul(uh, dvh)}
				minZ, maxZ := z[0], z[0]
				for _, v := range z[1:] {
					minZ, maxZ = minRat(minZ, v), maxRat(maxZ, v)
				}
				ul = new(big.Rat).Sub(ratInset(box.Min.X), ratInset(start.Max.X))
				uh = new(big.Rat).Sub(ratInset(box.Max.X), ratInset(start.Min.X))
				variation := new(big.Rat).Mul(c.variation, new(big.Rat).Sub(ratInset(b), ratInset(a)))
				remainder := new(big.Rat).Mul(maxRat(ul.Abs(ul), uh.Abs(uh)), variation)
				minZ, maxZ = new(big.Rat).Sub(minZ, remainder), new(big.Rat).Add(maxZ, remainder)
				if c.reversed {
					minZ, maxZ = maxZ.Neg(maxZ), minZ.Neg(minZ)
				}
				lo.Add(lo, minZ)
				hi.Add(hi, maxZ)
				start = finish
				a = b
			}
		}
		if lo.Sign() > 0 {
			return 1, nil
		}
		if hi.Sign() < 0 {
			return -1, nil
		}
	}
	return 0, geom.ErrConvergence
}
