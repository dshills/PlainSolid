package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
	"sort"
)

// WrappedCylinderFaceLineHit preserves original unnormalized line units and
// original fundamental source occurrences (including both seam endpoints).
// UV is the retained face lift. Normal is the exact unnormalized oriented
// Du cross Dv; CrossingSign is its dot product with direction, never a solid
// entering/exiting assertion. BoundaryMask1/2 marks finite lower/upper limits.
type WrappedCylinderFaceLineHit struct {
	Parameter     *big.Rat
	UV            [2]*big.Rat
	Point, Normal [3]*big.Rat
	SourceU       []*big.Rat
	SourceSpans   []geom.ParameterSpan
	Location      trimvalidate.Location
	BoundaryMask  uint8
	CrossingSign  int
}
type WrappedCylinderFaceLineIntersection struct {
	Complete                        bool
	Hits                            []WrappedCylinderFaceLineHit
	ExcludedByTrim, ExactOperations int
}

// IntersectHorizontalLine proves complete finite hits for a represented
// horizontal model line on the canonical wrapped cylinder. Height inversion
// must be exactly representable for original spline iso extraction. Full
// supporting-line roots must all be rational and simple; grazing, coincident
// and unrecovered algebraic contacts reject atomically, even outside limits.
// Raw plane construction also must be lossless. Only exact XYZ corroboration,
// exact periodic identity and exact trim location admit hits; no display sample
// or rounded inverse is used. Options bound shared work and source data.
func (r *WrappedQuadraticFace) IntersectHorizontalLine(ctx context.Context, origin, direction geom.Vec3, limits [2]*big.Rat, o geom.SurfacePatchOptions) (WrappedCylinderFaceLineIntersection, error) {
	var zero WrappedCylinderFaceLineIntersection
	if ctx == nil || r == nil || len(r.loops) == 0 || !origin.IsFinite() || !direction.IsFinite() || direction.Z != 0 || direction.X == 0 && direction.Y == 0 || limits[0] == nil || limits[1] == nil || limits[0].Cmp(limits[1]) > 0 {
		return zero, geom.ErrInvalid
	}
	w, ok := r.native.Face.Surface.(*geom.WrappedSurfacePatch)
	if !ok {
		return zero, geom.ErrConvergence
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 8000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxPoles == 0 {
		o.MaxPoles = 4096
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 || o.MaxPoles < 4 {
		return zero, geom.ErrInvalid
	}
	used := 0
	check := func(q *big.Rat) error {
		used++
		if used > o.MaxOperations || q.Num().BitLen() > o.MaxCoefficientBits || q.Denom().BitLen() > o.MaxCoefficientBits {
			return geom.ErrConvergence
		}
		return ctx.Err()
	}
	ori := [3]*big.Rat{ratInset(origin.X), ratInset(origin.Y), ratInset(origin.Z)}
	dir := [3]*big.Rat{ratInset(direction.X), ratInset(direction.Y), new(big.Rat)}
	for _, list := range [][]*big.Rat{ori[:], dir[:], limits[:]} {
		for _, q := range list {
			if e := check(q); e != nil {
				return zero, e
			}
		}
	}
	if used >= o.MaxOperations {
		return zero, geom.ErrConvergence
	}
	child := o
	child.MaxOperations -= used
	embedding, e := geom.CertifyWrappedCylinderEmbedding(ctx, w, child)
	if e != nil {
		return zero, e
	}
	if !embedding.Complete {
		return zero, geom.ErrConvergence
	}
	used += embedding.ExactOperations
	if ori[2].Cmp(embedding.LowerZ) < 0 || ori[2].Cmp(embedding.UpperZ) > 0 {
		if e := ctx.Err(); e != nil {
			return zero, e
		}
		return WrappedCylinderFaceLineIntersection{Complete: true, ExactOperations: used}, nil
	}
	d := w.Domain()
	v := new(big.Rat).Add(ratInset(d.VMin), new(big.Rat).Mul(new(big.Rat).Sub(ratInset(d.VMax), ratInset(d.VMin)), new(big.Rat).Quo(new(big.Rat).Sub(ori[2], embedding.LowerZ), new(big.Rat).Sub(embedding.UpperZ, embedding.LowerZ))))
	if e := check(v); e != nil {
		return zero, e
	}
	fixed, e := geom.RepresentedExactScalar(v)
	if e != nil {
		return zero, e
	}
	if used >= o.MaxOperations {
		return zero, geom.ErrConvergence
	}
	child = o
	child.MaxOperations -= used
	iso, e := w.FundamentalPatch().IsoCurve(1, fixed, child)
	if e != nil {
		return zero, e
	}
	used += iso.ExactOperations()
	var bx [3]*big.Rat
	for axis := 0; axis < 3; axis++ {
		bx[axis] = new(big.Rat).Add(ori[axis], dir[axis])
		if e := check(bx[axis]); e != nil {
			return zero, e
		}
	}
	b, e := geom.RepresentedExactPoint(bx)
	if e != nil {
		return zero, e
	}
	cx := [3]*big.Rat{new(big.Rat).Set(ori[0]), new(big.Rat).Set(ori[1]), new(big.Rat).Add(ori[2], big.NewRat(1, 1))}
	c, e := geom.RepresentedExactPoint(cx)
	if e != nil {
		return zero, e
	}
	if used >= o.MaxOperations {
		return zero, geom.ErrConvergence
	}
	roots, e := iso.IntersectRawPlane(ctx, origin, b, c, geom.SplinePlaneOptions{MaxOperations: o.MaxOperations - used, MaxCoefficientBits: o.MaxCoefficientBits, MaxControlPoints: o.MaxPoles})
	if e != nil {
		return zero, e
	}
	used += roots.Operations
	if !roots.Complete || len(roots.Coincident) != 0 {
		return zero, geom.ErrConvergence
	}
	pivot := 0
	if dir[0].Sign() == 0 {
		pivot = 1
	}
	period := w.Periods()[0]
	type candidate struct {
		u, t   *big.Rat
		point  [3]*big.Rat
		source []*big.Rat
		spans  []geom.ParameterSpan
	}
	var candidates []candidate
	for _, h := range roots.Hits {
		if h.Parameter == nil || h.Point[0] == nil || h.Multiplicity != 1 || h.Tangent {
			return zero, geom.ErrConvergence
		}
		t := new(big.Rat).Quo(new(big.Rat).Sub(h.Point[pivot], ori[pivot]), dir[pivot])
		if e := check(t); e != nil {
			return zero, e
		}
		for axis := 0; axis < 3; axis++ {
			p := new(big.Rat).Add(ori[axis], new(big.Rat).Mul(t, dir[axis]))
			if e := check(p); e != nil {
				return zero, e
			}
			if p.Cmp(h.Point[axis]) != 0 {
				return zero, geom.ErrConvergence
			}
		}
		u := new(big.Rat).Set(h.Parameter)
		if u.Cmp(ratInset(d.UMax)) == 0 {
			u.SetFloat64(d.UMin)
		}
		found := -1
		for i, p := range candidates {
			if p.t.Cmp(t) == 0 {
				delta := new(big.Rat).Quo(new(big.Rat).Sub(h.Parameter, p.u), period)
				if e := check(delta); e != nil {
					return zero, e
				}
				if !delta.IsInt() || u.Cmp(p.u) != 0 {
					return zero, geom.ErrConvergence
				}
				for axis := 0; axis < 3; axis++ {
					if p.point[axis].Cmp(h.Point[axis]) != 0 {
						return zero, geom.ErrConvergence
					}
				}
				found = i
				break
			}
		}
		if found < 0 {
			candidates = append(candidates, candidate{u: u, t: t, point: h.Point})
			found = len(candidates) - 1
		}
		p := &candidates[found]
		duplicate := false
		for _, x := range p.source {
			duplicate = duplicate || x.Cmp(h.Parameter) == 0
		}
		if !duplicate {
			p.source = append(p.source, new(big.Rat).Set(h.Parameter))
		}
		p.spans = append(p.spans, h.Span)
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].t.Cmp(candidates[j].t) < 0 })
	result := WrappedCylinderFaceLineIntersection{}
	for _, p := range candidates {
		if p.t.Cmp(limits[0]) < 0 || p.t.Cmp(limits[1]) > 0 {
			continue
		}
		if used >= o.MaxOperations {
			return zero, geom.ErrConvergence
		}
		child = o
		child.MaxOperations -= used
		location, e := r.ClassifyCylinderRationalPoint(ctx, p.point, [2]*big.Rat{p.u, v}, child)
		if e != nil {
			return zero, e
		}
		used += location.ExactOperations
		if location.Location == trimvalidate.Outside {
			result.ExcludedByTrim++
			continue
		}
		if !location.MatchedLift || location.Location == trimvalidate.Unknown {
			return zero, geom.ErrConvergence
		}
		if used >= o.MaxOperations {
			return zero, geom.ErrConvergence
		}
		jet, e := w.EvaluateRational(ctx, location.FaceUV[0], location.FaceUV[1], geom.SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations - used, MaxCoefficientBits: o.MaxCoefficientBits})
		if e != nil {
			return zero, e
		}
		used += jet.ExactOperations
		hit := WrappedCylinderFaceLineHit{Parameter: p.t, UV: location.FaceUV, Point: p.point, SourceU: p.source, SourceSpans: p.spans, Location: location.Location}
		dot := new(big.Rat)
		for axis := 0; axis < 3; axis++ {
			j, k := (axis+1)%3, (axis+2)%3
			normal := new(big.Rat).Sub(new(big.Rat).Mul(jet.Du[j], jet.Dv[k]), new(big.Rat).Mul(jet.Du[k], jet.Dv[j]))
			if r.native.Face.Reversed {
				normal.Neg(normal)
			}
			if e := check(normal); e != nil {
				return zero, e
			}
			hit.Normal[axis] = normal
			dot.Add(dot, new(big.Rat).Mul(normal, dir[axis]))
		}
		if e := check(dot); e != nil {
			return zero, e
		}
		if dot.Sign() == 0 {
			return zero, geom.ErrConvergence
		}
		hit.CrossingSign = dot.Sign()
		if p.t.Cmp(limits[0]) == 0 {
			hit.BoundaryMask |= 1
		}
		if p.t.Cmp(limits[1]) == 0 {
			hit.BoundaryMask |= 2
		}
		result.Hits = append(result.Hits, hit)
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	result.Complete = true
	result.ExactOperations = used
	return result, nil
}
