//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// pullbackStationLine retains the same affine parameter: R^-1(p+t*d-b)
// equals localOrigin+t*localDirection. In particular no length normalization
// or float conversion can change a root bracket or finite clipping endpoint.
func (q WrappedRationalStationOccurrence) pullbackStationLine(ctx context.Context, p, d [3]*big.Rat) ([3]*big.Rat, [3]*big.Rat, error) {
	empty := [3]*big.Rat{}
	if q.bits == 0 {
		return empty, empty, geom.ErrInvalid
	}
	local, e := q.pose.TransformPoint(ctx, p, true, q.bits)
	if e != nil {
		return empty, empty, e
	}
	direction, e := q.pose.TransformVector(ctx, d, true, q.bits)
	if e != nil {
		return empty, empty, e
	}
	return local, direction, nil
}

// ClipLineRational returns finite world line contacts and open material spans
// with precisely the original rational t and certified algebraic root brackets.
// Original face IDs, multiplicities and Parts are unchanged; Parts denotes
// original lower/upper caps, not world Z directions. Coincident intervals keep
// the source isolated-contact API's strict rejection. The two fixed-size pose
// transformations have the occurrence's bit limit; original query quotas apply
// once, unchanged, to the pulled-back line. All failures are atomic.
func (q WrappedRationalStationOccurrence) ClipLineRational(ctx context.Context, p, d [3]*big.Rat, minT, maxT *big.Rat, o WrappedRationalStationLineOptions) (WrappedRationalStationLine, error) {
	local, direction, e := q.pullbackStationLine(ctx, p, d)
	if e != nil {
		return WrappedRationalStationLine{}, e
	}
	return q.source.ClipWrappedRationalStationLoftLineRational(ctx, local, direction, minT, maxT, o)
}

// ClipCapLineRational is the separate coincident-boundary API for a world line
// on an original terminal cap, including an admitted singleton clip. Boundary
// spans are never material interior; cap/rim face owners retain original IDs.
// A transformed original cap need not be horizontal in world coordinates.
func (q WrappedRationalStationOccurrence) ClipCapLineRational(ctx context.Context, p, d [3]*big.Rat, minT, maxT *big.Rat, o WrappedRationalStationLineOptions) (WrappedRationalStationCapLine, error) {
	local, direction, e := q.pullbackStationLine(ctx, p, d)
	if e != nil {
		return WrappedRationalStationCapLine{}, e
	}
	return q.source.ClipWrappedRationalStationLoftCapLineRational(ctx, local, direction, minT, maxT, o)
}

// ClipGeneratorRational proves coincidence on the selected original wall only.
// It retains the source API's positive-length requirement and original cap or
// crease ending owners, making no material claim about the rest of the world
// line. Other bands and artificial station disks are not substituted.
func (q WrappedRationalStationOccurrence) ClipGeneratorRational(ctx context.Context, face int, p, d [3]*big.Rat, minT, maxT *big.Rat, o WrappedRationalStationLineOptions) (WrappedRationalBoundarySpan, error) {
	local, direction, e := q.pullbackStationLine(ctx, p, d)
	if e != nil {
		return WrappedRationalBoundarySpan{}, e
	}
	return q.source.ClipWrappedRationalStationLoftGeneratorRational(ctx, face, local, direction, minT, maxT, o)
}
