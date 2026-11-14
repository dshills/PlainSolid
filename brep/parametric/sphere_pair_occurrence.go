package parametric

import (
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// SpherePairOccurrence retains an immutable original native sphere pair and an
// exact proper rational rigid pose. Original().Solid() stays in source space;
// this query occurrence does not manufacture a rounded world BRep.
type SpherePairOccurrence struct {
	source SpherePairSet
	pose   placement.RationalRigid
}

// PlaceRigid independently validates the pose with a fixed 4096-bit limit.
func (s SpherePairSet) PlaceRigid(ctx context.Context, pose placement.RationalRigid) (SpherePairOccurrence, error) {
	if ctx == nil || !s.valid {
		return SpherePairOccurrence{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return SpherePairOccurrence{}, e
	}
	m, e := pose.Matrix()
	if e != nil {
		return SpherePairOccurrence{}, e
	}
	p, e := placement.NewRationalRigid(m, 4096)
	if e != nil {
		return SpherePairOccurrence{}, e
	}
	return SpherePairOccurrence{s, p}, nil
}
func (s SpherePairOccurrence) Original() SpherePairSet       { return s.source }
func (s SpherePairOccurrence) Pose() ([3][4]*big.Rat, error) { return s.pose.Matrix() }

// ClipRationalLine pulls back both vectors exactly, preserving t, direction,
// finite clipping, and original operand owners. Pose arithmetic has a separate
// fixed 4096-bit bound; the original query work contract is unchanged.
func (s SpherePairOccurrence) ClipRationalLine(ctx context.Context, origin, direction [3]*big.Rat, min, max *big.Rat, o geom.ToroidLineOptions) (geom.SpherePairMaterialLine, error) {
	var z geom.SpherePairMaterialLine
	if !s.source.valid {
		return z, geom.ErrInvalid
	}
	p, e := s.pose.TransformPoint(ctx, origin, true, 4096)
	if e != nil {
		return z, e
	}
	d, e := s.pose.TransformVector(ctx, direction, true, 4096)
	if e != nil {
		return z, e
	}
	return s.source.ClipRationalLine(ctx, p, d, min, max, o)
}

// SpherePairOccurrenceNearest retains the original proof and independently
// owned world symbolic payload. Proper rigidity preserves squared distance,
// complete families, normal signs, and every original owner/face ID. World
// centers/plane points are translated; numerators/normals are rotated. No
// algebraic foot or non-singleton family is replaced by a rounded point.
type SpherePairOccurrenceNearest struct {
	Base  SpherePairNativeNearest
	World geom.SpherePairNearest
}

// NearestBoundaryRational performs one source query, then bounded exact witness
// mapping. Operations remains the source ledger; mapping uses at most 32 vector
// transforms, each with a separate fixed 4096-bit arithmetic bound. Any failure
// is atomic, including mapping failure after a successful original proof.
func (s SpherePairOccurrence) NearestBoundaryRational(ctx context.Context, p [3]*big.Rat, o geom.SphereFullBoundaryDistanceOptions) (SpherePairOccurrenceNearest, error) {
	var z SpherePairOccurrenceNearest
	if !s.source.valid {
		return z, geom.ErrInvalid
	}
	q, e := s.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return z, e
	}
	b, e := s.source.NearestBoundaryRational(ctx, q, o)
	if e != nil {
		return z, e
	}
	w, e := mapSphereNearest(ctx, s.pose, b.Geometry)
	if e != nil {
		return z, e
	}
	return SpherePairOccurrenceNearest{Base: b, World: w}, nil
}

func sphereOccurrenceRat(q *big.Rat) *big.Rat {
	if q == nil {
		return nil
	}
	return new(big.Rat).Set(q)
}
func mapSphereNearest(ctx context.Context, pose placement.RationalRigid, b geom.SpherePairNearest) (geom.SpherePairNearest, error) {
	w := b
	var err error
	point := func(v [3]*big.Rat, vector bool) [3]*big.Rat {
		if err != nil {
			return [3]*big.Rat{}
		}
		if v[0] == nil && v[1] == nil && v[2] == nil {
			return v
		}
		var a [3]*big.Rat
		if vector {
			a, err = pose.TransformVector(ctx, v, false, 4096)
		} else {
			a, err = pose.TransformPoint(ctx, v, false, 4096)
		}
		return a
	}
	rat := sphereOccurrenceRat
	roots := func(v [2]*big.Rat) [2]*big.Rat { return [2]*big.Rat{rat(v[0]), rat(v[1])} }
	full := func(v geom.SphereFullBoundaryWitness) geom.SphereFullBoundaryWitness {
		v.Center = point(v.Center, false)
		v.Numerator = point(v.Numerator, true)
		v.Radius = rat(v.Radius)
		v.Radicand = rat(v.Radicand)
		v.Root = roots(v.Root)
		return v
	}
	cavity := func(v geom.SphereCavityNearest) geom.SphereCavityNearest {
		v.Lower = rat(v.Lower)
		v.Upper = rat(v.Upper)
		for i := range v.Candidates {
			v.Candidates[i] = full(v.Candidates[i])
		}
		return v
	}
	w.Lower = rat(b.Lower)
	w.Upper = rat(b.Upper)
	if b.Separated != nil {
		v := *b.Separated
		v.Lower = rat(v.Lower)
		v.Upper = rat(v.Upper)
		for i := range v.Candidates {
			v.Candidates[i] = full(v.Candidates[i])
		}
		w.Separated = &v
	}
	if b.Cavity != nil {
		v := cavity(*b.Cavity)
		w.Cavity = &v
	}
	if b.Single != nil {
		v := *b.Single
		v.Lower = rat(v.Lower)
		v.Upper = rat(v.Upper)
		v.Witness = full(v.Witness)
		w.Single = &v
	}
	if b.Touching != nil {
		v := *b.Touching
		v.Boundary = cavity(v.Boundary)
		v.Contact = point(v.Contact, false)
		w.Touching = &v
	}
	if b.Isolated != nil {
		v := *b.Isolated
		v.Lower = rat(v.Lower)
		v.Upper = rat(v.Upper)
		for i := range v.Candidates {
			c := &v.Candidates[i]
			c.Center = point(c.Center, false)
			c.Numerator = point(c.Numerator, true)
			c.Root = roots(c.Root)
			c.Radicand = rat(c.Radicand)
			c.Constant = rat(c.Constant)
			c.Coefficient = rat(c.Coefficient)
			c.Lower = rat(c.Lower)
			c.Upper = rat(c.Upper)
		}
		w.Isolated = &v
	}
	if b.Axis != nil {
		v := *b.Axis
		v.Lower = rat(v.Lower)
		v.Upper = rat(v.Upper)
		for i := range v.Families {
			f := &v.Families[i]
			f.Center = point(f.Center, false)
			f.Normal = point(f.Normal, true)
			f.PlanePoint = point(f.PlanePoint, false)
			f.RadiusSquared = rat(f.RadiusSquared)
			f.Constant = rat(f.Constant)
			f.Coefficient = rat(f.Coefficient)
			f.Radicand = rat(f.Radicand)
			f.Lower = rat(f.Lower)
			f.Upper = rat(f.Upper)
			if f.Pole != nil {
				p := *f.Pole
				p.Center = point(p.Center, false)
				p.Numerator = point(p.Numerator, true)
				p.Radicand = rat(p.Radicand)
				p.Root = roots(p.Root)
				f.Pole = &p
			}
		}
		w.Axis = &v
	}
	if err != nil {
		return geom.SpherePairNearest{}, err
	}
	return w, nil
}
