package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// PolynomialPlaneSpanOptions limits free-axis pieces (default32, maximum128).
// Faces limits apply separately to source verification and each span proof.
// Each span uses at most one exact trim, one coefficient expansion and six
// endpoint evaluations, each with the corresponding Faces limits. Total work
// is bounded by this fixed per-span budget times MaxSpans, plus source
// verification; there is no unreported global operation counter.
type PolynomialPlaneSpanOptions struct {
	MaxSpans int
	Faces    SplineRectangleEmbeddingOptions
}

// PolynomialPlaneKnot retains original face, free-axis span ordinals and exact
// shared UV/model point. Its layout matches the quadratic provenance contract.
type PolynomialPlaneKnot = QuadraticPlaneKnot

// PiecewisePolynomialPlaneFaceSection binds one original face and its complete
// ordered strip decomposition. Only free-axis knots are supported. Every strip
// is polynomial degree≤4/free and linear/dependent, with strictly interior
// trim, nonzero consistent dependent derivative, and full-range exact C0 seams.
type PiecewisePolynomialPlaneFaceSection struct {
	source           Solid
	face, axis, sign int
	patch            *geom.SurfacePatch
	pieces           []SplineUV
	domains          []geom.ParameterRectangle
	knots            []PolynomialPlaneKnot
	edges            [2]int
	parameters       [2]*big.Rat
}

