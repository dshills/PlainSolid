package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// WrappedAnisotropicBoreMassOptions controls volume and each centroidal inertia
// entry's absolute enclosure width. Nil tolerances default to 1/100. Source
// authentication has its existing per-pass limits; all integration work uses
// one cumulative ledger, including refinement and returned interval arithmetic.
type WrappedAnisotropicBoreMassOptions struct {
	Source                                         geom.SurfacePatchOptions
	VolumeTolerance, InertiaTolerance              *big.Rat
	MaxSegments, MaxOperations, MaxCoefficientBits int
}

// WrappedAnisotropicBoreMassBounds encloses unit-density original material.
// Raw first/second moments are relative to Origin; Centroid is absolute.
// No separate centroid-width promise is made. Source maps both independent
// station grids to the original inward/outward faces and annular caps.
type WrappedAnisotropicBoreMassBounds struct {
	Source                           WrappedAnisotropicStationBoreProof
	Origin                           [3]*big.Rat
	Volume                           WrappedStationMassInterval
	FirstMoments, Centroid           [3]WrappedStationMassInterval
	SecondMoments, CentroidalInertia [3][3]WrappedStationMassInterval
	Segments, ExactOperations        int
}

func (s Solid) BoundWrappedAnisotropicStationBoreMass(ctx context.Context, o WrappedAnisotropicBoreMassOptions) (WrappedAnisotropicBoreMassBounds, error) {
	var empty WrappedAnisotropicBoreMassBounds
	limits, e := normalizeWrappedStationMassOptions(ctx, WrappedRationalStationMassOptions{Source: o.Source, VolumeTolerance: o.VolumeTolerance, MaxSegments: o.MaxSegments, MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
	if e != nil {
		return empty, e
	}
	it := o.InertiaTolerance
	if it == nil {
		it = big.NewRat(1, 100)
	}
	if it.Sign() <= 0 {
		return empty, geom.ErrInvalid
	}
	b := &stationMassBudget{ctx: ctx, limit: limits.MaxOperations, bits: limits.MaxCoefficientBits}
	vt := new(big.Rat).Set(b.check(limits.VolumeTolerance))
	it = new(big.Rat).Set(b.check(it))
	if b.err != nil {
		return empty, b.err
	}
	proof, _, e := s.validateAnisotropicStationBore(ctx, o.Source)
	if e != nil {
		return empty, e
	}
	band := s.faces[proof.OuterWalls[0]].Surface.(*geom.WrappedSurfacePatch)
	segments, weights, e := stationProfileSegments(band, b)
	if e != nil {
		return empty, e
	}
	symmetric := stationProfileCentrallySymmetric(weights, b)
	origin := [3]*big.Rat{new(big.Rat).Set(proof.Outer.Center[0]), new(big.Rat).Set(proof.Outer.Center[1]), new(big.Rat).Set(proof.Outer.Heights[0])}
	a := anisotropicBoreMomentFactors(proof.Outer, origin[2], b)
	sub := anisotropicBoreMomentFactors(proof.Inner, origin[2], b)
	for i := range a {
		a[i] = b.sub(a[i], sub[i])
	}
	if b.err != nil {
		return empty, b.err
	}
	if a[0].Sign() <= 0 {
		return empty, geom.ErrConvergence
	}
	for {
		area, first, e := stationProfileEnclosure(segments, true, b)
		if e != nil {
			return empty, e
		}
		second, e := stationProfileSecondMoments(segments, b)
		if e != nil {
			return empty, e
		}
		if symmetric {
			first = [2]WrappedStationMassInterval{{new(big.Rat), new(big.Rat)}, {new(big.Rat), new(big.Rat)}}
		}
		out := WrappedAnisotropicBoreMassBounds{Source: proof, Origin: origin, Segments: len(segments)}
		out.Volume = b.scaled(area, a[0])
		out.FirstMoments = [3]WrappedStationMassInterval{b.scaled(first[0], a[1]), b.scaled(first[1], a[2]), b.scaled(area, a[3])}
		q := &out.SecondMoments
		q[0][0], q[0][1], q[1][1] = b.scaled(second[0], a[4]), b.scaled(second[1], a[5]), b.scaled(second[2], a[6])
		q[0][2], q[1][2], q[2][2] = b.scaled(first[0], a[7]), b.scaled(first[1], a[8]), b.scaled(area, a[9])
		var c [3][3]WrappedStationMassInterval
		for i := 0; i < 3; i++ {
			out.Centroid[i] = b.plus(b.quotient(out.FirstMoments[i], out.Volume), WrappedStationMassInterval{origin[i], origin[i]})
			for j := i; j < 3; j++ {
				shift := b.quotient(stationIntervalProduct(out.FirstMoments[i], out.FirstMoments[j], i == j, b), out.Volume)
				c[i][j] = WrappedStationMassInterval{b.sub(q[i][j].Lower, shift.Upper), b.sub(q[i][j].Upper, shift.Lower)}
				if i == j && c[i][j].Lower.Sign() < 0 {
					c[i][j].Lower = new(big.Rat)
				}
				q[j][i] = WrappedStationMassInterval{new(big.Rat).Set(q[i][j].Lower), new(big.Rat).Set(q[i][j].Upper)}
			}
		}
		// Shared area cancels exactly in axial mean and variance, and in XZ/YZ shifts.
		zm := b.div(a[3], a[0])
		cz := b.add(origin[2], zm)
		out.Centroid[2] = WrappedStationMassInterval{new(big.Rat).Set(cz), new(big.Rat).Set(cz)}
		c[2][2] = b.scaled(area, b.sub(a[9], b.mul(a[3], zm)))
		c[0][2] = b.scaled(first[0], b.sub(a[7], b.mul(a[1], zm)))
		c[1][2] = b.scaled(first[1], b.sub(a[8], b.mul(a[2], zm)))
		for i := 0; i < 3; i++ {
			out.CentroidalInertia[i][i] = b.plus(c[(i+1)%3][(i+1)%3], c[(i+2)%3][(i+2)%3])
			for j := i + 1; j < 3; j++ {
				out.CentroidalInertia[i][j] = b.scaled(c[i][j], big.NewRat(-1, 1))
				out.CentroidalInertia[j][i] = b.scaled(c[i][j], big.NewRat(-1, 1))
			}
		}
		complete := b.sub(out.Volume.Upper, out.Volume.Lower).Cmp(vt) <= 0
		for i := 0; i < 3; i++ {
			for j := i; j < 3; j++ {
				v := out.CentroidalInertia[i][j]
				if b.sub(v.Upper, v.Lower).Cmp(it) > 0 {
					complete = false
				}
			}
		}
		if b.err != nil {
			return empty, b.err
		}
		if complete {
			out.ExactOperations = b.used
			return out, nil
		}
		if len(segments) > limits.MaxSegments/2 {
			return empty, geom.ErrConvergence
		}
		segments = splitStationProfileSegments(segments, b)
		if b.err != nil {
			return empty, b.err
		}
	}
}

// Integral factors for 1,x,y,z,xx,xy,yy,xz,yz,zz in normalized
// profile coordinates and z relative to origin. Each wall has its own cuts.
func anisotropicBoreMomentFactors(p WrappedAnisotropicStationProof, origin *big.Rat, b *stationMassBudget) [10]*big.Rat {
	powers := [10][3]int{{1, 1, 0}, {2, 1, 0}, {1, 2, 0}, {1, 1, 1}, {3, 1, 0}, {2, 2, 0}, {1, 3, 0}, {2, 1, 1}, {1, 2, 1}, {1, 1, 2}}
	var out [10]*big.Rat
	for k, power := range powers {
		out[k] = new(big.Rat)
		for i := 0; i+1 < len(p.Heights); i++ {
			z := [2]*big.Rat{p.Heights[i], p.Heights[i+1]}
			bases := [3][2]*big.Rat{{p.Scales[i][0], p.Scales[i+1][0]}, {p.Scales[i][1], p.Scales[i+1][1]}, {b.sub(z[0], origin), b.sub(z[1], origin)}}
			factors := make([][2]*big.Rat, 0, 4)
			for axis, n := range power {
				for j := 0; j < n; j++ {
					factors = append(factors, bases[axis])
				}
			}
			out[k] = b.add(out[k], anisotropicStationProductIntegral(b, z, factors...))
		}
	}
	return out
}
