package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// WrappedRationalStationOccurrenceMass retains original certified unit-density
// measures and source ancestry, with independently owned world intervals.
// Volume is invariant. FirstMoments are absolute world integrals of position;
// Centroid is transformed directly from its proved original interval. Interval
// dependence can widen first moments; no independence or exactness is inferred.
type WrappedRationalStationOccurrenceMass struct {
	Original               WrappedRationalStationMassBounds
	Volume                 WrappedStationMassInterval
	FirstMoments, Centroid [3]WrappedStationMassInterval
	TransformOperations    int
}

// BoundMassProperties performs one original certified profile integration and
// the exact interval identities M'=R*M+t*V and C'=R*C+t. Negative coefficients
// reverse interval endpoints. The proper rigid determinant is exactly1, so
// volume and original support ownership are unchanged. Source integration
// budgets remain separate from the at-most1024 scalar checks/arithmetic of the
// fixed-size affine interval transform, using the occurrence's bit limit.
// Cancellation, integration or transform exhaustion returns an empty result.
func (q WrappedRationalStationOccurrence) BoundMassProperties(ctx context.Context, o WrappedRationalStationMassOptions) (WrappedRationalStationOccurrenceMass, error) {
	empty := WrappedRationalStationOccurrenceMass{}
	if ctx == nil || q.bits == 0 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	original, e := q.source.BoundWrappedRationalStationMass(ctx, o)
	if e != nil {
		return empty, e
	}
	matrix, e := q.pose.Matrix()
	if e != nil {
		return empty, e
	}
	b := stationMassBudget{ctx: ctx, limit: 1024, bits: q.bits}
	result := WrappedRationalStationOccurrenceMass{Original: original, Volume: WrappedStationMassInterval{new(big.Rat).Set(original.Volume.Lower), new(big.Rat).Set(original.Volume.Upper)}}
	for i, row := range matrix {
		result.FirstMoments[i] = b.scaled(original.Volume, row[3])
		result.Centroid[i] = WrappedStationMassInterval{new(big.Rat).Set(row[3]), new(big.Rat).Set(row[3])}
		for j := 0; j < 3; j++ {
			result.FirstMoments[i] = b.plus(result.FirstMoments[i], b.scaled(original.FirstMoments[j], row[j]))
			result.Centroid[i] = b.plus(result.Centroid[i], b.scaled(original.Centroid[j], row[j]))
		}
	}
	if b.err != nil {
		return empty, b.err
	}
	result.TransformOperations = b.used
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	return result, nil
}
