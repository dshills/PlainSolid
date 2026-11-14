package geom

import (
	"context"
	"math/big"
)

// SphereFullBoundaryDistanceOptions shares the bounded arithmetic contract of
// the original full-support cavity query; no independent budget is restarted.
type SphereFullBoundaryDistanceOptions = SphereCavityDistanceOptions

// SphereFullBoundaryWitness is an attained original sphere minimum or explicit
// center family. For separated union both MaterialNormalSign values are+1.
type SphereFullBoundaryWitness = SphereCavityDistanceWitness

// SphereSeparatedUnionNearest completely classifies original boundary owners
// of two balls with disjoint INTERIORS. External tangency is permitted in this
// material query; both owners may share a nearest foot there. This is not a
// manifold native assembly certificate. Center families remain explicit.
type SphereSeparatedUnionNearest struct {
	Lower, Upper           *big.Rat
	Owners                 BoreBoundaryMask
	Comparison, Operations int
	Candidates             [2]SphereFullBoundaryWitness
}

// NearestSeparatedUnionBoundaryRational proves exact center distance squared
// >=(R+r)^2 before using both full spherical boundaries. Overlapping, contained
// or coincident supports are rejected rather than treating hidden sphere pieces
// as exposed. Guarded exact radical order preserves all ties, even when
// enclosures overlap. All failures are atomic under one shared work/bit budget.
func (s SpherePairMaterial) NearestSeparatedUnionBoundaryRational(ctx context.Context, p [3]*big.Rat, o SphereFullBoundaryDistanceOptions) (SphereSeparatedUnionNearest, error) {
	v, e := s.nearestFullSphereBoundaryRational(ctx, p, o, false)
	if e != nil {
		return SphereSeparatedUnionNearest{}, e
	}
	return SphereSeparatedUnionNearest{Lower: v.Lower, Upper: v.Upper, Owners: v.Owners, Comparison: v.Comparison, Operations: v.Operations, Candidates: v.Candidates}, nil
}
