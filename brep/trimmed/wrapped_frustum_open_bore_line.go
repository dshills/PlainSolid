package trimmed

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/geom"
)

// IntersectWrappedCircularFrustumOpenBoreLine returns contacts with the
// native bore boundary in the supplied finite, unnormalized line parameter.
// Faces use canonical IDs (outer wall, lower end, upper end, inner wall,
// floor). In the shared cap plane the disk interior is omitted, and the
// annular overlap keeps both rim owners. Distinct uncertain root enclosures
// must separate before the result is accepted.
func IntersectWrappedCircularFrustumOpenBoreLine(ctx context.Context, outer, inner *geom.WrappedSurfacePatch,
	origin, direction geom.Vec3, minT, maxT float64, o geom.ToroidLineOptions) (WrappedLoftLineIntersections, error) {
	return intersectParallelFrustumBoreLine(ctx, outer, inner, origin, direction, minT, maxT, o, true, true)
}

// IntersectWrappedEccentricCircularFrustumOpenBoreLine also supports an
// eccentric parallel inner wall. The two cap circles retain their original
// centers and rim provenance.
func IntersectWrappedEccentricCircularFrustumOpenBoreLine(ctx context.Context, outer, inner *geom.WrappedSurfacePatch,
	origin, direction geom.Vec3, minT, maxT float64, o geom.ToroidLineOptions) (WrappedLoftLineIntersections, error) {
	return intersectParallelFrustumBoreLine(ctx, outer, inner, origin, direction, minT, maxT, o, false, true)
}

// IntersectWrappedEccentricCircularFrustumBoreLine handles an independently
// contained circular frustum with zero, one, or both ends open.
// Owners are the two original walls, the outer lower/upper ends, and any
// retained inner caps in canonical lower-to-upper order.
func IntersectWrappedEccentricCircularFrustumBoreLine(ctx context.Context, outer, inner *geom.WrappedSurfacePatch,
	origin, direction geom.Vec3, minT, maxT float64, o geom.ToroidLineOptions) (WrappedLoftLineIntersections, error) {
	return intersectParallelFrustumBoreLine(ctx, outer, inner, origin, direction, minT, maxT, o, false, false)
}

func intersectParallelFrustumBoreLine(ctx context.Context, outer, inner *geom.WrappedSurfacePatch,
	origin, direction geom.Vec3, minT, maxT float64, o geom.ToroidLineOptions, coaxial, oneOpening bool) (WrappedLoftLineIntersections, error) {
	var empty WrappedLoftLineIntersections
	if !origin.IsFinite() || !direction.IsFinite() || !finite(minT) || !finite(maxT) {
		return empty, geom.ErrInvalid
	}
	return intersectParallelFrustumBoreLineRational(ctx, outer, inner,
		[3]*big.Rat{ratInset(origin.X), ratInset(origin.Y), ratInset(origin.Z)},
		[3]*big.Rat{ratInset(direction.X), ratInset(direction.Y), ratInset(direction.Z)}, ratInset(minT), ratInset(maxT), o, coaxial, oneOpening)
}

// IntersectWrappedEccentricCircularFrustumBoreLineRational queries a finite
// closed parameter interval with exact rational origin and direction.
func IntersectWrappedEccentricCircularFrustumBoreLineRational(ctx context.Context, outer, inner *geom.WrappedSurfacePatch,
	origin, direction [3]*big.Rat, minT, maxT *big.Rat, o geom.ToroidLineOptions) (WrappedLoftLineIntersections, error) {
	return intersectParallelFrustumBoreLineRational(ctx, outer, inner, origin, direction, minT, maxT, o, false, false)
}

