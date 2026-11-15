package trimmed

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

// TrimLoopReport lists UV topology certificates in face order. Complete
// means every face is certified. Model-space embedding and continuous edge
// agreement are separate from this proof.
type TrimLoopReport struct {
	Faces    []trimvalidate.Report
	Complete bool
}

// SplineFaceRegularity reports a regularity certificate for the rectangular
// domain covering one spline face. Unknown supports remain unproved.
type SplineFaceRegularity struct {
	Face      int
	Supported bool
	Proof     geom.SurfaceRegularity
}

// ValidateSplineRegularity proves one-sided regularity over each supported
// spline or exact Bézier face's conservative UV rectangle, with work limits per face.
// Singularities outside a trimmed region can make the result inconclusive.
// It does not certify global surface or shell embedding.
func (s Solid) ValidateSplineRegularity(ctx context.Context, patch geom.SurfacePatchOptions, options geom.SurfaceRegularityOptions) ([]SplineFaceRegularity, error) {
	if ctx == nil {
		return nil, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return nil, e
	}
	r := make([]SplineFaceRegularity, 0, len(s.faces))
	for i, f := range s.faces {
		if e := ctx.Err(); e != nil {
			return r, e
		}
		base, _ := parametric.SurfaceMapping(f.Surface)
		if !supportsExactSplinePatch(base) {
			r = append(r, SplineFaceRegularity{Face: i})
			continue
		}
		p, e := exactFaceSupportPatch(f, patch)
		if e != nil {
			return r, e
		}
		d, e := faceUVBounds(f)
		if e != nil {
			return r, e
		}
		p, e = p.Trim(d)
		if e != nil {
			return r, e
		}
		q, e := geom.CertifySurfaceRegularity(p, options)
		r = append(r, SplineFaceRegularity{i, true, q})
		if e != nil {
			return r, e
		}
	}
	return r, nil
}

// ValidateTrimLoops proves p-curve winding, disjointness and hole containment
// for lines and continuous clamped rational splines. Degree one and two
// spans are supported directly; degrees three through eight require a
// proved monotone chord projection per span. Other cases are unresolved.
func (s Solid) ValidateTrimLoops(ctx context.Context, o trimvalidate.Options) (TrimLoopReport, error) {
	r := TrimLoopReport{Complete: true}
	if ctx == nil {
		return TrimLoopReport{}, fmt.Errorf("trimmed: nil validation context")
	}
	if e := ctx.Err(); e != nil {
		return TrimLoopReport{}, e
	}
	for _, f := range s.faces {
		v, e := certifyFaceLoops(ctx, f, o)
		if e != nil {
			r.Complete = false
			return r, e
		}
		r.Faces = append(r.Faces, v)
		r.Complete = r.Complete && v.Status == trimvalidate.Certified
	}
	return r, nil
}

func certifyFaceLoops(ctx context.Context, f Face, o trimvalidate.Options) (trimvalidate.Report, error) {
	return ValidateFaceTrimLoops(ctx, f, o)
}

// ValidateFaceTrimLoops proves only UV topology. It uses exact homogeneous
// pieces for supported spline ranges and maps reported indices back to the
// original topological uses. MaxEdges counts proof pieces, not just uses;
// MaxWork includes exact spline subdivision.
func ValidateFaceTrimLoops(ctx context.Context, f Face, options trimvalidate.Options) (trimvalidate.Report, error) {
	return validateFaceTrimLoops(ctx, f, options, nil, nil)
}

// LocateInFaceTrim identifies material in the face's UV chart after proving
// its trim topology. This does not invert the model surface or certify its
// embedding. Higher-degree boundary membership can remain unresolved.
func LocateInFaceTrim(ctx context.Context, f Face, uv geom.Vec2, options trimvalidate.Options) (trimvalidate.Location, trimvalidate.Report, error) {
	if !uvFinite(uv) {
		return trimvalidate.Unknown, trimvalidate.Report{}, geom.ErrInvalid
	}
	return LocateRationalInFaceTrim(ctx, f, [2]*big.Rat{ratInset(uv.X), ratInset(uv.Y)}, options)
}

