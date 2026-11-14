package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// SphereCavityNativeNearest maps the exact material owner mask to immutable
// original native face IDs. Geometry retains both attained sphere candidates
// and explicit center families. Inner faces have reversed material normals.
type SphereCavityNativeNearest struct {
	Geometry                 geom.SphereCavityNearest
	TiedFaces, ExcludedFaces []int
}

// NearestCavityBoundaryRational binds the strict full-sphere cavity proof to
// the checked original native sphere pair. Existing point/line contracts are
// unchanged; transverse or tangent material cannot enter this distance subset.
func (s SpherePairSet) NearestCavityBoundaryRational(ctx context.Context, p [3]*big.Rat, o geom.SphereCavityDistanceOptions) (SphereCavityNativeNearest, error) {
	var zero SphereCavityNativeNearest
	if !s.valid {
		return zero, geom.ErrInvalid
	}
	geometry, e := s.query.NearestCavityBoundaryRational(ctx, p, o)
	if e != nil {
		return zero, e
	}
	out := SphereCavityNativeNearest{Geometry: geometry}
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
