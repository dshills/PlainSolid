package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// QuadraticPlaneSpanOptions limits free-axis pieces (default32, maximum128).
// Faces limits apply separately to source verification and each span proof.
// Each span uses at most one exact trim, one coefficient expansion and six
// endpoint evaluations, each with the corresponding Faces limits. Total work
// is bounded by this fixed per-span budget times MaxSpans, plus source
// verification; there is no unreported global operation counter.
type QuadraticPlaneSpanOptions struct {
	MaxSpans int
	Faces    SplineRectangleEmbeddingOptions
}

// QuadraticPlaneKnot is an internal section junction, not an original edge hit.
// Pieces names the two incident ordered section pieces on Face. UV and Point
// are identical exact one-sided endpoints. Returned rationals are caller-owned.
type QuadraticPlaneKnot struct {
	Face, FreeAxis int
	Pieces         [2]int
	Knot           *big.Rat
	UV             [2]*big.Rat
	Point          [3]*big.Rat
}

// PiecewiseQuadraticPlaneFaceSection binds one original face and its complete
// ordered strip decomposition. Only free-axis knots are supported. Every strip
// is polynomial quadratic/free and linear/dependent, with strictly interior
// trim, nonzero consistent dependent derivative, and full-range exact C0 seams.
type PiecewiseQuadraticPlaneFaceSection struct {
	source           Solid
	face, axis, sign int
	patch            *geom.SurfacePatch
	pieces           []ExactQuadraticUV
	domains          []geom.ParameterRectangle
	knots            []QuadraticPlaneKnot
	edges            [2]int
	parameters       [2]*big.Rat
}

func (p *PiecewiseQuadraticPlaneFaceSection) Face() int      { return p.face }
func (p *PiecewiseQuadraticPlaneFaceSection) FreeAxis() int  { return p.axis }
func (p *PiecewiseQuadraticPlaneFaceSection) AboveSign() int { return p.sign }
func (p *PiecewiseQuadraticPlaneFaceSection) Pieces() []ExactQuadraticUV {
	return append([]ExactQuadraticUV(nil), p.pieces...)
}
func (p *PiecewiseQuadraticPlaneFaceSection) Domains() []geom.ParameterRectangle {
	return append([]geom.ParameterRectangle(nil), p.domains...)
}
func (p *PiecewiseQuadraticPlaneFaceSection) Edges() [2]int { return p.edges }
func (p *PiecewiseQuadraticPlaneFaceSection) Parameters() [2]*big.Rat {
	return [2]*big.Rat{new(big.Rat).Set(p.parameters[0]), new(big.Rat).Set(p.parameters[1])}
}
func (p *PiecewiseQuadraticPlaneFaceSection) Knots() []QuadraticPlaneKnot {
	out := append([]QuadraticPlaneKnot(nil), p.knots...)
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
func CertifyPiecewiseQuadraticPlaneFaceSection(ctx context.Context, s Solid, face int, height float64, freeAxis int, o QuadraticPlaneSpanOptions) (*PiecewiseQuadraticPlaneFaceSection, error) {
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
	result := &PiecewiseQuadraticPlaneFaceSection{source: s, face: face, axis: freeAxis, patch: f.patch, edges: [2]int{f.edge[geom.SurfaceEdge(2*freeAxis)], f.edge[geom.SurfaceEdge(2*freeAxis+1)]}}
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
		trim, sign, e := quadraticPlaneTrimAxis(cell, ratInset(height), freeAxis)
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
			left, _ := result.pieces[i-1].Endpoints()
			right, _ := trim.Endpoints()
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
			result.knots = append(result.knots, QuadraticPlaneKnot{Face: face, FreeAxis: freeAxis, Pieces: [2]int{i - 1, i}, Knot: ratInset(cuts[i]), UV: left[1], Point: a.Point})
		}
		result.pieces = append(result.pieces, trim)
		result.domains = append(result.domains, d)
		previous = cell
	}
	first, _ := result.pieces[0].Endpoints()
	last, _ := result.pieces[len(result.pieces)-1].Endpoints()
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

// Both inputs have already proved affine dependence on the non-free axis.
func quadraticPlaneSeam(ctx context.Context, left, right *geom.SurfacePatch, axis int, knot float64, dependent []float64, o geom.SurfacePatchEvaluationOptions) error {
	for _, d := range dependent {
		uv := [2]*big.Rat{ratInset(knot), ratInset(d)}
		if axis == 1 {
			uv[0], uv[1] = uv[1], uv[0]
		}
		a, e := left.EvaluateRational(ctx, uv[0], uv[1], o)
		if e != nil {
			return e
		}
		b, e := right.EvaluateRational(ctx, uv[0], uv[1], o)
		if e != nil {
			return e
		}
		for k := range a.Point {
			if a.Point[k].Cmp(b.Point[k]) != 0 {
				return geom.ErrConvergence
			}
		}
	}
	return nil
}