// LocateRationalInFaceTrim classifies exact original UV coordinates.
func LocateRationalInFaceTrim(ctx context.Context, f Face, uv [2]*big.Rat, options trimvalidate.Options) (trimvalidate.Location, trimvalidate.Report, error) {
	var location trimvalidate.Location
	proof, e := validateFaceTrimLoops(ctx, f, options, &uv, &location)
	return location, proof, e
}

// LocateRationalBoxInFaceTrim proves every point of a closed exact UV
// enclosure belongs to the same side of the original certified trim loops.
func LocateRationalBoxInFaceTrim(ctx context.Context, f Face, lo, hi [2]*big.Rat, options trimvalidate.Options) (trimvalidate.Location, trimvalidate.Report, error) {
	var location trimvalidate.Location
	proof, e := validateFaceTrimLoops(ctx, f, options, nil, &location, [2][2]*big.Rat{lo, hi})
	return location, proof, e
}

func validateFaceTrimLoops(ctx context.Context, f Face, options trimvalidate.Options, query *[2]*big.Rat, location *trimvalidate.Location, boxes ...[2][2]*big.Rat) (trimvalidate.Report, error) {
	return proveFaceTrimLoops(ctx, f, options, nil, query, location, boxes...)
}

func proveFaceTrimLoops(ctx context.Context, f Face, options trimvalidate.Options, retain **trimvalidate.CertifiedLoops, query *[2]*big.Rat, location *trimvalidate.Location, boxes ...[2][2]*big.Rat) (trimvalidate.Report, error) {
	r := trimvalidate.Report{LoopA: -1, EdgeA: -1, LoopB: -1, EdgeB: -1}
	o, e := options.WithDefaults()
	if ctx == nil || e != nil {
		return r, trimvalidate.ErrOptions
	}
	if e = ctx.Err(); e != nil {
		return r, e
	}
	loops := make([]trimvalidate.Loop, 0, 1+len(f.Holes))
	indices := make([][]int, 0, 1+len(f.Holes))
	used := 0
	for i, l := range append([][]Use{f.Outer}, f.Holes...) {
		uv := make(trimvalidate.Loop, 0, min(len(l), o.MaxEdges-used))
		var index []int
		for j, u := range l {
			r.LoopA, r.EdgeA = i, j
			if e = ctx.Err(); e != nil {
				return r, e
			}
			if used >= o.MaxEdges || r.Work >= o.MaxWork {
				r.Reason = "trim conversion budget exhausted"
				return r, nil
			}
			if !uvFinite(u.UVStart) || !uvFinite(u.UVEnd) || u.UVEnd != l[(j+1)%len(l)].UVStart {
				r.Status, r.Reason = trimvalidate.Invalid, "nonfinite or disconnected UV endpoints"
				return r, nil
			}
			v, ok := TrimValidationSegment(u)
			if !ok {
				r.Status, r.Reason = trimvalidate.Invalid, "trim endpoints disagree"
				return r, nil
			}
			pieces := trimvalidate.Loop{v}
			if v.Kind == trimvalidate.UnsupportedCurve {
				c, supported := shortSplineTrim(u.Curve)
				if !supported {
					r.Reason = "unsupported curve representation"
					return r, nil
				}
				var work int
				pieces, work, e = exactTrimPieces(ctx, u, c, o.MaxEdges-used, o.MaxWork-r.Work, o.MaxBits)
				r.Work += work
				if e != nil {
					if e = ctx.Err(); e != nil {
						return r, e
					}
					r.Reason = "unsupported spline range, inexact trim endpoints or exhausted conversion budget"
					return r, nil
				}
			}
			uv = append(uv, pieces...)
			for range pieces {
				index = append(index, j)
			}
			used += len(pieces)
		}
		loops = append(loops, uv)
		indices = append(indices, index)
	}
	if r.Work >= o.MaxWork {
		r.Reason = "trim conversion budget exhausted"
		return r, nil
	}
	if len(loops[0]) == 0 {
		r.Status, r.Reason = trimvalidate.Invalid, "empty outer trim loop"
		return r, nil
	}
	o.MaxWork -= r.Work
	var proof trimvalidate.Report
	if len(boxes) == 1 {
		*location, proof, e = trimvalidate.LocateRationalBoxInLoops(ctx, loops[0], loops[1:], boxes[0][0], boxes[0][1], o)
	} else if retain != nil {
		*retain, proof, e = trimvalidate.CertifyLoops(ctx, loops[0], loops[1:], o)
	} else if query == nil {
		proof, e = trimvalidate.ValidateLoops(ctx, loops[0], loops[1:], o)
	} else {
		*location, proof, e = trimvalidate.LocateRationalInLoops(ctx, loops[0], loops[1:], *query, o)
	}
	proof.Work += r.Work
	for _, pair := range [][2]*int{{&proof.LoopA, &proof.EdgeA}, {&proof.LoopB, &proof.EdgeB}} {
		i, j := *pair[0], *pair[1]
		if i >= 0 && i < len(indices) && j >= 0 && j < len(indices[i]) {
			*pair[1] = indices[i][j]
		}
	}
	return proof, e
}

