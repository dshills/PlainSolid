package geom

import (
	"context"
	"math/big"
)

// SphereLensRim is the exact source circle of a transverse sphere intersection.
// Its plane is Axis·(p−Center)=0 and its radius is sqrt(RadiusSquared). Axis is
// the original directed center displacement, not a rounded unit vector.
// CenterFraction is the fraction along that axis from the first center.
// NormalDot is the dot product of the original outward unit normals anywhere
// on the rim. All rational values belong to the caller.
type SphereLensRim struct {
	Center, Axis                               [3]*big.Rat
	RadiusSquared, AxisSquared, CenterFraction *big.Rat
	NormalDot                                  *big.Rat
}

// TransverseLensRim derives and verifies the original sphere-circle equations
// without square roots of the center distance or rim radius. The result is
// independent of any native/analytic support representation or tessellation.
func (s SpherePairMaterial) TransverseLensRim(ctx context.Context, o SurfacePatchOptions) (SphereLensRim, error) {
	if ctx == nil || s.radius2[0] == nil {
		return SphereLensRim{}, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return SphereLensRim{}, err
	}
	if !s.transverse || s.op != SphereSetIntersection {
		return SphereLensRim{}, ErrDomain
	}
	limits, err := patchOptions(o)
	if err != nil {
		return SphereLensRim{}, err
	}
	a := &sphereOverlapArithmetic{ctx: ctx, b: &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}}
	for i, center := range s.center {
		for _, coordinate := range center {
			a.check(coordinate)
		}
		a.check(s.radius2[i])
	}
	return s.deriveLensRim(a)
}

func (s SpherePairMaterial) deriveLensRim(a *sphereOverlapArithmetic) (SphereLensRim, error) {
	var r SphereLensRim
	for j := range r.Axis {
		r.Axis[j] = a.sub(s.center[1][j], s.center[0][j])
	}
	two := big.NewRat(2, 1)
	r.AxisSquared = a.distance(s.center[0], s.center[1])
	r.CenterFraction = a.div(a.add(a.sub(s.radius2[0], s.radius2[1]), r.AxisSquared), a.mul(two, r.AxisSquared))
	r.RadiusSquared = a.sub(s.radius2[0], a.mul(a.mul(r.CenterFraction, r.CenterFraction), r.AxisSquared))
	for j := range r.Center {
		r.Center[j] = a.add(s.center[0][j], a.mul(r.CenterFraction, r.Axis[j]))
	}
	radiiProduct := a.mul(a.sqrt(s.radius2[0]), a.sqrt(s.radius2[1]))
	r.NormalDot = a.div(a.sub(a.add(s.radius2[0], s.radius2[1]), r.AxisSquared), a.mul(two, radiiProduct))
	if a.err != nil {
		return SphereLensRim{}, a.err
	}
	if r.RadiusSquared.Sign() <= 0 || new(big.Rat).Abs(r.NormalDot).Cmp(big.NewRat(1, 1)) >= 0 {
		return SphereLensRim{}, ErrConvergence
	}
	for i, c := range s.center {
		surface := a.add(a.distance(r.Center, c), r.RadiusSquared)
		if a.err != nil {
			return SphereLensRim{}, a.err
		}
		if surface.Cmp(s.radius2[i]) != 0 {
			return SphereLensRim{}, ErrConvergence
		}
	}
	return r, nil
}

// SphereLensRimEnclosure retains the exact rim and closed rational bounds
// on its radius and the two cap latitudes. The latitudes use a common axis
// directed from sphere one to sphere two; Latitude[0] is always above
// Latitude[1]. This also covers two joins on the same side of the equator.
type SphereLensRimEnclosure struct {
	Rim      SphereLensRim
	Radius   ExactParameterInterval
	Latitude [2]ExactParameterInterval
	Work     int
}

// EncloseTransverseLensRim supplies independently bounded chart parameters
// for the continuous spherical cap and rolling-band joins. PrecisionBits is
// an absolute interval width for lengths and radians; see SphereLensMeasureOptions.
// The original axis and center remain exact even if no binary64 chart can
// represent the contact circle. Failures return no partial parameters.
func (s SpherePairMaterial) EncloseTransverseLensRim(ctx context.Context, o SphereLensMeasureOptions) (SphereLensRimEnclosure, error) {
	var zero SphereLensRimEnclosure
	if ctx == nil || s.radius2[0] == nil {
		return zero, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	if !s.transverse || s.op != SphereSetIntersection {
		return zero, ErrDomain
	}
	if o.PrecisionBits == 0 {
		o.PrecisionBits = 40
	}
	if o.PrecisionBits < 8 || o.PrecisionBits > 128 {
		return zero, ErrInvalid
	}
	limits, err := patchOptions(o.Arithmetic)
	if err != nil {
		return zero, err
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	a := &sphereOverlapArithmetic{ctx: ctx, b: b}
	for i, center := range s.center {
		for _, coordinate := range center {
			a.check(coordinate)
		}
		a.check(s.radius2[i])
	}
	r, err := s.deriveLensRim(a)
	if err != nil {
		return zero, err
	}
	fractions := [2]*big.Rat{r.CenterFraction, a.sub(r.CenterFraction, big.NewRat(1, 1))}
	var slopes2 [2]*big.Rat
	for i, f := range fractions {
		slopes2[i] = a.div(a.mul(a.mul(f, f), r.AxisSquared), r.RadiusSquared)
	}
	if a.err != nil {
		return zero, a.err
	}
	n := lensMeasureIntervals{a}
	target := lensMeasurePowerOfTwo(o.PrecisionBits)
	for bits := o.PrecisionBits + 12; bits <= 512; bits += 32 {
		pi := lensMeasurePi(ctx, lensMeasurePowerOfTwo(bits+12), a)
		radius := n.root(r.RadiusSquared, bits)
		var angle [2]exactInterval
		for i, f := range fractions {
			angle[i] = n.angle(n.root(slopes2[i], bits), pi, bits)
			if f.Sign() < 0 {
				angle[i] = n.scale(angle[i], big.NewRat(-1, 1))
			}
		}
		width := func(q exactInterval) *big.Rat { return a.sub(q.hi, q.lo) }
		retry := radius.lo.Sign() <= 0 || angle[0].lo.Cmp(angle[1].hi) <= 0 ||
			width(radius).Cmp(target) > 0 || width(angle[0]).Cmp(target) > 0 || width(angle[1]).Cmp(target) > 0
		if a.err != nil {
			return zero, a.err
		}
		if retry {
			continue
		}
		if err := ctx.Err(); err != nil {
			return zero, err
		}
		out := SphereLensRimEnclosure{Rim: r, Radius: ExactParameterInterval{radius.lo, radius.hi}, Work: b.operations}
		for i, v := range angle {
			out.Latitude[i] = ExactParameterInterval{v.lo, v.hi}
		}
		return out, nil
	}
	return zero, ErrConvergence
}
