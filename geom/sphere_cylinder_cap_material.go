package geom

import (
	"context"
	"math/big"
)

type SphereCylinderCapMaterialKind uint8

const (
	SphereCylinderCapEmpty SphereCylinderCapMaterialKind = iota
	SphereCylinderCapPointMaterial
	SphereCylinderCapCylinderDisk
	SphereCylinderCapSphereDisk
	SphereCylinderCapCoincidentDisk
	SphereCylinderCapLens
)

type SphereCylinderCapCircleOwner uint8

const (
	CapCylinderCircle SphereCylinderCapCircleOwner = iota
	CapSphereCircle
)

// SphereCylinderCapBoundaryArc is positively oriented around the signed raw
// axis (material on the left). Partial arcs refer to exact event indices; full
// circles use -1 endpoints. ProjectionSense +1/-1 retains respectively >=/<=
// RadicalAxisValue in Projection·(point-CylinderCenter). Together with Owner
// this determines the entire arc, without ambiguous endpoint-only selection.
type SphereCylinderCapBoundaryArc struct {
	Owner                SphereCylinderCapCircleOwner
	Whole                bool
	StartEvent, EndEvent int
	ProjectionSense      int
}
type SphereCylinderCapMaterial struct {
	Complete                                                     bool
	Kind                                                         SphereCylinderCapMaterialKind
	Tangent                                                      bool
	CylinderCenter, SphereSectionCenter                          [3]*big.Rat
	CylinderRadiusSquared, SphereRadiusSquared, RadicalAxisValue *big.Rat
	Projection                                                   [3]*big.Rat
	Events                                                       SphereCylinderCapIntersection
	Boundary                                                     []SphereCylinderCapBoundaryArc
	ExactOperations                                              int
}

// ClassifySphereCylinderCapMaterial classifies the closed intersection of the
// cylindrical cap disk and sphere material. Sphere circle radius is retained
// squared, so a nonrepresentable square root never changes the geometry. It
// covers empty/point/disk/lens outcomes, both internal and external tangency,
// and coincident circles. Axis is coordinate-aligned with arbitrary scale.
// Only intersection material is classified; difference/union need distinct
// orientation and containment rules.
func ClassifySphereCylinderCapMaterial(ctx context.Context, center Vec3, sphereRadius float64, origin, axis Vec3, radius float64, capParameter *big.Rat, o SurfacePatchEvaluationOptions) (out SphereCylinderCapMaterial, err error) {
	events, e := IntersectSphereCylinderCap(ctx, center, sphereRadius, origin, axis, radius, capParameter, o)
	out.Events = events
	out.ExactOperations = events.ExactOperations
	if e != nil {
		return out, e
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
	}
	if bits == 0 {
		bits = 16384
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits, operations: events.ExactOperations}
	defer func() {
		out.ExactOperations = b.operations
		if err != nil {
			out.Complete = false
			out.Boundary = nil
		}
	}()
	ax := 0
	if axis.Y != 0 {
		ax = 1
	} else if axis.Z != 0 {
		ax = 2
	}
	r := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	H := new(big.Rat)
	for i := 0; i < 3; i++ {
		out.CylinderCenter[i] = new(big.Rat).Add(r(coordinate(origin, i)), new(big.Rat).Mul(r(coordinate(axis, i)), capParameter))
		out.SphereSectionCenter[i] = r(coordinate(center, i))
		if i == ax {
			out.SphereSectionCenter[i].Set(out.CylinderCenter[i])
		}
		out.Projection[i] = new(big.Rat).Sub(out.SphereSectionCenter[i], out.CylinderCenter[i])
		H.Add(H, new(big.Rat).Mul(out.Projection[i], out.Projection[i]))
	}
	dz := new(big.Rat).Sub(out.CylinderCenter[ax], r(coordinate(center, ax)))
	S := new(big.Rat).Sub(new(big.Rat).Mul(r(sphereRadius), r(sphereRadius)), new(big.Rat).Mul(dz, dz))
	C := new(big.Rat).Mul(r(radius), r(radius))
	N := new(big.Rat).Sub(new(big.Rat).Add(C, H), S)
	D := new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(C, H)), new(big.Rat).Mul(N, N))
	out.SphereRadiusSquared = S
	out.CylinderRadiusSquared = C
	out.RadicalAxisValue = new(big.Rat).Quo(N, big.NewRat(2, 1))
	for _, q := range []*big.Rat{H, S, C, N, D, out.RadicalAxisValue} {
		if e = b.check(q); e != nil {
			return out, e
		}
	}
	whole := func(owner SphereCylinderCapCircleOwner) {
		out.Boundary = []SphereCylinderCapBoundaryArc{{Owner: owner, Whole: true, StartEvent: -1, EndEvent: -1}}
	}
	if S.Sign() < 0 {
		out.Complete = true
		return out, nil
	}
	if S.Sign() == 0 {
		if H.Cmp(C) <= 0 {
			out.Kind = SphereCylinderCapPointMaterial
			out.Tangent = true
		}
		out.Complete = true
		return out, nil
	}
	if H.Sign() == 0 {
		switch S.Cmp(C) {
		case -1:
			out.Kind = SphereCylinderCapSphereDisk
			whole(CapSphereCircle)
		case 0:
			out.Kind = SphereCylinderCapCoincidentDisk
			whole(CapCylinderCircle)
		case 1:
			out.Kind = SphereCylinderCapCylinderDisk
			whole(CapCylinderCircle)
		}
		out.Complete = true
		return out, nil
	}
	switch D.Sign() {
	case 1:
		if len(events.Points) != 2 {
			return out, ErrConvergence
		}
		out.Kind = SphereCylinderCapLens
		out.Boundary = []SphereCylinderCapBoundaryArc{{Owner: CapCylinderCircle, StartEvent: 0, EndEvent: 1, ProjectionSense: 1}, {Owner: CapSphereCircle, StartEvent: 1, EndEvent: 0, ProjectionSense: -1}}
	case 0:
		out.Tangent = true
		if len(events.Points) != 1 {
			return out, ErrConvergence
		}
		if N.Sign() < 0 {
			out.Kind = SphereCylinderCapCylinderDisk
			whole(CapCylinderCircle)
		} else if H.Cmp(C) < 0 {
			out.Kind = SphereCylinderCapSphereDisk
			whole(CapSphereCircle)
		} else {
			out.Kind = SphereCylinderCapPointMaterial
		}
	case -1:
		if N.Sign() < 0 {
			out.Kind = SphereCylinderCapCylinderDisk
			whole(CapCylinderCircle)
		} else if H.Cmp(C) < 0 {
			out.Kind = SphereCylinderCapSphereDisk
			whole(CapSphereCircle)
		}
	}
	out.Complete = true
	return out, nil
}
