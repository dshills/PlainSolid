package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// SpherePairNativeNearest binds the unified exact symbolic report to immutable
// original native face IDs. It preserves all complete family/tie owners; native
// construction still has its own coordinate-axis and manifold gates.
type SpherePairNativeNearest struct {
	Geometry                 geom.SpherePairNearest
	TiedFaces, ExcludedFaces []int
}

func (s SpherePairSet) NearestBoundaryRational(ctx context.Context, p [3]*big.Rat, o geom.SphereFullBoundaryDistanceOptions) (SpherePairNativeNearest, error) {
	var zero SpherePairNativeNearest
	if !s.valid {
		return zero, geom.ErrInvalid
	}
	geometry, e := s.query.NearestBoundaryRational(ctx, p, o)
	if e != nil {
		return zero, e
	}
	out := SpherePairNativeNearest{Geometry: geometry}

	for owner := 0; owner < 2; owner++ {
		if len(s.faces[owner]) > 1 {
			return zero, geom.ErrDomain
		}
		if geometry.Owners&(1<<uint(owner)) != 0 {
			if len(s.faces[owner]) != 1 {
				return zero, geom.ErrDomain
			}
			id := s.faces[owner][0]
			if len(out.TiedFaces) == 0 || out.TiedFaces[0] != id {
				out.TiedFaces = append(out.TiedFaces, id)
			}
		} else if len(s.faces[owner]) == 1 {
			out.ExcludedFaces = append(out.ExcludedFaces, s.faces[owner][0])
		}
	}

	sort.Ints(out.TiedFaces)
	sort.Ints(out.ExcludedFaces)
	return out, nil
}