func shortSplineTrim(p PCurve) (SplineUV, bool) {
	if q, ok := p.(*SplineUV); ok && q != nil {
		p = *q
	}
	c, ok := p.(SplineUV)
	return c, ok && c.Curve != nil && c.Curve.Degree() >= 1 && c.Curve.Degree() <= 8
}

func supportsTrimProof(u Use) bool {
	if v, ok := TrimValidationSegment(u); !ok || v.Kind != trimvalidate.UnsupportedCurve {
		return ok
	}
	c, ok := shortSplineTrim(u.Curve)
	if !ok {
		return false
	}
	a, b := c.Curve.Domain()
	knots := c.Curve.Knots()
	d := c.Curve.Degree()
	if len(knots) < 2*(d+1) || c.Range.Min < a || c.Range.Max > b || c.Range.Min >= c.Range.Max {
		return false
	}
	for i, x := range knots {
		if i <= d && x != a || i >= len(knots)-d-1 && x != b ||
			i > d && i < len(knots)-d-1 && x == knots[i-d] {
			return false
		}
	}
	return true
}

func exactTrimPieces(ctx context.Context, u Use, c SplineUV, count, work, bits int) (trimvalidate.Loop, int, error) {
	if bits < 64 {
		return nil, 0, trimvalidate.ErrLimit
	}
	spans, used, e := c.Curve.ExactBezierSpans(ctx, c.Range.Min, c.Range.Max, geom.ExactBezierSpanOptions{
		SurfacePatchOptions: geom.SurfacePatchOptions{MaxOperations: work, MaxCoefficientBits: bits},
		MaxSpans:            count,
	})
	if e != nil {
		if errors.Is(e, geom.ErrConvergence) {
			return nil, min(used, work), trimvalidate.ErrLimit
		}
		return nil, used, e
	}
	type hpoint = [3]*big.Rat
	var result trimvalidate.Loop
	var first, last hpoint
	for i := range spans {
		k := i
		if c.Reversed {
			k = len(spans) - i - 1
		}
		span := spans[k]
		at := func(i int) (hpoint, bool) {
			if c.Reversed {
				i = len(span.Control) - i - 1
			}
			h := span.Control[i]
			return hpoint{h[0], h[1], h[3]}, h[2].Sign() == 0
		}
		a, aok := at(0)
		z, zok := at(len(span.Control) - 1)
		if !aok || !zok {
			return nil, used, geom.ErrInvalid
		}
		h := make([]hpoint, max(3, len(span.Control)))
		h[0], h[len(h)-1] = a, z
		if len(span.Control) == 2 {
			for j := range h[1] {
				h[1][j] = new(big.Rat).Quo(new(big.Rat).Add(a[j], z[j]), big.NewRat(2, 1))
			}
		} else {
			for i := 1; i+1 < len(h); i++ {
				var ok bool
				h[i], ok = at(i)
				if !ok {
					return nil, used, geom.ErrInvalid
				}
			}
		}
		v, e := trimvalidate.ExactBezier(h)
		if e != nil {
			return nil, used, e
		}
		if i == 0 {
			first = a
		}
		last = z
		result = append(result, v)
	}
	sameEnd := func(h hpoint, p geom.Vec2) bool {
		return new(big.Rat).Mul(h[2], new(big.Rat).SetFloat64(p.X)).Cmp(h[0]) == 0 &&
			new(big.Rat).Mul(h[2], new(big.Rat).SetFloat64(p.Y)).Cmp(h[1]) == 0
	}
	if len(result) == 0 || !sameEnd(first, u.UVStart) || !sameEnd(last, u.UVEnd) {
		return nil, used, geom.ErrUnrepresentable
	}
	return result, used, nil
}

