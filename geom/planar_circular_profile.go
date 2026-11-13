package geom

import (
	"context"
	"math/big"
)

// PlanarCircularProfile is an ideal closed Euclidean disk defined by exact
// represented center, radius and a raw nonzero plane normal. No normalization
// or approximate orthonormal frame alters the defining plane.
type PlanarCircularProfile struct {
	center, normal Vec3
	radius         float64
}

func NewPlanarCircularProfile(center, normal Vec3, radius float64) (PlanarCircularProfile, error) {
	if !center.IsFinite() || !normal.IsFinite() || normal == (Vec3{}) || !finite(radius) || radius <= 0 {
		return PlanarCircularProfile{}, ErrInvalid
	}
	return PlanarCircularProfile{center, normal, radius}, nil
}
func (p PlanarCircularProfile) Parameters() (center, normal Vec3, radius float64) {
	return p.center, p.normal, p.radius
}
func (p PlanarCircularProfile) valid() bool {
	return p.center.IsFinite() && p.normal.IsFinite() && p.normal != (Vec3{}) && finite(p.radius) && p.radius > 0
}

type CircularProfileRelation uint8

const (
	CircularProfilesNonCoplanar CircularProfileRelation = iota
	CircularProfilesDisjoint
	CircularProfilesExternalTangent
	CircularProfilesOverlap
	CircularProfileAStrictlyContainsB
	CircularProfileBStrictlyContainsA
	CircularProfileAContainsBTangent
	CircularProfileBContainsATangent
	CircularProfilesCoincident
)

// CircularProfileComparison distinguishes ideal closed disk footprints in one
// plane. Overlap means their circular boundaries cross at two points. Internal
// tangency also has area overlap. NonCoplanar makes no disk-intersection claim.
// NormalOrientation is ±1 for coplanar normal directions. All rationals are owned.
type CircularProfileComparison struct {
	Complete, Coplanar    bool
	Relation              CircularProfileRelation
	NormalOrientation     int
	CenterDistanceSquared *big.Rat
	ExactOperations       int
}

// ComparePlanarCircularProfiles uses exact dyadic predicates only. It includes
// subnormal and overflow-scale inputs without squared-distance float overflow.
// Anti-parallel normals are accepted; near-coplanar planes are not collapsed.
func ComparePlanarCircularProfiles(ctx context.Context, a, c PlanarCircularProfile, o SurfacePatchEvaluationOptions) (out CircularProfileComparison, err error) {
	if ctx == nil || !a.valid() || !c.valid() {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 10000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { out.ExactOperations = b.operations }()
	vector := func(v Vec3) [3]*big.Rat {
		return [3]*big.Rat{new(big.Rat).SetFloat64(v.X), new(big.Rat).SetFloat64(v.Y), new(big.Rat).SetFloat64(v.Z)}
	}
	na, nc, ca, cc := vector(a.normal), vector(c.normal), vector(a.center), vector(c.center)
	for _, v := range [][3]*big.Rat{na, nc, ca, cc} {
		for _, q := range v {
			if e := b.check(q); e != nil {
				return out, e
			}
		}
	}
	delta := [3]*big.Rat{}
	distance, dot, offset := new(big.Rat), new(big.Rat), new(big.Rat)
	parallel := true
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		cross := new(big.Rat).Sub(new(big.Rat).Mul(na[j], nc[k]), new(big.Rat).Mul(na[k], nc[j]))
		if e := b.check(cross); e != nil {
			return out, e
		}
		if cross.Sign() != 0 {
			parallel = false
		}
		delta[i] = new(big.Rat).Sub(cc[i], ca[i])
		distance.Add(distance, new(big.Rat).Mul(delta[i], delta[i]))
		dot.Add(dot, new(big.Rat).Mul(na[i], nc[i]))
		offset.Add(offset, new(big.Rat).Mul(na[i], delta[i]))
	}
	for _, q := range []*big.Rat{distance, dot, offset} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	out.CenterDistanceSquared = distance
	if !parallel || offset.Sign() != 0 {
		out.Complete = true
		out.Relation = CircularProfilesNonCoplanar
		return out, nil
	}
	out.Coplanar = true
	out.NormalOrientation = dot.Sign()
	ra, rc := new(big.Rat).SetFloat64(a.radius), new(big.Rat).SetFloat64(c.radius)
	sum, difference := new(big.Rat).Add(ra, rc), new(big.Rat).Sub(ra, rc)
	sum.Mul(sum, sum)
	difference.Mul(difference, difference)
	for _, q := range []*big.Rat{sum, difference} {
		if e := b.check(q); e != nil {
			return out, e
		}
	}
	switch {
	case distance.Sign() == 0 && ra.Cmp(rc) == 0:
		out.Relation = CircularProfilesCoincident
	case distance.Cmp(sum) > 0:
		out.Relation = CircularProfilesDisjoint
	case distance.Cmp(sum) == 0:
		out.Relation = CircularProfilesExternalTangent
	case distance.Cmp(difference) < 0:
		if ra.Cmp(rc) > 0 {
			out.Relation = CircularProfileAStrictlyContainsB
		} else {
			out.Relation = CircularProfileBStrictlyContainsA
		}
	case distance.Cmp(difference) == 0:
		if ra.Cmp(rc) > 0 {
			out.Relation = CircularProfileAContainsBTangent
		} else {
			out.Relation = CircularProfileBContainsATangent
		}
	default:
		out.Relation = CircularProfilesOverlap
	}
	out.Complete = true
	return out, nil
}

