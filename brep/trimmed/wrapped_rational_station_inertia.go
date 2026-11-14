package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// WrappedRationalStationInertiaOptions requests a positive exact width for
// each independent entry of the unit-density centroidal inertia tensor.
type WrappedRationalStationInertiaOptions struct {
	Source                                         geom.SurfacePatchOptions
	InertiaTolerance                               *big.Rat
	MaxSegments, MaxOperations, MaxCoefficientBits int
}

// WrappedRationalStationInertiaBounds encloses the centroidal physical tensor
// in the original model axes. Raw moments integrate (x-Origin) and
// (x-Origin)(x-Origin)^T over the original solid. Origin lies on the common
// station axis at the lower terminal plane to keep translation out of the
// centroidal inertia calculation. Face IDs index the original solid.
type WrappedRationalStationInertiaBounds struct {
	Origin                           [3]*big.Rat
	Volume                           WrappedStationMassInterval
	FirstMoments                     [3]WrappedStationMassInterval
	SecondMoments, CentroidalInertia [3][3]WrappedStationMassInterval
	WallFaces                        []int
	CapFaces                         [2]int
	Segments, ExactOperations        int
}

// BoundWrappedRationalStationInertia encloses the original filled station
// loft's unit-density inertia using the exact weighted profile.
func (s Solid) BoundWrappedRationalStationInertia(ctx context.Context, o WrappedRationalStationInertiaOptions) (WrappedRationalStationInertiaBounds, error) {
	limits, err := o.normalize(ctx)
	if err != nil {
		return WrappedRationalStationInertiaBounds{}, err
	}
	proof, bands, err := s.validateWrappedProfileStationLoft(ctx, o.Source, false)
	if err != nil {
		return WrappedRationalStationInertiaBounds{}, err
	}
	return boundWrappedStationInertia(ctx, proof, nil, bands[0], proof.History.WallFaces, proof.History.CapFaces, limits)
}

// BoundWrappedRationalStationBoreInertia encloses the unit-density inertia of a
// strictly nested station bore. Outer and inner axial factors are subtracted
// exactly before multiplying the common bounded profile moments.
func (s Solid) BoundWrappedRationalStationBoreInertia(ctx context.Context, o WrappedRationalStationInertiaOptions) (WrappedRationalStationInertiaBounds, error) {
	limits, err := o.normalize(ctx)
	if err != nil {
		return WrappedRationalStationInertiaBounds{}, err
	}
	proof, err := s.ValidateWrappedRationalStationBore(ctx, o.Source)
	if err != nil {
		return WrappedRationalStationInertiaBounds{}, err
	}
	band := s.faces[proof.OuterWalls[0]].Surface.(*geom.WrappedSurfacePatch)
	walls := append(append([]int(nil), proof.OuterWalls...), proof.InnerWalls...)
	return boundWrappedStationInertia(ctx, proof.Outer, &proof.Inner, band, walls, proof.Caps, limits)
}

func (o WrappedRationalStationInertiaOptions) normalize(ctx context.Context) (WrappedRationalStationMassOptions, error) {
	return normalizeWrappedStationMassOptions(ctx, WrappedRationalStationMassOptions{
		Source: o.Source, VolumeTolerance: o.InertiaTolerance, MaxSegments: o.MaxSegments,
		MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits,
	})
}

