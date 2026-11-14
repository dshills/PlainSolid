package trimmed

import (
	"context"
	"math/big"

	"plainsolid/brep/parametric"
	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// LocatePlacedWrappedFrustumBoreRational locates a world point against a
// proper rigid occurrence of this original native bore. Curves and face IDs
// remain in the original solid; no placed spline is constructed or rounded.
func (s Solid) LocatePlacedWrappedFrustumBoreRational(ctx context.Context, pose placement.RationalRigid,
	point [3]*big.Rat, o geom.SurfacePatchOptions) (parametric.Location, error) {
	p, e := pose.TransformPoint(ctx, point, true, o.MaxCoefficientBits)
	if e != nil {
		return parametric.Outside, e
	}
	return s.LocateCanonicalWrappedFrustumBoreRational(ctx, p, o)
}

// IntersectPlacedWrappedFrustumBoreLineRational keeps the caller's exact
// world line parameter and every original native cap/rim owner.
func (s Solid) IntersectPlacedWrappedFrustumBoreLineRational(ctx context.Context, pose placement.RationalRigid,
	origin, direction [3]*big.Rat, minT, maxT *big.Rat, o geom.ToroidLineOptions) (WrappedLoftLineIntersections, error) {
	p, e := pose.TransformPoint(ctx, origin, true, o.Surface.MaxCoefficientBits)
	if e != nil {
		return WrappedLoftLineIntersections{}, e
	}
	d, e := pose.TransformVector(ctx, direction, true, o.Surface.MaxCoefficientBits)
	if e != nil {
		return WrappedLoftLineIntersections{}, e
	}
	return s.IntersectCanonicalWrappedFrustumBoreLineRational(ctx, p, d, minT, maxT, o)
}

// PlacedFrustumBoreWitness is a world-space point of the form
// Base + Numerator/sqrt(Radical). All coefficients are independently owned
// exact rationals. It retains original bore face and part ownership.
type PlacedFrustumBoreWitness struct {
	Base, Numerator       [3]*big.Rat
	Radical               *big.Rat
	Face                  int
	Part                  geom.CylinderBoundaryPart
	RotationallyNonunique bool
}

type PlacedFrustumBoreDistance struct {
	Lower, Upper                *big.Rat
	Witness                     PlacedFrustumBoreWitness
	Bisections, ExactOperations int
}

// BoundPointPlacedWrappedFrustumBoreDistanceRational preserves exact
// Euclidean bounds by rigid invariance, and returns the certified witness
// in the supplied world occurrence frame.
func (s Solid) BoundPointPlacedWrappedFrustumBoreDistanceRational(ctx context.Context, pose placement.RationalRigid,
	point [3]*big.Rat, o geom.FrustumDistanceOptions) (PlacedFrustumBoreDistance, error) {
	var out PlacedFrustumBoreDistance
	bits := o.Surface.MaxCoefficientBits
	p, e := pose.TransformPoint(ctx, point, true, bits)
	if e != nil {
		return out, e
	}
	q, e := s.BoundPointCanonicalWrappedFrustumBoreDistanceRational(ctx, p, o)
	if e != nil {
		return out, e
	}
	w := q.Witness
	base, e := pose.TransformPoint(ctx, [3]*big.Rat{w.CenterX, w.CenterY, w.Height}, false, bits)
	if e != nil {
		return out, e
	}
	n, e := pose.TransformVector(ctx, [3]*big.Rat{new(big.Rat).Mul(w.Radius, w.AzimuthX), new(big.Rat).Mul(w.Radius, w.AzimuthY), new(big.Rat)}, false, bits)
	if e != nil {
		return out, e
	}
	return PlacedFrustumBoreDistance{Lower: q.Lower, Upper: q.Upper, Bisections: q.Bisections, ExactOperations: q.ExactOperations,
		Witness: PlacedFrustumBoreWitness{Base: base, Numerator: n, Radical: new(big.Rat).Set(w.AzimuthSquared), Face: q.Face, Part: w.Part, RotationallyNonunique: w.RotationallyNonunique}}, nil
}

// PlacedFrustumBoreSection carries a section cut by (world−Origin)·Axis=0.
// Local retains exact original curves and face IDs. Centers are exact world
// points for each present circle. Axis, U, and V are orthonormal world vectors
// defining the section plane; no rounded world spline is constructed.
type PlacedFrustumBoreSection struct {
	Local                    WrappedFrustumBoreSection
	Origin, Axis, U, V       [3]*big.Rat
	OuterCenter, InnerCenter [3]*big.Rat
}

// SectionPlacedWrappedFrustumBoreRational places the native circular section
// at its exact original axial coordinate, which may be nonbinary.
func (s Solid) SectionPlacedWrappedFrustumBoreRational(ctx context.Context, pose placement.RationalRigid,
	height *big.Rat, o geom.SurfacePatchOptions) (out PlacedFrustumBoreSection, err error) {
	defer func() {
		if err != nil {
			out = PlacedFrustumBoreSection{}
		}
	}()
	bits := o.MaxCoefficientBits
	z := new(big.Rat)
	one := big.NewRat(1, 1)
	out.Origin, err = pose.TransformPoint(ctx, [3]*big.Rat{z, z, height}, false, bits)
	if err != nil {
		return out, err
	}
	for i, v := range [3]*[3]*big.Rat{&out.U, &out.V, &out.Axis} {
		basis := [3]*big.Rat{z, z, z}
		basis[i] = one
		*v, err = pose.TransformVector(ctx, basis, false, bits)
		if err != nil {
			return out, err
		}
	}
	out.Local, err = s.SectionCanonicalWrappedFrustumBoreRational(ctx, height, o)
	if err != nil {
		return out, err
	}
	if out.Local.OuterFace >= 0 {
		v := out.Local.Outer
		out.OuterCenter, err = pose.TransformPoint(ctx, [3]*big.Rat{v.CenterX, v.CenterY, height}, false, bits)
		if err != nil {
			return out, err
		}
	}
	if out.Local.InnerFace >= 0 {
		v := out.Local.Inner
		out.InnerCenter, err = pose.TransformPoint(ctx, [3]*big.Rat{v.CenterX, v.CenterY, height}, false, bits)
	}
	return out, err
}