func (p *PiecewisePolynomialPlaneFaceSection) Face() int      { return p.face }
func (p *PiecewisePolynomialPlaneFaceSection) FreeAxis() int  { return p.axis }
func (p *PiecewisePolynomialPlaneFaceSection) AboveSign() int { return p.sign }
func (p *PiecewisePolynomialPlaneFaceSection) Pieces() []SplineUV {
	return append([]SplineUV(nil), p.pieces...)
}
func (p *PiecewisePolynomialPlaneFaceSection) Domains() []geom.ParameterRectangle {
	return append([]geom.ParameterRectangle(nil), p.domains...)
}
func (p *PiecewisePolynomialPlaneFaceSection) Edges() [2]int { return p.edges }
func (p *PiecewisePolynomialPlaneFaceSection) Parameters() [2]*big.Rat {
	return [2]*big.Rat{new(big.Rat).Set(p.parameters[0]), new(big.Rat).Set(p.parameters[1])}
}
func (p *PiecewisePolynomialPlaneFaceSection) Knots() []PolynomialPlaneKnot {
	out := append([]PolynomialPlaneKnot(nil), p.knots...)
	for i := range out {
		out[i].Knot = new(big.Rat).Set(out[i].Knot)
		for j := range out[i].UV {
			out[i].UV[j] = new(big.Rat).Set(out[i].UV[j])
		}
		for j := range out[i].Point {
			out[i].Point[j] = new(big.Rat).Set(out[i].Point[j])
		}
	}
	return out
}
func CertifyPiecewisePolynomialPlaneFaceSection(ctx context.Context, s Solid, face int, height float64, freeAxis int, o PolynomialPlaneSpanOptions) (*PiecewisePolynomialPlaneFaceSection, error) {
	if ctx == nil || face < 0 || face >= len(s.faces) || !finite(height) || freeAxis < 0 || freeAxis > 1 {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	if o.MaxSpans == 0 {
		o.MaxSpans = 32
	}
	if o.MaxSpans < 1 || o.MaxSpans > 128 {
		return nil, geom.ErrInvalid
	}
	f, _, e := s.splineEmbeddingFace(ctx, s.faces[face], o.Faces)
	if e != nil {
		return nil, e
	}
	cuts, other := f.patch.KnotBreaks()
	if freeAxis == 1 {
		cuts, other = other, cuts
	}
	if len(other) != 2 || len(cuts) < 2 || len(cuts)-1 > o.MaxSpans {
		return nil, geom.ErrConvergence
	}
	result := &PiecewisePolynomialPlaneFaceSection{source: s, face: face, axis: freeAxis, patch: f.patch, edges: [2]int{f.edge[geom.SurfaceEdge(2*freeAxis)], f.edge[geom.SurfaceEdge(2*freeAxis+1)]}}
	evaluation := geom.SurfacePatchEvaluationOptions{MaxOperations: o.Faces.Patches.MaxOperations, MaxCoefficientBits: o.Faces.Patches.MaxCoefficientBits}
	var previous *geom.SurfacePatch
	for i := 0; i < len(cuts)-1; i++ {
		if e = ctx.Err(); e != nil {
			return nil, e
		}
		d := f.patch.Domain()
		if freeAxis == 0 {
			d.UMin, d.UMax = cuts[i], cuts[i+1]
		} else {
			d.VMin, d.VMax = cuts[i], cuts[i+1]
		}
		cell, e := f.patch.Trim(d)
		if e != nil {
			return nil, e
		}
		trim, sign, e := polynomialPlaneTrimAxis(cell, ratInset(height), freeAxis)
		if e != nil {
			return nil, e
		}
		if i == 0 {
			result.sign = sign
		} else if sign != result.sign {
			return nil, geom.ErrConvergence
		}
		if previous != nil {
			// Every coordinate of both one-sided seam curves is affine in the
			// dependent coordinate. Exact equality at both ends proves the WHOLE seam.
			if e = quadraticPlaneSeam(ctx, previous, cell, freeAxis, cuts[i], other, evaluation); e != nil {
				return nil, e
			}
			left, _, e := polynomialPlaneEndpoints(result.pieces[i-1])
			if e != nil {
				return nil, e
			}
			right, _, e := polynomialPlaneEndpoints(trim)
			if e != nil {
				return nil, e
			}
			for axis := range left[1] {
				if left[1][axis].Cmp(right[0][axis]) != 0 {
					return nil, geom.ErrConvergence
				}
			}
			a, e := previous.EvaluateRational(ctx, left[1][0], left[1][1], evaluation)
			if e != nil {
				return nil, e
			}
			b, e := cell.EvaluateRational(ctx, right[0][0], right[0][1], evaluation)
			if e != nil {
				return nil, e
			}
			for axis := range a.Point {
				if a.Point[axis].Cmp(b.Point[axis]) != 0 {
					return nil, geom.ErrConvergence
				}
				if _, e = geom.RepresentedExactScalar(a.Point[axis]); e != nil {
					return nil, e
				}
			}
			result.knots = append(result.knots, PolynomialPlaneKnot{Face: face, FreeAxis: freeAxis, Pieces: [2]int{i - 1, i}, Knot: ratInset(cuts[i]), UV: left[1], Point: a.Point})
		}
		result.pieces = append(result.pieces, trim)
		result.domains = append(result.domains, d)
		previous = cell
	}
	first, _, e := polynomialPlaneEndpoints(result.pieces[0])
	if e != nil {
		return nil, e
	}
	last, _, e := polynomialPlaneEndpoints(result.pieces[len(result.pieces)-1])
	if e != nil {
		return nil, e
	}
	ends := [2][2]*big.Rat{first[0], last[1]}
	for i, id := range result.edges {
		var use *Use
		for j := range s.faces[face].Outer {
			if s.faces[face].Outer[j].Edge == id {
				use = &s.faces[face].Outer[j]
			}
		}
		if use == nil {
			return nil, geom.ErrConvergence
		}
		start, end := use.UVStart.Y, use.UVEnd.Y
		if freeAxis == 1 {
			start, end = use.UVStart.X, use.UVEnd.X
		}
		if start == end {
			return nil, geom.ErrConvergence
		}
		alpha := new(big.Rat).Quo(new(big.Rat).Sub(ends[i][1-freeAxis], ratInset(start)), new(big.Rat).Sub(ratInset(end), ratInset(start)))
		if alpha.Sign() <= 0 || alpha.Cmp(big.NewRat(1, 1)) >= 0 {
			return nil, geom.ErrConvergence
		}
		if use.Reversed {
			alpha.Sub(big.NewRat(1, 1), alpha)
		}
		edge := s.edges[id]
		result.parameters[i] = new(big.Rat).Add(ratInset(edge.Range.Min), new(big.Rat).Mul(alpha, new(big.Rat).Sub(ratInset(edge.Range.Max), ratInset(edge.Range.Min))))
	}
	return result, nil
}