func boundWrappedStationInertia(ctx context.Context, proof WrappedStationLoftProof, inner *WrappedStationLoftProof,
	band *geom.WrappedSurfacePatch, walls []int, caps [2]int, o WrappedRationalStationMassOptions) (WrappedRationalStationInertiaBounds, error) {
	var empty WrappedRationalStationInertiaBounds
	b := &stationMassBudget{ctx: ctx, limit: o.MaxOperations, bits: o.MaxCoefficientBits}
	tolerance := new(big.Rat).Set(b.check(o.VolumeTolerance))
	segments, weights, err := stationProfileSegments(band, b)
	if err != nil {
		return empty, err
	}
	symmetric := stationProfileCentrallySymmetric(weights, b)
	originZ := proof.Heights[0]
	axial := stationInertiaAxialFactors(proof, originZ, b)
	if inner != nil {
		subtract := stationInertiaAxialFactors(*inner, originZ, b)
		for i := range axial {
			axial[i] = b.sub(axial[i], subtract[i])
		}
	}
	if b.err != nil {
		return empty, b.err
	}
	if axial[0].Sign() <= 0 {
		return empty, geom.ErrConvergence
	}
	for {
		area, first, err := stationProfileEnclosure(segments, true, b)
		if err != nil {
			return empty, err
		}
		second, err := stationProfileSecondMoments(segments, b)
		if err != nil {
			return empty, err
		}
		if symmetric {
			first = [2]WrappedStationMassInterval{{new(big.Rat), new(big.Rat)}, {new(big.Rat), new(big.Rat)}}
		}
		result := stationInertiaTransfer(area, first, second, axial, b)
		complete := true
		for i := range result.CentroidalInertia {
			for j := i; j < 3; j++ {
				entry := result.CentroidalInertia[i][j]
				if b.sub(entry.Upper, entry.Lower).Cmp(tolerance) > 0 {
					complete = false
				}
			}
		}
		if b.err != nil {
			return empty, b.err
		}
		if complete {
			result.Origin = [3]*big.Rat{new(big.Rat).Set(proof.Center[0]), new(big.Rat).Set(proof.Center[1]), new(big.Rat).Set(originZ)}
			result.WallFaces = append([]int(nil), walls...)
			result.CapFaces = caps
			result.Segments, result.ExactOperations = len(segments), b.used
			return result, nil
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

func stationInertiaAxialFactors(proof WrappedStationLoftProof, origin *big.Rat, b *stationMassBudget) [6]*big.Rat {
	// Entries are integrals of radius^r times (z-origin)^z with respect to z.
	powers := [6][2]int{{2, 0}, {3, 0}, {2, 1}, {4, 0}, {3, 1}, {2, 2}}
	var factors [6]*big.Rat
	for k, power := range powers {
		factors[k] = new(big.Rat)
		for i := 0; i+1 < len(proof.Heights); i++ {
			h := b.sub(proof.Heights[i+1], proof.Heights[i])
			z := b.sub(proof.Heights[i], origin)
			r := proof.Radii[i]
			d := b.sub(proof.Radii[i+1], r)
			var coefficients [5]*big.Rat
			coefficients[0] = big.NewRat(1, 1)
			for degree := 0; degree < power[0]+power[1]; degree++ {
				constant, slope := r, d
				if degree >= power[0] {
					constant, slope = z, h
				}
				coefficients[degree+1] = b.mul(slope, coefficients[degree])
				for j := degree; j > 0; j-- {
					coefficients[j] = b.add(b.mul(constant, coefficients[j]), b.mul(slope, coefficients[j-1]))
				}
				coefficients[0] = b.mul(constant, coefficients[0])
			}
			integral := new(big.Rat)
			for j := 0; j <= power[0]+power[1]; j++ {
				integral = b.add(integral, b.div(coefficients[j], big.NewRat(int64(j+1), 1)))
			}
			factors[k] = b.add(factors[k], b.mul(h, integral))
		}
	}
	return factors
}

func stationProfileSecondMoments(segments []stationMassSegment, b *stationMassBudget) ([3]WrappedStationMassInterval, error) {
	var second [3]WrappedStationMassInterval
	for i := range second {
		second[i] = WrappedStationMassInterval{new(big.Rat), new(big.Rat)}
	}
	for _, segment := range segments {
		p, innerArea, gap, err := stationProfileTriangle(segment, b)
		if err != nil {
			return second, err
		}
		var coordinates [2]WrappedStationMassInterval
		for i := range coordinates {
			lo, hi := p[0][i], p[0][i]
			for _, v := range p[1:] {
				if v[i].Cmp(lo) < 0 {
					lo = v[i]
				}
				if v[i].Cmp(hi) > 0 {
					hi = v[i]
				}
			}
			coordinates[i] = WrappedStationMassInterval{lo, hi}
		}
		for k, pair := range [3][2]int{{0, 0}, {0, 1}, {1, 1}} {
			i, j := pair[0], pair[1]
			diagonal := b.add(b.mul(p[0][i], p[0][j]), b.mul(p[2][i], p[2][j]))
			mixed := b.add(b.mul(p[0][i], p[2][j]), b.mul(p[2][i], p[0][j]))
			q := b.mul(innerArea, b.div(b.add(b.add(diagonal, diagonal), mixed), big.NewRat(12, 1)))
			pointwise := stationIntervalProduct(coordinates[i], coordinates[j], i == j, b)
			outer := stationIntervalProduct(pointwise, WrappedStationMassInterval{new(big.Rat), gap}, false, b)
			second[k] = b.plus(second[k], b.plus(WrappedStationMassInterval{q, q}, outer))
		}
	}
	return second, b.err
}

func stationIntervalProduct(a, c WrappedStationMassInterval, square bool, b *stationMassBudget) WrappedStationMassInterval {
	if square {
		lo, hi := b.mul(a.Lower, a.Lower), b.mul(a.Upper, a.Upper)
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
		}
		if a.Lower.Sign() <= 0 && a.Upper.Sign() >= 0 {
			lo = new(big.Rat)
		}
		return WrappedStationMassInterval{lo, hi}
	}
	lo, hi := b.mul(a.Lower, c.Lower), b.mul(a.Upper, c.Upper)
	if lo.Cmp(hi) > 0 {
		lo, hi = hi, lo
	}
	for _, v := range []*big.Rat{b.mul(a.Lower, c.Upper), b.mul(a.Upper, c.Lower)} {
		if v.Cmp(lo) < 0 {
			lo = v
		}
		if v.Cmp(hi) > 0 {
			hi = v
		}
	}
	return WrappedStationMassInterval{lo, hi}
}

func stationInertiaTransfer(area WrappedStationMassInterval, first [2]WrappedStationMassInterval, second [3]WrappedStationMassInterval,
	a [6]*big.Rat, b *stationMassBudget) WrappedRationalStationInertiaBounds {
	var out WrappedRationalStationInertiaBounds
	out.Volume = b.scaled(area, a[0])
	out.FirstMoments = [3]WrappedStationMassInterval{b.scaled(first[0], a[1]), b.scaled(first[1], a[1]), b.scaled(area, a[2])}
	q := &out.SecondMoments
	q[0][0], q[0][1], q[1][1] = b.scaled(second[0], a[3]), b.scaled(second[1], a[3]), b.scaled(second[2], a[3])
	q[0][2], q[1][2], q[2][2] = b.scaled(first[0], a[4]), b.scaled(first[1], a[4]), b.scaled(area, a[5])
	var centroidal [3][3]WrappedStationMassInterval
	for i := 0; i < 2; i++ {
		for j := i; j < 2; j++ {
			shift := b.quotient(stationIntervalProduct(out.FirstMoments[i], out.FirstMoments[j], i == j, b), out.Volume)
			lo, hi := b.sub(q[i][j].Lower, shift.Upper), b.sub(q[i][j].Upper, shift.Lower)
			if i == j && lo.Sign() < 0 {
				lo = new(big.Rat)
			}
			centroidal[i][j] = WrappedStationMassInterval{lo, hi}
		}
	}
	// The shared profile area cancels from the axial mean. Compute its
	// variance and each mixed axial factor exactly before applying intervals.
	zMean := b.div(a[2], a[0])
	centroidal[2][2] = b.scaled(area, b.sub(a[5], b.mul(a[2], zMean)))
	xzFactor := b.sub(a[4], b.mul(a[1], zMean))
	centroidal[0][2], centroidal[1][2] = b.scaled(first[0], xzFactor), b.scaled(first[1], xzFactor)
	for i := range q {
		for j := i + 1; j < 3; j++ {
			q[j][i] = WrappedStationMassInterval{new(big.Rat).Set(q[i][j].Lower), new(big.Rat).Set(q[i][j].Upper)}
		}
	}
	for i := range out.CentroidalInertia {
		out.CentroidalInertia[i][i] = b.plus(centroidal[(i+1)%3][(i+1)%3], centroidal[(i+2)%3][(i+2)%3])
		for j := i + 1; j < 3; j++ {
			m := centroidal[i][j]
			out.CentroidalInertia[i][j] = WrappedStationMassInterval{b.sub(new(big.Rat), m.Upper), b.sub(new(big.Rat), m.Lower)}
			out.CentroidalInertia[j][i] = WrappedStationMassInterval{b.sub(new(big.Rat), m.Upper), b.sub(new(big.Rat), m.Lower)}
		}
	}
	return out
}
