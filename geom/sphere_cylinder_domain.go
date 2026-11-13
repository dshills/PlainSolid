package geom

import (
	"context"
	"math/big"
)

// SphereCylinderTopology classifies the complete ideal infinite cylindrical
// support intersection, without finite axial clipping or tolerance welding.
type SphereCylinderTopology uint8

const (
	SphereCylinderEmpty SphereCylinderTopology = iota
	SphereCylinderIsolatedPoint
	SphereCylinderOneClosedCurve
	SphereCylinderTwoClosedCurves
	SphereCylinderTwoLoopsAtNode
	SphereCylinderEquatorialCircle
)

// SphereCylinderAngularRay is an exact unit radial direction
// Base+RootSign*sqrt(Radicand)*Vector, with outward component bounds.
type SphereCylinderAngularRay struct {
	Base, Vector [3]*big.Rat
	Radicand     *big.Rat
	RootSign     int
	Bounds       [3][2]*big.Rat
}

// SphereCylinderAngularDomain proves the domain D=A+2*r*(Projection·ray)>=0
// on the unit radial circle. BoundaryRays are D=0 directions. At two simple
// folds the surface intersection is regular (OneClosedCurve); a unique max
// zero is an isolated point, and a unique min zero is a four-germ node joining
// two loops. At that node the surfaces have parallel normals, but the two
// curve tangent lines are distinct: in local angular displacement delta,
// z-z0 = +/-sqrt(cylinderRadius*|Projection|)*delta + O(delta^3).
// The latter is not an embedded manifold. EquatorialCircle has
// tangent surfaces along the whole circle. Outputs are caller-owned.
type SphereCylinderAngularDomain struct {
	Complete                        bool
	Topology                        SphereCylinderTopology
	A, Threshold, ProjectionSquared *big.Rat
	Projection                      [3]*big.Rat
	BoundaryRays                    []SphereCylinderAngularRay
	ExactOperations                 int
}

// ClassifySphereCylinderAngularDomain derives complete angular branch topology
// for an arbitrary represented sphere and an infinite coordinate-axis circular
// cylinder. Raw axis may have any nonzero represented scale/sign. Projection
// and threshold use model lengths, not raw axis t. No finite patch topology or
// rounded display embedding is asserted. Positive radii are required.
func ClassifySphereCylinderAngularDomain(ctx context.Context, center Vec3, sphereRadius float64, axisOrigin, axis Vec3, cylinderRadius float64, o SurfacePatchEvaluationOptions) (out SphereCylinderAngularDomain, err error) {
	if ctx == nil || !center.IsFinite() || !axisOrigin.IsFinite() || !axis.IsFinite() || !finite(sphereRadius) || !finite(cylinderRadius) || sphereRadius <= 0 || cylinderRadius <= 0 {
		return out, ErrInvalid
	}
	ax := -1
	for i := 0; i < 3; i++ {
		if coordinate(axis, i) != 0 {
			if ax >= 0 {
				return out, ErrDomain
			}
			ax = i
		}
	}
	if ax < 0 {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	ops, bits := o.MaxOperations, o.MaxCoefficientBits
	if ops == 0 {
		ops = 10000
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
			out.Complete = false
			out.BoundaryRays = nil
		}
	}()
	r := func(v float64) *big.Rat { return new(big.Rat).SetFloat64(v) }
	h := new(big.Rat)
	for i := 0; i < 3; i++ {
		out.Projection[i] = new(big.Rat)
		if i != ax {
			out.Projection[i].Sub(r(coordinate(center, i)), r(coordinate(axisOrigin, i)))
		}
		h.Add(h, new(big.Rat).Mul(out.Projection[i], out.Projection[i]))
		if e := b.check(h); e != nil {
			return out, e
		}
	}
	rr := new(big.Rat).Mul(r(cylinderRadius), r(cylinderRadius))
	A := new(big.Rat).Sub(new(big.Rat).Sub(new(big.Rat).Mul(r(sphereRadius), r(sphereRadius)), rr), h)
	K := new(big.Rat).Quo(new(big.Rat).Neg(A), new(big.Rat).Mul(big.NewRat(2, 1), r(cylinderRadius)))
	out.A = A
	out.Threshold = K
	out.ProjectionSquared = new(big.Rat).Set(h)
	for _, v := range []*big.Rat{A, K, rr} {
		if e := b.check(v); e != nil {
			return out, e
		}
	}
	if h.Sign() == 0 {
		switch A.Sign() {
		case -1:
			out.Topology = SphereCylinderEmpty
		case 0:
			out.Topology = SphereCylinderEquatorialCircle
		case 1:
			out.Topology = SphereCylinderTwoClosedCurves
		}
		out.Complete = true
		return out, nil
	}
	disc := new(big.Rat).Sub(h, new(big.Rat).Mul(K, K))
	if e := b.check(disc); e != nil {
		return out, e
	}
	if disc.Sign() < 0 {
		if A.Sign() > 0 {
			out.Topology = SphereCylinderTwoClosedCurves
		} else {
			out.Topology = SphereCylinderEmpty
		}
		out.Complete = true
		return out, nil
	}
	if disc.Sign() > 0 {
		out.Topology = SphereCylinderOneClosedCurve
	} else if A.Sign() > 0 {
		out.Topology = SphereCylinderTwoLoopsAtNode
	} else {
		out.Topology = SphereCylinderIsolatedPoint
	}
	rad := new(big.Rat).Quo(disc, new(big.Rat).Mul(h, h))
	lo, hi, e := rationalSqrtBounds(rad, 96, b)
	if e != nil {
		return out, e
	}
	signs := []int{-1, 1}
	if rad.Sign() == 0 {
		signs = []int{0}
	}
	j, k := (ax+1)%3, (ax+2)%3
	orientation := int64(1)
	if coordinate(axis, ax) < 0 {
		orientation = -1
	}
	for _, sign := range signs {
		q := SphereCylinderAngularRay{Radicand: new(big.Rat).Set(rad), RootSign: sign}
		for i := 0; i < 3; i++ {
			q.Base[i] = new(big.Rat).Quo(new(big.Rat).Mul(K, out.Projection[i]), h)
			q.Vector[i] = new(big.Rat)
		}
		q.Vector[j].Mul(big.NewRat(-orientation, 1), out.Projection[k])
		q.Vector[k].Mul(big.NewRat(orientation, 1), out.Projection[j])
		for i := 0; i < 3; i++ {
			v := new(big.Rat).Mul(big.NewRat(int64(sign), 1), q.Vector[i])
			l, u := lo, hi
			if v.Sign() < 0 {
				l, u = u, l
			}
			q.Bounds[i] = [2]*big.Rat{new(big.Rat).Add(q.Base[i], new(big.Rat).Mul(v, l)), new(big.Rat).Add(q.Base[i], new(big.Rat).Mul(v, u))}
			for _, value := range []*big.Rat{q.Base[i], q.Vector[i], q.Bounds[i][0], q.Bounds[i][1]} {
				if e = b.check(value); e != nil {
					return out, e
				}
			}
		}
		out.BoundaryRays = append(out.BoundaryRays, q)
	}
	out.Complete = true
	return out, nil
}