func intersectParallelFrustumBoreLineRational(ctx context.Context, outer, inner *geom.WrappedSurfacePatch,
	origin, direction [3]*big.Rat, minT, maxT *big.Rat, o geom.ToroidLineOptions, coaxial, oneOpening bool) (WrappedLoftLineIntersections, error) {
	var empty WrappedLoftLineIntersections
	if ctx == nil || o.Surface.MaxCoefficientBits < 0 || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 ||
		o.MaxSubdivisions < 0 || o.MaxSubdivisions > 65536 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	bits := o.Surface.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	check := func(values ...*big.Rat) error {
		for _, n := range values {
			if n == nil {
				return geom.ErrInvalid
			}
			if n.Num().BitLen() > bits || n.Denom().BitLen() > bits {
				return geom.ErrConvergence
			}
		}
		return nil
	}
	if e := check(origin[0], origin[1], origin[2], direction[0], direction[1], direction[2], minT, maxT); e != nil {
		return empty, e
	}
	if direction[0].Sign() == 0 && direction[1].Sign() == 0 && direction[2].Sign() == 0 || minT.Cmp(maxT) > 0 {
		return empty, geom.ErrInvalid
	}
	walls := [2]*geom.WrappedSurfacePatch{outer, inner}
	var proof [2]geom.WrappedFrustumEmbedding
	for i, w := range walls {
		p, e := geom.CertifyWrappedFrustumEmbedding(ctx, w, o.Surface)
		if e != nil {
			return empty, e
		}
		proof[i] = p
	}
	if !strictWrappedParallelFrustumBore(proof[0], proof[1], coaxial) {
		return empty, geom.ErrConvergence
	}
	open := [2]bool{proof[0].LowerZ.Cmp(proof[1].LowerZ) == 0, proof[0].UpperZ.Cmp(proof[1].UpperZ) == 0}
	if oneOpening && open[0] == open[1] {
		return empty, geom.ErrConvergence
	}
	caps := [2]geom.CylinderBoundaryPart{geom.CylinderLowerCap, geom.CylinderUpperCap}
	heights := [2]*big.Rat{proof[0].LowerZ, proof[0].UpperZ}
	var radii [2][2]*big.Rat // wall, end
	for i, p := range proof {
		radii[i] = [2]*big.Rat{p.LowerRadius, p.UpperRadius}
	}
	innerCap := [2]int{}
	next := 4
	for i, opened := range open {
		innerCap[i] = i + 1
		if !opened {
			innerCap[i] = next
			next++
		}
	}
	var result WrappedLoftLineIntersections
	for end, opened := range open {
		if !opened || direction[2].Sign() != 0 || origin[2].Cmp(heights[end]) != 0 {
			continue
		}
		q, e := geom.IntersectEccentricCircularAnnulusLineRational(ctx, [2]*big.Rat{proof[0].CenterX, proof[0].CenterY},
			[2]*big.Rat{proof[1].CenterX, proof[1].CenterY}, radii[1][end], radii[0][end],
			[2]*big.Rat{origin[0], origin[1]}, [2]*big.Rat{direction[0], direction[1]}, minT, maxT, o)
		if e != nil {
			return empty, e
		}
		for _, h := range q.Rims {
			ids := []int{0, end + 1}
			if h.Inner {
				ids = []int{end + 1, 3}
			}
			result.Hits = append(result.Hits, WrappedLoftLineHit{Parameter: h.Parameter, Faces: ids, Parts: caps[end] | geom.CylinderLateral})
		}
		if q.Point != nil {
			result.Hits = append(result.Hits, WrappedLoftLineHit{Parameter: *q.Point, Faces: []int{end + 1}, Parts: caps[end]})
		}
		for _, span := range q.Spans {
			result.Overlaps = append(result.Overlaps, WrappedLoftLineOverlap{Start: span[0], End: span[1], Face: end + 1, Part: caps[end]})
		}
		return result, nil
	}
	// Off the opening plane, both complete frustum surfaces account for
	// all wall and cap candidates. Only the shared disk interior is removed.
	var openingT [2]*big.Rat
	var openingSign [2]int
	for i, opened := range open {
		if !opened || direction[2].Sign() == 0 {
			continue
		}
		t := new(big.Rat).Quo(new(big.Rat).Sub(heights[i], origin[2]), direction[2])
		if t.Cmp(minT) < 0 || t.Cmp(maxT) > 0 {
			continue
		}
		if e := check(t); e != nil {
			return empty, e
		}
		x := new(big.Rat).Add(new(big.Rat).Sub(origin[0], proof[1].CenterX), new(big.Rat).Mul(t, direction[0]))
		y := new(big.Rat).Add(new(big.Rat).Sub(origin[1], proof[1].CenterY), new(big.Rat).Mul(t, direction[1]))
		r2 := new(big.Rat).Add(x.Mul(x, x), y.Mul(y, y))
		if e := check(r2); e != nil {
			return empty, e
		}
		openingT[i], openingSign[i] = t, r2.Cmp(new(big.Rat).Mul(radii[1][i], radii[1][i]))
	}
	for i, w := range walls {
		q, e := geom.IntersectWrappedCircularFrustumLineRational(ctx, w, origin, direction, minT, maxT, o)
		if e != nil {
			return empty, e
		}
		lateral := 3 * i
		owner := func(part geom.CylinderBoundaryPart) int {
			switch part {
			case geom.CylinderLowerCap:
				if i == 0 {
					return 1
				}
				return innerCap[0]
			case geom.CylinderUpperCap:
				if i == 0 {
					return 2
				}
				return innerCap[1]
			default:
				return lateral
			}
		}
		for _, h := range q.Hits {
			parts := h.Parts
			for end, opened := range open {
				if !opened || h.Parts&caps[end] == 0 {
					continue
				}
				t := openingT[end]
				if t == nil || h.Parameter.Min.Cmp(t) != 0 || h.Parameter.Max.Cmp(t) != 0 {
					return empty, geom.ErrConvergence
				}
				rim := openingSign[end]
				if i == 0 && rim <= 0 || i == 1 && rim < 0 {
					// Only the tool owns the inner rim. Neither input's
					// face survives over the removed disk interior.
					parts &^= caps[end]
				}
			}
			ids := make([]int, 0, 2)
			for _, part := range [3]geom.CylinderBoundaryPart{geom.CylinderLateral, caps[0], caps[1]} {
				if parts&part != 0 {
					ids = append(ids, owner(part))
				}
			}
			if len(ids) != 0 {
				sort.Ints(ids)
				result.Hits = append(result.Hits, WrappedLoftLineHit{Parameter: h.Parameter, Faces: ids, Parts: parts})
			}
		}
		for _, h := range q.Overlaps {
			if !(open[0] && h.Part == caps[0] || open[1] && h.Part == caps[1]) {
				result.Overlaps = append(result.Overlaps, WrappedLoftLineOverlap{Start: h.Start, End: h.End, Face: owner(h.Part), Part: h.Part})
			}
		}
	}
	sort.Slice(result.Hits, func(i, j int) bool { return result.Hits[i].Parameter.Min.Cmp(result.Hits[j].Parameter.Min) < 0 })
	for i := 1; i < len(result.Hits); i++ {
		if result.Hits[i-1].Parameter.Max.Cmp(result.Hits[i].Parameter.Min) >= 0 {
			return empty, geom.ErrConvergence
		}
	}
	sort.SliceStable(result.Overlaps, func(i, j int) bool {
		return result.Overlaps[i].Start.Min.Cmp(result.Overlaps[j].Start.Min) < 0
	})
	return result, nil
}
