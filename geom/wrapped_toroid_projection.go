package geom

import (
	"context"
	"math/big"
)

// ToroidProjection combines exact material location with certified distance and
// an algebraic closest-boundary projection descriptor. SignedSquaredMin/Max
// are negative inside, zero on the boundary, positive outside. These are
// signed squared distances, not signed distances. Distance retains explicit
// centerline/axis nonuniqueness and exact witness construction data.
type ToroidProjection struct {
	Location                           ToroidMeridianLocation
	SignedSquaredMin, SignedSquaredMax *big.Rat
	Distance                           ToroidDistanceBounds
}

// ProjectWrappedCircularToroid certifies location, signed squared distance and
// algebraic projection for the equal-minor-radius circular toroid subset.
// Material classification never infers a sign from an approximate distance.
// Each independent certificate receives the supplied bounded options; failure
// returns no partial projection. Elliptic meridians remain unsupported here.
func ProjectWrappedCircularToroid(ctx context.Context, w *WrappedSurfacePatch, q Vec3, o FrustumDistanceOptions) (ToroidProjection, error) {
	empty := ToroidProjection{}
	location, e := LocateWrappedCircularToroid(ctx, w, q, o.Surface)
	if e != nil {
		return empty, e
	}
	d, e := BoundPointWrappedCircularToroidDistance(ctx, w, q, o)
	if e != nil {
		return empty, e
	}
	lo, hi := new(big.Rat).Set(d.Lower), new(big.Rat).Set(d.Upper)
	switch location {
	case ToroidMeridianInside:
		lo.Neg(d.Upper)
		hi.Neg(d.Lower)
	case ToroidMeridianBoundary:
		lo.SetInt64(0)
		hi.SetInt64(0)
	}
	return ToroidProjection{location, lo, hi, d}, nil
}
