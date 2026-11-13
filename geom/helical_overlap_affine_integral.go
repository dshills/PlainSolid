package geom

import (
	"context"
	"math/big"
)

// HelicalAffineOverlapIntegral is the exact volume of ALL retained material
// intersection on the supplied radial/phase cell, within the two stock bands.
// Volume=PiCoefficient*pi. LengthSlope/Intercept describe total axial overlap
// length as a function of phase on the open cell; boundary jumps have zero
// volume. Coefficients and bounds are caller-owned.
type HelicalAffineOverlapIntegral struct {
	Proven                                                                bool
	Radius, Phase                                                         [2]*big.Rat
	LengthSlope, LengthIntercept, FrameAbsoluteDeterminant, PiCoefficient *big.Rat
	ExactOperations                                                       int
}

// IntegrateHelicalConstantSliceCell supports profiles whose exact axial slice
// is constant throughout the radial band and a phase cell with no internal
// endpoint-order or finite-turn event. These sufficient gates prove total
// overlap length affine on the open phase cell. Two interior evaluations give
// its coefficients; endpoint values are deliberately not used. Unsupported
// radial variation or an interior event returns ErrConvergence.
func IntegrateHelicalConstantSliceCell(ctx context.Context, a, c SeparatedHelicalStarts, shift *big.Rat, radii, phases, bandA, bandB [2]*big.Rat, o HelicalStartsLineOptions) (out HelicalAffineOverlapIntegral, err error) {
	if ctx == nil || a.count < 1 || c.count < 1 || radii[0] == nil || radii[1] == nil || radii[0].Sign() <= 0 || radii[0].Cmp(radii[1]) >= 0 || phases[0] == nil || phases[1] == nil || phases[0].Cmp(phases[1]) >= 0 {
		return out, ErrInvalid
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 1000000
	}
	if bits == 0 {
		bits = 16384
	}
	if ops < 1 || bits < 64 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits}
	defer func() {
		out.ExactOperations = b.operations
		if err != nil {
			out.Proven = false
		}
	}()
	if e := ctx.Err(); e != nil {
		return out, e
	}
	for _, s := range []SeparatedHelicalStarts{a, c} {
		reference, e := exactHelicalPolygonSlice(ctx, s.base.profile, radii[0], b)
		if e != nil {
			return out, e
		}
		rs := []*big.Rat{radii[1]}
		for _, p := range s.base.profile {
			r := new(big.Rat).SetFloat64(p.X)
			if r.Cmp(radii[0]) > 0 && r.Cmp(radii[1]) < 0 {
				rs = append(rs, r)
			}
		}
		for _, r := range rs {
			q, e := exactHelicalPolygonSlice(ctx, s.base.profile, r, b)
			if e != nil {
				return out, e
			}
			if q.Present != reference.Present || (q.Present && (q.Lower.Cmp(reference.Lower) != 0 || q.Upper.Cmp(reference.Upper) != 0)) {
				return out, ErrConvergence
			}
		}
	}
	radius := new(big.Rat).Mul(new(big.Rat).Add(radii[0], radii[1]), big.NewRat(1, 2))
	oo := o
	if b.operations >= ops {
		return out, ErrConvergence
	}
	oo.MaxOperations = ops - b.operations
	event, e := CompareHelicalRetainedPhaseWindow(ctx, a, c, radius, shift, phases, bandA, bandB, oo)
	b.operations += event.ExactOperations
	if e != nil {
		return out, e
	}
	for _, p := range event.Events {
		if p.Cmp(phases[0]) > 0 && p.Cmp(phases[1]) < 0 {
			return out, ErrConvergence
		}
	}
	width := new(big.Rat).Sub(phases[1], phases[0])
	var xs, ys [2]*big.Rat
	for i := 0; i < 2; i++ {
		xs[i] = new(big.Rat).Add(phases[0], new(big.Rat).Mul(width, big.NewRat(int64(i+1), 3)))
		if b.operations >= ops {
			return out, ErrConvergence
		}
		oo.MaxOperations = ops - b.operations
		q, e := compareHelicalRetainedPhase(ctx, a, c, radius, xs[i], shift, bandA, bandB, oo)
		b.operations += q.ExactOperations
		if e != nil {
			return out, e
		}
		ys[i] = new(big.Rat)
		for _, contact := range q.Contacts {
			ys[i].Add(ys[i], new(big.Rat).Sub(contact.Bounds[1], contact.Bounds[0]))
		}
	}
	slope := new(big.Rat).Quo(new(big.Rat).Sub(ys[1], ys[0]), new(big.Rat).Sub(xs[1], xs[0]))
	intercept := new(big.Rat).Sub(ys[0], new(big.Rat).Mul(slope, xs[0]))
	// Symmetric interior ordinates have the same mean as an affine function's
	// integral average, including when a finite cutter appears only at an end.
	integratedLength := new(big.Rat).Mul(width, new(big.Rat).Mul(new(big.Rat).Add(ys[0], ys[1]), big.NewRat(1, 2)))
	f := a.base.reference.frame
	v := [3]Vec3{f.XAxis(), f.YAxis(), f.ZAxis()}
	det := new(big.Rat)
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		term := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).SetFloat64(coordinate(v[1], j)), new(big.Rat).SetFloat64(coordinate(v[2], k))), new(big.Rat).Mul(new(big.Rat).SetFloat64(coordinate(v[1], k)), new(big.Rat).SetFloat64(coordinate(v[2], j))))
		det.Add(det, new(big.Rat).Mul(new(big.Rat).SetFloat64(coordinate(v[0], i)), term))
	}
	det.Abs(det)
	if det.Sign() == 0 {
		return out, ErrDegenerate
	}
	coeff := new(big.Rat).Mul(det, new(big.Rat).Mul(new(big.Rat).Sub(new(big.Rat).Mul(radii[1], radii[1]), new(big.Rat).Mul(radii[0], radii[0])), integratedLength))
	for _, q := range []*big.Rat{slope, intercept, det, coeff} {
		if e = b.check(q); e != nil {
			return out, e
		}
	}
	out.LengthSlope = slope
	out.LengthIntercept = intercept
	out.FrameAbsoluteDeterminant = det
	out.PiCoefficient = coeff
	for i := 0; i < 2; i++ {
		out.Radius[i] = new(big.Rat).Set(radii[i])
		out.Phase[i] = new(big.Rat).Set(phases[i])
	}
	out.Proven = true
	return out, nil
}
