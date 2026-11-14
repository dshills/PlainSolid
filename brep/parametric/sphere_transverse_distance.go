package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// NearestTransverseBoundaryRational permits irrational common-circle radii in
// the immutable native sphere binding. Its original constructor still requires
// coordinate-aligned centers; arbitrary directions are supported by geom only.
func (s SpherePairSet) NearestTransverseBoundaryRational(ctx context.Context, p [3]*big.Rat, o geom.SphereFullBoundaryDistanceOptions) (SphereOverlapAlgebraicNativeNearest, error) {
	var zero SphereOverlapAlgebraicNativeNearest
	if !s.valid {
		return zero, geom.ErrInvalid
	}
	geometry, e := s.query.NearestTransverseBoundaryRational(ctx, p, o)
	if e != nil {
		return zero, e
	}
	out := SphereOverlapAlgebraicNativeNearest{Geometry: geometry}
	for owner := 0; owner < 2; owner++ {
		if len(s.faces[owner]) != 1 {
			return zero, geom.ErrDomain
		}
		if geometry.Owners&(1<<uint(owner)) != 0 {
			out.TiedFaces = append(out.TiedFaces, s.faces[owner][0])
		} else {
			out.ExcludedFaces = append(out.ExcludedFaces, s.faces[owner][0])
		}
	}
	sort.Ints(out.TiedFaces)
	sort.Ints(out.ExcludedFaces)
	return out, nil
}