// AnnularFootprintRelation describes a closed disk versus a closed annulus.
type AnnularFootprintRelation uint8

const (
	AnnularFootprintNonCoplanar AnnularFootprintRelation = iota
	AnnularFootprintDisjoint
	AnnularFootprintBoundaryOnly
	AnnularFootprintStrictlyInside
	AnnularFootprintInsideTouching
	AnnularFootprintOverlap
)

type AnnularFootprintComparison struct {
	Complete        bool
	Relation        AnnularFootprintRelation
	Outer, Inner    CircularProfileComparison
	ExactOperations int
}

// CompareDiskAnnularFootprint compares a disk against the concentric closed
// annulus innerRadius<=radial distance<=outer.radius. Zero inner radius denotes
// a disk. BoundaryOnly includes isolated tangency and coincident hole boundary;
// InsideTouching contains the disk but not strictly. Overlap is every other
// coplanar area-overlap case. The raw outer plane is retained exactly.
func CompareDiskAnnularFootprint(ctx context.Context, disk, outer PlanarCircularProfile, innerRadius float64, o SurfacePatchEvaluationOptions) (out AnnularFootprintComparison, err error) {
	if !finite(innerRadius) || innerRadius < 0 || !outer.valid() || innerRadius >= outer.radius {
		return out, ErrInvalid
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 10000
	}
	remaining := o.MaxOperations
	compare := func(other PlanarCircularProfile) (CircularProfileComparison, error) {
		if remaining < 1 {
			return CircularProfileComparison{}, ErrConvergence
		}
		options := o
		options.MaxOperations = remaining
		r, e := ComparePlanarCircularProfiles(ctx, disk, other, options)
		remaining -= r.ExactOperations
		out.ExactOperations += r.ExactOperations
		return r, e
	}
	var e error
	out.Outer, e = compare(outer)
	if e != nil {
		return out, e
	}
	if !out.Outer.Coplanar {
		out.Relation = AnnularFootprintNonCoplanar
		out.Complete = true
		return out, nil
	}
	switch out.Outer.Relation {
	case CircularProfilesDisjoint:
		out.Relation = AnnularFootprintDisjoint
		out.Complete = true
		return out, nil
	case CircularProfilesExternalTangent:
		out.Relation = AnnularFootprintBoundaryOnly
		out.Complete = true
		return out, nil
	}
	holeFree := true
	holeTouch := false
	if innerRadius > 0 {
		inner := outer
		inner.radius = innerRadius
		out.Inner, e = compare(inner)
		if e != nil {
			return out, e
		}
		switch out.Inner.Relation {
		case CircularProfileBStrictlyContainsA:
			out.Relation = AnnularFootprintDisjoint
			out.Complete = true
			return out, nil
		case CircularProfileBContainsATangent, CircularProfilesCoincident:
			out.Relation = AnnularFootprintBoundaryOnly
			out.Complete = true
			return out, nil
		case CircularProfilesDisjoint:
		case CircularProfilesExternalTangent:
			holeTouch = true
		default:
			holeFree = false
		}
	}
	if holeFree && (out.Outer.Relation == CircularProfileBStrictlyContainsA || out.Outer.Relation == CircularProfileBContainsATangent || out.Outer.Relation == CircularProfilesCoincident) {
		out.Relation = AnnularFootprintStrictlyInside
		if holeTouch || out.Outer.Relation != CircularProfileBStrictlyContainsA {
			out.Relation = AnnularFootprintInsideTouching
		}
	} else {
		out.Relation = AnnularFootprintOverlap
	}
	out.Complete = true
	return out, nil
}
