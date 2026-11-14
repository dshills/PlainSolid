package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// SphereSeparatedUnionNativeNearest binds the exact full-support report to
// original native face IDs. It does not weaken NewSpherePairSet's rejection
// of a nonmanifold external tangent union.
type SphereSeparatedUnionNativeNearest struct {
	Geometry                 geom.SphereSeparatedUnionNearest
	TiedFaces, ExcludedFaces []int
}

func (s SpherePairSet) NearestSeparatedUnionBoundaryRational(ctx context.Context, p [3]*big.Rat, o geom.SphereFullBoundaryDistanceOptions) (SphereSeparatedUnionNativeNearest, error) {
	var zero SphereSeparatedUnionNativeNearest
	if !s.valid {
		return zero, geom.ErrInvalid
	}
	geometry, e := s.query.NearestSeparatedUnionBoundaryRational(ctx, p, o)
	if e != nil {
		return zero, e
	}
	out := SphereSeparatedUnionNativeNearest{Geometry: geometry}
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
