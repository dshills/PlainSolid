package parametric

import (
	"context"
	"plainsolid/geom"
)

// FiniteTransverseCylinderSet binds exact regularized point semantics to a
// successfully constructed native strict-transverse parallel cylinder set.
// Geometry and query data are immutable; the source operands are not retained
// by reference. A reconstructed arbitrary Solid is not implicitly certified.
type FiniteTransverseCylinderSet struct {
	solid Solid
	query geom.TransverseCylinderSetQuery
	valid bool
}

// NewFiniteTransverseCylinderSet validates the exact query domain and constructs
// the native Union or Difference before publishing a bound query.
func NewFiniteTransverseCylinderSet(ctx context.Context, a, b geom.RawCylinder, op BooleanOp, tolerance float64) (FiniteTransverseCylinderSet, error) {
	var zero FiniteTransverseCylinderSet
	if op != Union && op != Difference {
		return zero, geom.ErrDomain
	}
	q, e := geom.NewTransverseCylinderSetQuery(a, b, op == Difference)
	if e != nil {
		return zero, e
	}
	var solid Solid
	if op == Union {
		r, e := FiniteCylinderPairUnion(ctx, a, b, tolerance)
		if e != nil {
			return zero, e
		}
		solid = r.Solid
	} else {
		r, e := FiniteCylinderPairDifference(ctx, a, b, tolerance)
		if e != nil {
			return zero, e
		}
		if len(r.Solids[1].Faces()) != 0 {
			return zero, geom.ErrDomain
		}
		solid = r.Solids[0]
	}
	return FiniteTransverseCylinderSet{solid: solid, query: q, valid: true}, nil
}

// Solid returns the immutable native boundary certified at construction.
func (s FiniteTransverseCylinderSet) Solid() Solid { return s.solid }

// Classify uses exact represented coordinates, without a tolerance threshold.
func (s FiniteTransverseCylinderSet) Classify(p geom.Vec3) (geom.CylinderSetPoint, error) {
	if !s.valid {
		return geom.CylinderSetPoint{}, geom.ErrInvalid
	}
	return s.query.Classify(p)
}

// ClipLine returns regularized material and separate original support incidence
// on the caller's finite raw line. It never substitutes closed-set difference.
func (s FiniteTransverseCylinderSet) ClipLine(ctx context.Context, origin, direction geom.Vec3, minT, maxT float64, o geom.ToroidLineOptions) (geom.TransverseCylinderMaterialLine, error) {
	if !s.valid {
		return geom.TransverseCylinderMaterialLine{}, geom.ErrInvalid
	}
	return s.query.ClipLine(ctx, origin, direction, minT, maxT, o)
}

// FirstBoundaryHit preserves exact algebraic hit identity, outward rational
// model enclosures and all exposed oriented source normals on 0<=t<=maxT.
func (s FiniteTransverseCylinderSet) FirstBoundaryHit(ctx context.Context, origin, direction geom.Vec3, maxT float64, o geom.ToroidLineOptions) (geom.TransverseCylinderFirstHit, error) {
	if !s.valid {
		return geom.TransverseCylinderFirstHit{}, geom.ErrInvalid
	}
	return s.query.FirstBoundaryHit(ctx, origin, direction, maxT, o)
}
