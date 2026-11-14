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
	return intersectParallelFrustumOpenBoreLine(ctx, outer, inner, origin, direction, minT, maxT, o, true)
}

// IntersectWrappedEccentricCircularFrustumOpenBoreLine also supports an
// eccentric parallel inner wall. The two cap circles retain their original
// centers and rim provenance.
func IntersectWrappedEccentricCircularFrustumOpenBoreLine(ctx context.Context, outer, inner *geom.WrappedSurfacePatch,
	origin, direction geom.Vec3, minT, maxT float64, o geom.ToroidLineOptions) (WrappedLoftLineIntersections, error) {
	return intersectParallelFrustumOpenBoreLine(ctx, outer, inner, origin, direction, minT, maxT, o, false)
}

func intersectParallelFrustumOpenBoreLine(ctx context.Context, outer, inner *geom.WrappedSurfacePatch,
	origin, direction geom.Vec3, minT, maxT float64, o geom.ToroidLineOptions, coaxial bool) (WrappedLoftLineIntersections, error) {
	var empty WrappedLoftLineIntersections
	if ctx == nil || !origin.IsFinite() || !direction.IsFinite() || direction == (geom.Vec3{}) ||
		!finite(minT) || !finite(maxT) || minT > maxT || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 ||
		o.MaxSubdivisions < 0 || o.MaxSubdivisions > 65536 {
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
	loOpen := proof[0].LowerZ.Cmp(proof[1].LowerZ) == 0
	hiOpen := proof[0].UpperZ.Cmp(proof[1].UpperZ) == 0
	if loOpen == hiOpen {
		return empty, geom.ErrConvergence
	}
	open := 0
	if hiOpen {
		open = 1
	}
	caps := [2]geom.CylinderBoundaryPart{geom.CylinderLowerCap, geom.CylinderUpperCap}
	height := [2]*big.Rat{proof[0].LowerZ, proof[0].UpperZ}[open]
	radii := [2]*big.Rat{
		[2]*big.Rat{proof[0].LowerRadius, proof[0].UpperRadius}[open],
		[2]*big.Rat{proof[1].LowerRadius, proof[1].UpperRadius}[open],
	}
	var result WrappedLoftLineIntersections
	if direction.Z == 0 && ratInset(origin.Z).Cmp(height) == 0 {
		q, e := geom.IntersectEccentricCircularAnnulusLine(ctx, [2]*big.Rat{proof[0].CenterX, proof[0].CenterY},
			[2]*big.Rat{proof[1].CenterX, proof[1].CenterY}, radii[1], radii[0],
			geom.Vec2{X: origin.X, Y: origin.Y}, geom.Vec2{X: direction.X, Y: direction.Y}, minT, maxT, o)
		if e != nil {
			return empty, e
		}
		for _, h := range q.Rims {
			ids := []int{0, open + 1}
			if h.Inner {
				ids = []int{open + 1, 3}
			}
			result.Hits = append(result.Hits, WrappedLoftLineHit{Parameter: h.Parameter, Faces: ids, Parts: caps[open] | geom.CylinderLateral})
		}
		if q.Point != nil {
			result.Hits = append(result.Hits, WrappedLoftLineHit{Parameter: *q.Point, Faces: []int{open + 1}, Parts: caps[open]})
		}
		for _, span := range q.Spans {
			result.Overlaps = append(result.Overlaps, WrappedLoftLineOverlap{Start: span[0], End: span[1], Face: open + 1, Part: caps[open]})
		}
		return result, nil
	}
	// Off the opening plane, both complete frustum surfaces account for
	// all wall and cap candidates. Only the shared disk interior is removed.
	var openingT, openingR2 *big.Rat
	if direction.Z != 0 {
		openingT = new(big.Rat).Quo(new(big.Rat).Sub(height, ratInset(origin.Z)), ratInset(direction.Z))
		x := new(big.Rat).Add(new(big.Rat).Sub(ratInset(origin.X), proof[1].CenterX), new(big.Rat).Mul(openingT, ratInset(direction.X)))
		y := new(big.Rat).Add(new(big.Rat).Sub(ratInset(origin.Y), proof[1].CenterY), new(big.Rat).Mul(openingT, ratInset(direction.Y)))
		openingR2 = new(big.Rat).Add(x.Mul(x, x), y.Mul(y, y))
	}
	square := func(q *big.Rat) *big.Rat { return new(big.Rat).Mul(q, q) }
	innerR2 := square(radii[1])
	for i, w := range walls {
		q, e := geom.IntersectWrappedCircularFrustumLine(ctx, w, origin, direction, minT, maxT, o)
		if e != nil {
			return empty, e
		}
		lateral := 3 * i
		owner := func(part geom.CylinderBoundaryPart) int {
			switch part {
			case geom.CylinderLowerCap:
				if i == 0 || open == 0 {
					return 1
				}
			case geom.CylinderUpperCap:
				if i == 0 || open == 1 {
					return 2
				}
			default:
				return lateral
			}
			return 4
		}
		for _, h := range q.Hits {
			parts := h.Parts
			if h.Parts&caps[open] != 0 {
				if openingT == nil || h.Parameter.Min.Cmp(openingT) != 0 || h.Parameter.Max.Cmp(openingT) != 0 {
					return empty, geom.ErrConvergence
				}
				rim := openingR2.Cmp(innerR2)
				if i == 0 && rim <= 0 || i == 1 && rim < 0 {
					// Only the tool owns the inner rim. Neither input's
					// face survives over the removed disk interior.
					parts &^= caps[open]
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
			if h.Part != caps[open] {
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
