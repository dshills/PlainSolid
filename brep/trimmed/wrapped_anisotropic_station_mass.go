package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// WrappedAnisotropicStationMassBounds encloses unit-density original material.
// AxialVolumeFactor=integral(sx*sy dz), AxialFirstXYFactors are integrals
// (sx²*sy dz, sx*sy² dz), and AxialFirstZFactor=integral(z*sx*sy dz).
// The axial factors and Z centroid are exact. The normalized original profile
// area and first moments have rational enclosures. VolumeTolerance controls
// only volume width; no separate centroid-width promise is made.
type WrappedAnisotropicStationMassBounds struct {
	Source                               WrappedAnisotropicStationProof
	ProfileArea, Volume                  WrappedStationMassInterval
	ProfileFirstMoments                  [2]WrappedStationMassInterval
	FirstMoments, Centroid               [3]WrappedStationMassInterval
	AxialVolumeFactor, AxialFirstZFactor *big.Rat
	AxialFirstXYFactors                  [2]*big.Rat
	Segments, ExactOperations            int
}

// BoundWrappedAnisotropicStationMass reauthenticates the original native
// source, then integrates its normalized rational profile with the same exact
// chord/control-triangle enclosure as the homothetic mass path. Exact full rim
// agreement proves that profile applies to every band. Independent X/Y scale
// products are integrated analytically, never replaced by a single radius.
// One aggregate integration ledger covers all bands, subdivisions and output
// arithmetic. Native source reproof keeps its own bounded per-pass quotas.
func (s Solid) BoundWrappedAnisotropicStationMass(ctx context.Context, o WrappedRationalStationMassOptions) (WrappedAnisotropicStationMassBounds, error) {
	empty := WrappedAnisotropicStationMassBounds{}
	o, e := normalizeWrappedStationMassOptions(ctx, o)
	if e != nil {
		return empty, e
	}
	b := &stationMassBudget{ctx: ctx, limit: o.MaxOperations, bits: o.MaxCoefficientBits}
	tolerance := b.check(o.VolumeTolerance)
	if b.err != nil {
		return empty, b.err
	}
	tolerance = new(big.Rat).Set(tolerance)
	proof, bands, e := s.validateWrappedAnisotropicStationLoft(ctx, o.Source)
	if e != nil {
		return empty, e
	}
	segments, weights, e := stationProfileSegments(bands[0], b)
	if e != nil {
		return empty, e
	}
	symmetric := stationProfileCentrallySymmetric(weights, b)
	volumeFactor, zFactor := new(big.Rat), new(big.Rat)
	xyFactors := [2]*big.Rat{new(big.Rat), new(big.Rat)}
	for i := range bands {
		x := [2]*big.Rat{proof.Scales[i][0], proof.Scales[i+1][0]}
		y := [2]*big.Rat{proof.Scales[i][1], proof.Scales[i+1][1]}
		z := [2]*big.Rat{proof.Heights[i], proof.Heights[i+1]}
		volumeFactor = b.add(volumeFactor, anisotropicStationProductIntegral(b, z, x, y))
		xyFactors[0] = b.add(xyFactors[0], anisotropicStationProductIntegral(b, z, x, x, y))
		xyFactors[1] = b.add(xyFactors[1], anisotropicStationProductIntegral(b, z, x, y, y))
		zFactor = b.add(zFactor, anisotropicStationProductIntegral(b, z, x, y, z))
	}
	if b.err != nil {
		return empty, b.err
	}
	if volumeFactor.Sign() <= 0 || xyFactors[0].Sign() <= 0 || xyFactors[1].Sign() <= 0 {
		return empty, geom.ErrConvergence
	}
	for {
		area, moments, e := stationProfileEnclosure(segments, true, b)
		if e != nil {
			return empty, e
		}
		volume := b.scaled(area, volumeFactor)
		width := b.sub(volume.Upper, volume.Lower)
		if b.err != nil {
			return empty, b.err
		}
		if width.Cmp(tolerance) <= 0 {
			out := WrappedAnisotropicStationMassBounds{Source: proof, ProfileArea: area, Volume: volume, AxialVolumeFactor: volumeFactor, AxialFirstXYFactors: xyFactors, AxialFirstZFactor: zFactor, Segments: len(segments)}
			for axis := 0; axis < 2; axis++ {
				if symmetric {
					moments[axis] = WrappedStationMassInterval{new(big.Rat), new(big.Rat)}
				}
				out.ProfileFirstMoments[axis] = WrappedStationMassInterval{new(big.Rat).Set(moments[axis].Lower), new(big.Rat).Set(moments[axis].Upper)}
				out.FirstMoments[axis] = b.plus(b.scaled(volume, proof.Center[axis]), b.scaled(moments[axis], xyFactors[axis]))
				offset := b.scaled(b.quotient(moments[axis], area), b.div(xyFactors[axis], volumeFactor))
				out.Centroid[axis] = b.plus(offset, WrappedStationMassInterval{proof.Center[axis], proof.Center[axis]})
			}
			out.FirstMoments[2] = b.scaled(area, zFactor)
			centerZ := b.div(zFactor, volumeFactor)
			out.Centroid[2] = WrappedStationMassInterval{new(big.Rat).Set(centerZ), new(big.Rat).Set(centerZ)}
			if b.err != nil {
				return empty, b.err
			}
			out.ExactOperations = b.used
			return out, nil
		}
		if len(segments) > o.MaxSegments/2 {
			return empty, geom.ErrConvergence
		}
		segments = splitStationProfileSegments(segments, b)
		if b.err != nil {
			return empty, b.err
		}
	}
}

// Fixed-size products of two or three affine factors; every scalar operation
// is charged to the caller's one integration ledger.
func anisotropicStationProductIntegral(b *stationMassBudget, z [2]*big.Rat, factors ...[2]*big.Rat) *big.Rat {
	coefficients := []*big.Rat{big.NewRat(1, 1)}
	for i := 0; i < len(factors); i++ {
		a, delta := factors[i][0], b.sub(factors[i][1], factors[i][0])
		next := make([]*big.Rat, len(coefficients)+1)
		for j := range next {
			next[j] = new(big.Rat)
		}
		for j, c := range coefficients {
			next[j] = b.add(next[j], b.mul(c, a))
			next[j+1] = b.add(next[j+1], b.mul(c, delta))
		}
		coefficients = next
	}
	value := new(big.Rat)
	for i, c := range coefficients {
		value = b.add(value, b.div(c, big.NewRat(int64(i+1), 1)))
	}
	return b.mul(b.sub(z[1], z[0]), value)
}
