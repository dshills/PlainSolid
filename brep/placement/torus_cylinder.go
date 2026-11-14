package placement

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// TorusCylinderOccurrence keeps the original analytic material and an immutable
// proper rational rigid pose. It does not construct a transformed native BRep.
type TorusCylinderOccurrence struct {
	source geom.TorusCylinderMaterial
	pose   RationalRigid
}

// PlaceTorusCylinder validates both original material and the proper pose.
// Pose transformations have a separate fixed4096-bit bound from source query
// work. Returned normals and classifications retain original support masks.
func PlaceTorusCylinder(ctx context.Context, s geom.TorusCylinderMaterial, p RationalRigid) (TorusCylinderOccurrence, error) {
	var z TorusCylinderOccurrence
	if _, _, e := s.ClassifyRational(ctx, [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}, geom.SurfacePatchOptions{}); e != nil {
		return z, e
	}
	m, e := p.Matrix()
	if e != nil {
		return z, e
	}
	p, e = NewRationalRigid(m, 4096)
	if e != nil {
		return z, e
	}
	return TorusCylinderOccurrence{s, p}, nil
}
func (s TorusCylinderOccurrence) Original() geom.TorusCylinderMaterial { return s.source }
func (s TorusCylinderOccurrence) Pose() ([3][4]*big.Rat, error)        { return s.pose.Matrix() }
func (s TorusCylinderOccurrence) ClassifyRational(ctx context.Context, p [3]*big.Rat, o geom.SurfacePatchOptions) (geom.BorePointLocation, geom.BoreBoundaryMask, error) {
	q, e := s.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return geom.BoreOutside, 0, e
	}
	return s.source.ClassifyRational(ctx, q, o)
}

// ClipRationalLine preserves original raw t without normalization or rounding.
func (s TorusCylinderOccurrence) ClipRationalLine(ctx context.Context, p, d [3]*big.Rat, min, max *big.Rat, o geom.ToroidLineOptions) (geom.TorusCylinderLine, error) {
	q, e := s.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return geom.TorusCylinderLine{}, e
	}
	v, e := s.pose.TransformVector(ctx, d, true, 4096)
	if e != nil {
		return geom.TorusCylinderLine{}, e
	}
	return s.source.ClipRationalLine(ctx, q, v, min, max, o)
}

// BoundaryNormalsRational rotates each exact exposed source normal direction.
// At a shared rim it returns both directions; no unique seam normal is asserted.
// Source ExactOperations excludes the at-most-four fixed-cost pose rotations.
func (s TorusCylinderOccurrence) BoundaryNormalsRational(ctx context.Context, p [3]*big.Rat, o geom.SurfacePatchOptions) (geom.TorusCylinderNormals, error) {
	q, e := s.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return geom.TorusCylinderNormals{}, e
	}
	v, e := s.source.BoundaryNormalsRational(ctx, q, o)
	if e != nil {
		return geom.TorusCylinderNormals{}, e
	}
	for i := range v.Directions {
		if v.Directions[i][0] != nil {
			v.Directions[i], e = s.pose.TransformVector(ctx, v.Directions[i], false, 4096)
			if e != nil {
				return geom.TorusCylinderNormals{}, e
			}
		}
	}
	return v, nil
}
