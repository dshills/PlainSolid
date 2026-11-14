package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// IntersectPlacedWrappedToroidLineRational pulls an exact world line into the
// independently certified original toroid without changing its parameter or
// the multiplicity of any root. No placed support is rounded or rebuilt.
func (s Solid) IntersectPlacedWrappedToroidLineRational(ctx context.Context, pose placement.RationalRigid,
	origin, direction [3]*big.Rat, minT, maxT *big.Rat, o geom.ToroidLineOptions) ([]geom.ToroidLineRoot, error) {
	p, e := pose.TransformPoint(ctx, origin, true, o.Surface.MaxCoefficientBits)
	if e != nil {
		return nil, e
	}
	d, e := pose.TransformVector(ctx, direction, true, o.Surface.MaxCoefficientBits)
	if e != nil {
		return nil, e
	}
	return s.IntersectCanonicalWrappedToroidLineRational(ctx, p, d, minT, maxT, o)
}

// PlacedToroidProjection preserves the original exact meridian witness plus
// the world position and three orthonormal placement axes used to map it.
// Local retains signed distance, exact local descriptor and nonuniqueness.
type PlacedToroidProjection struct {
	Local  geom.ToroidProjection
	Center [3]*big.Rat
	Axes   [3][3]*big.Rat // world basis vectors of the local X, Y, Z axes
	Face   int
}

// ProjectPlacedCircularToroidRational computes world signed distance by rigid
// invariance and expresses its exact witness in the original occurrence frame.
func (s Solid) ProjectPlacedCircularToroidRational(ctx context.Context, pose placement.RationalRigid,
	world [3]*big.Rat, o geom.FrustumDistanceOptions) (out PlacedToroidProjection, err error) {
	defer func() {
		if err != nil {
			out = PlacedToroidProjection{Face: -1}
		}
	}()
	bits := o.Surface.MaxCoefficientBits
	p, e := pose.TransformPoint(ctx, world, true, bits)
	if e != nil {
		return out, e
	}
	out.Local, err = s.ProjectCanonicalCircularToroidRational(ctx, p, o)
	if err != nil {
		return out, err
	}
	out.Center, err = pose.TransformPoint(ctx, out.Local.Distance.Center, false, bits)
	if err != nil {
		return out, err
	}
	z, one := new(big.Rat), big.NewRat(1, 1)
	for i := range out.Axes {
		basis := [3]*big.Rat{z, z, z}
		basis[i] = one
		out.Axes[i], err = pose.TransformVector(ctx, basis, false, bits)
		if err != nil {
			return out, err
		}
	}
	return out, ctx.Err()
}
