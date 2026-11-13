package geom

import (
	"context"
	"math/big"
)

// PolygonCircularCoverage reports containment of a closed planar polygon with
// open hole interiors in a closed disk with open circular-hole interiors.
// Covered permits boundary contact. Touching is set only on a proved coverage.
// Uncovered does not imply complete disjointness. All circles are independently
// checked for coplanarity, strict hole nesting and mutual hole disjointness.
type PolygonCircularCoverage struct {
	Complete, Covered, Touching bool
	OuterVertexOutside          int
	HoleRelations               []DiskPolygonComparison
	ExactOperations             int
}

// ClassifyPolygonCircularRegion validates the source polygon through the exact
// disk/polygon predicate. Since the outer disk is convex, containing all outer
// vertices proves containing the whole source. A target hole is excluded only
// when its disk has no area intersection with the source's filled region; this
// also permits a source polygon hole to enclose an eccentric circular bore.
func ClassifyPolygonCircularRegion(ctx context.Context, outer []Vec3, holes [][]Vec3, disk PlanarCircularProfile, roundHoles []PlanarCircularProfile, o DiskPolygonOptions) (out PolygonCircularCoverage, err error) {
	out.OuterVertexOutside = -1
	if ctx == nil || !disk.valid() || len(roundHoles) > 64 {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	max := o.MaxOperations
	if max == 0 {
		max = 1000000
	}
	bits := o.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	if max < 1 || bits < 64 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: max, maxBits: bits}
	defer func() {
		out.ExactOperations = b.operations
		if err != nil {
			out.Complete = false
			out.Covered = false
			out.Touching = false
		}
	}()
	compare := func(a, c PlanarCircularProfile) (CircularProfileComparison, error) {
		remaining := max - b.operations
		if remaining < 1 {
			return CircularProfileComparison{}, ErrConvergence
		}
		r, e := ComparePlanarCircularProfiles(ctx, a, c, SurfacePatchEvaluationOptions{MaxOperations: remaining, MaxCoefficientBits: bits})
		b.operations += r.ExactOperations
		return r, e
	}
	for i, hole := range roundHoles {
		r, e := compare(disk, hole)
		if e != nil {
			return out, e
		}
		if r.Relation != CircularProfileAStrictlyContainsB {
			return out, ErrDomain
		}
		for j := 0; j < i; j++ {
			r, e := compare(hole, roundHoles[j])
			if e != nil {
				return out, e
			}
			if r.Relation != CircularProfilesDisjoint {
				return out, ErrDomain
			}
		}
	}
	classify := func(circle PlanarCircularProfile) (DiskPolygonComparison, error) {
		remaining := max - b.operations
		if remaining < 1 {
			return DiskPolygonComparison{}, ErrConvergence
		}
		options := o
		options.MaxOperations = remaining
		options.MaxCoefficientBits = bits
		r, e := ClassifyDiskPlanarPolygon(ctx, circle, outer, holes, options)
		b.operations += r.ExactOperations
		return r, e
	}
	// This first call establishes validity even when later a vertex is outside.
	if _, e := classify(disk); e != nil {
		return out, e
	}
	center := [3]float64{disk.center.X, disk.center.Y, disk.center.Z}
	radius := new(big.Rat).SetFloat64(disk.radius)
	radius.Mul(radius, radius)
	touch := false
	for i, p := range outer {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		distance := new(big.Rat)
		for axis, value := range [3]float64{p.X, p.Y, p.Z} {
			d := new(big.Rat).Sub(new(big.Rat).SetFloat64(value), new(big.Rat).SetFloat64(center[axis]))
			distance.Add(distance, new(big.Rat).Mul(d, d))
		}
		if e := b.check(distance); e != nil {
			return out, e
		}
		cmp := distance.Cmp(radius)
		if cmp > 0 {
			out.OuterVertexOutside = i
			out.Complete = true
			return out, nil
		}
		touch = touch || cmp == 0
	}
	for _, hole := range roundHoles {
		r, e := classify(hole)
		if e != nil {
			return out, e
		}
		out.HoleRelations = append(out.HoleRelations, r)
		if r.Relation != DiskPolygonOutside && r.Relation != DiskPolygonOutsideTouching {
			out.Complete = true
			return out, nil
		}
		touch = touch || r.Relation == DiskPolygonOutsideTouching
	}
	out.Complete = true
	out.Covered = true
	out.Touching = touch
	return out, nil
}