// TrimValidationSegment converts a model edge use to the exact UV proof
// representation. The boolean is false on inconsistent endpoints. Curves
// outside the supported representations return UnsupportedCurve.
func TrimValidationSegment(u Use) (trimvalidate.Segment, bool) {
	v := trimvalidate.Segment{Start: u.UVStart, End: u.UVEnd}
	switch c := u.Curve.(type) {
	case MappedSplineUV:
		u.Curve = c.curve
		return TrimValidationSegment(u)
	case nil:
	case QuadraticUV:
		if c.Start != u.UVStart || c.End != u.UVEnd {
			return v, false
		}
		v.Kind, v.Control = trimvalidate.Quadratic, c.Control
	case ExactQuadraticUV:
		if c.ends != ([2]geom.Vec2{u.UVStart, u.UVEnd}) {
			return v, false
		}
		q, e := trimvalidate.ExactQuadratic(c.control)
		return q, e == nil
	case ExactBezierUV:
		if c.ends != ([2]geom.Vec2{u.UVStart, u.UVEnd}) {
			return v, false
		}
		q, e := trimvalidate.ExactBezier(c.control)
		return q, e == nil
	case *QuadraticUV:
		if c == nil {
			return v, false
		}
		u.Curve = *c
		return TrimValidationSegment(u)
	case *SplineUV:
		if c == nil {
			return v, false
		}
		u.Curve = *c
		return TrimValidationSegment(u)
	case SplineUV:
		v.Kind = trimvalidate.UnsupportedCurve
		if c.Curve == nil || c.Curve.Degree() != 2 {
			break
		}
		a, b := c.Curve.Domain()
		if c.Range.Min != a || c.Range.Max != b {
			break
		}
		k := c.Curve.Knots()
		if len(k) != 6 || k[0] != a || k[1] != a || k[2] != a || k[3] != b || k[4] != b || k[5] != b {
			break
		}
		p, w := c.Curve.Poles(), c.Curve.Weights()
		if len(p) != 3 || len(w) != 3 {
			break
		}
		if c.Reversed {
			p[0], p[2], w[0], w[2] = p[2], p[0], w[2], w[0]
		}
		point := func(p geom.Vec3) geom.Vec2 { return geom.Vec2{X: p.X, Y: p.Y} }
		if p[0].Z != 0 || p[1].Z != 0 || p[2].Z != 0 {
			break
		}
		if point(p[0]) != u.UVStart || point(p[2]) != u.UVEnd {
			// Rational evaluation may round the clamped endpoints slightly.
			// Such trims cannot use the exact shared-endpoint proof.
			break
		}
		v.Kind, v.Control, v.Weights = trimvalidate.RationalQuadratic, point(p[1]), [3]float64{w[0], w[1], w[2]}
	default:
		v.Kind = trimvalidate.UnsupportedCurve
	}
	return v, true
}
