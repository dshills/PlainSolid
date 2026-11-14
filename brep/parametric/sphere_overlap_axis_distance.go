package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// SphereOverlapAxisNativeNearest binds exact ideal retained spherical-cap minima to
// the immutable native original face IDs. Native seam/latitude geometry retains
// CombineSpheres' independent sewing tolerance; this query never uses it as an
// exact source. Complete symbolic pole/cap/circle families are retained by Geometry.
type SphereOverlapAxisNativeNearest struct {
	Geometry                 geom.SphereOverlapAxisNearest
	TiedFaces, ExcludedFaces []int
}

func (s SpherePairSet) NearestOverlapAxisBoundaryRational(ctx context.Context, p [3]*big.Rat, o geom.SurfacePatchOptions) (SphereOverlapAxisNativeNearest, error) {
	var zero SphereOverlapAxisNativeNearest
	if !s.valid {
		return zero, geom.ErrInvalid
	}
	geometry, e := s.query.NearestOverlapAxisBoundaryRational(ctx, p, o)
	if e != nil {
		return zero, e
	}
	out := SphereOverlapAxisNativeNearest{Geometry: geometry}
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
