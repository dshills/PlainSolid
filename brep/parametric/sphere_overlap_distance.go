package parametric

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"sort"
)

// SphereOverlapNativeNearest binds exact ideal retained spherical-cap minima to
// the immutable native original face IDs. Native seam/latitude geometry retains
// CombineSpheres' independent sewing tolerance; this query never uses it as an
// exact source. Buried radial feet are explicitly reported by Geometry.
type SphereOverlapNativeNearest struct {
	Geometry                 geom.SphereOverlapNearest
	TiedFaces, ExcludedFaces []int
}

func (s SpherePairSet) NearestOverlapBoundaryRational(ctx context.Context, p [3]*big.Rat, o geom.SurfacePatchOptions) (SphereOverlapNativeNearest, error) {
	var zero SphereOverlapNativeNearest
	if !s.valid {
		return zero, geom.ErrInvalid
	}
	geometry, e := s.query.NearestOverlapBoundaryRational(ctx, p, o)
	if e != nil {
		return zero, e
	}
	out := SphereOverlapNativeNearest{Geometry: geometry}
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
