package geom

import (
	"context"
	"math/big"
)

// FiniteSphereCylinderBranch describes all points on one physical axial-height
// sheet z-zSphere=HeightSign*sqrt(A+2*r*(Projection·ray)). A unit radial ray is
// retained iff its projection lies in [ProjectionMin,ProjectionMax]. This
// exact interval is implicitly clipped by the radial circle itself.
type FiniteSphereCylinderBranch struct {
	HeightSign                   int
	Present                      bool
	ProjectionMin, ProjectionMax *big.Rat
}
type FiniteSphereCylinderComponent struct {
	HalfBranches []int
	Closed       bool
}

// HalfBranches use index2*branch+half; branch0 is negative physical height,
// branch1 positive. Half0/1 are the two sides of the projected-center ray.
// They join at included radial projection extrema and at regular sphere folds.
// DomainsComplete includes tangent/zero-width pieces; ConnectivityComplete
// excludes them until a dedicated singular graph proof is available.
type FiniteSphereCylinderIntersection struct {
	// CapHeightSigns assigns each cap event to its physical height sheet; zero
	// is the shared fold and retains both sheet owners.
	CapHeightSigns                        [2]int
	DomainsComplete, ConnectivityComplete bool
	Angular                               SphereCylinderAngularDomain
	Branches                              [2]FiniteSphereCylinderBranch
	CapEvents                             [2]SphereCylinderCapIntersection
	Components                            []FiniteSphereCylinderComponent
	ExactOperations                       int
}

// IntersectFiniteOffAxisSphereCylinder gives complete exact finite-wall sheet
// domains and cap events. For nondegenerate sheets it additionally certifies
// connected components and whether each is closed. Coaxial input is unsupported
// here (use IntersectCoaxialSphereCylinder). A global tangent node, isolated
// point or a sheet reduced to zero angular width keeps exact domains/events but
// returns ConnectivityComplete=false, rather than inventing branch ownership.
func IntersectFiniteOffAxisSphereCylinder(ctx context.Context, center Vec3, sphereRadius float64, origin, axis Vec3, radius float64, window [2]*big.Rat, o SurfacePatchEvaluationOptions) (out FiniteSphereCylinderIntersection, err error) {
	if ctx == nil || window[0] == nil || window[1] == nil || window[0].Cmp(window[1]) >= 0 {
		return out, ErrInvalid
	}
	ops := o.MaxOperations
	if ops == 0 {
		ops = 10000
	}
	if ops < 1 {
		return out, ErrInvalid
	}
	bits := o.MaxCoefficientBits
	if bits == 0 {
		bits = 16384
	}
	b := &polynomialBudget{maxOperations: ops, maxBits: bits}
	defer func() {
		out.ExactOperations = b.operations
		if err != nil {
			out.DomainsComplete = false
			out.ConnectivityComplete = false
			out.Components = nil
		}
	}()
	callOptions := func() (SurfacePatchEvaluationOptions, error) {
		if b.operations >= ops {
			return o, ErrConvergence
		}
		q := o
		q.MaxOperations = ops - b.operations
		return q, nil
	}
	oo, e := callOptions()
	if e != nil {
		return out, e
	}
	out.Angular, e = ClassifySphereCylinderAngularDomain(ctx, center, sphereRadius, origin, axis, radius, oo)
	b.operations += out.Angular.ExactOperations
	if e != nil {
		return out, e
	}
	if out.Angular.ProjectionSquared.Sign() == 0 {
		return out, ErrDomain
	}
	for i := 0; i < 2; i++ {
		oo, e = callOptions()
		if e != nil {
			return out, e
		}
		out.CapEvents[i], e = IntersectSphereCylinderCap(ctx, center, sphereRadius, origin, axis, radius, window[i], oo)
		b.operations += out.CapEvents[i].ExactOperations
		if e != nil {
			return out, e
		}
	}
	ax := 0
	if axis.Y != 0 {
		ax = 1
	} else if axis.Z != 0 {
		ax = 2
	}
	r := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	delta := new(big.Rat).Sub(r(coordinate(origin, ax)), r(coordinate(center, ax)))
	lo := new(big.Rat).Add(delta, new(big.Rat).Mul(r(coordinate(axis, ax)), window[0]))
	hi := new(big.Rat).Add(delta, new(big.Rat).Mul(r(coordinate(axis, ax)), window[1]))
	out.CapHeightSigns = [2]int{lo.Sign(), hi.Sign()}
	if lo.Cmp(hi) > 0 {
		lo, hi = hi, lo
	}
	H := out.Angular.ProjectionSquared
	// Compare a rational to sign*sqrt(H), exactly.
	cmpExtreme := func(q *big.Rat, sign int) int {
		if sign > 0 {
			if q.Sign() < 0 {
				return -1
			}
			return new(big.Rat).Mul(q, q).Cmp(H)
		}
		if q.Sign() > 0 {
			return 1
		}
		return -new(big.Rat).Mul(q, q).Cmp(H)
	}
	active := [4]bool{}
	parent := [4]int{0, 1, 2, 3}
	open := [4]int{}
	find := func(i int) int {
		for parent[i] != i {
			i = parent[i]
		}
		return i
	}
	join := func(i, j int) { parent[find(i)] = find(j) }
	regular := out.Angular.Topology != SphereCylinderTwoLoopsAtNode && out.Angular.Topology != SphereCylinderIsolatedPoint
	fold := out.Angular.Threshold
	hasFold := lo.Sign() <= 0 && hi.Sign() >= 0 && cmpExtreme(fold, -1) > 0 && cmpExtreme(fold, 1) < 0
	for i, sign := range []int{-1, 1} {
		out.Branches[i].HeightSign = sign
		l, h := new(big.Rat).Set(lo), new(big.Rat).Set(hi)
		if sign < 0 {
			l, h = new(big.Rat).Neg(hi), new(big.Rat).Neg(lo)
		}
		if h.Sign() < 0 {
			continue
		}
		if l.Sign() < 0 {
			l.SetInt64(0)
		}
		qlo := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(l, l), out.Angular.A), new(big.Rat).Mul(big.NewRat(2, 1), r(radius)))
		qhi := new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(h, h), out.Angular.A), new(big.Rat).Mul(big.NewRat(2, 1), r(radius)))
		for _, v := range []*big.Rat{qlo, qhi} {
			if e = b.check(v); e != nil {
				return out, e
			}
		}
		branch := &out.Branches[i]
		branch.ProjectionMin = qlo
		branch.ProjectionMax = qhi
		if cmpExtreme(qlo, 1) > 0 || cmpExtreme(qhi, -1) < 0 {
			continue
		}
		branch.Present = true
		if qlo.Cmp(qhi) == 0 || cmpExtreme(qlo, 1) == 0 || cmpExtreme(qhi, -1) == 0 {
			regular = false
			continue
		}
		active[2*i] = true
		active[2*i+1] = true
		lowerExtreme := cmpExtreme(qlo, -1) <= 0
		upperExtreme := cmpExtreme(qhi, 1) >= 0
		if lowerExtreme || upperExtreme {
			join(2*i, 2*i+1)
		}
		for half := 0; half < 2; half++ {
			if !lowerExtreme && !(hasFold && qlo.Cmp(fold) == 0) {
				open[2*i+half]++
			}
			if !upperExtreme {
				open[2*i+half]++
			}
		}
	}
	out.DomainsComplete = true
	if !regular {
		return out, nil
	}
	if hasFold && active[0] && active[2] {
		join(0, 2)
		join(1, 3)
	}
	groups := map[int]int{}
	for i, on := range active {
		if !on {
			continue
		}
		root := find(i)
		j, ok := groups[root]
		if !ok {
			j = len(out.Components)
			groups[root] = j
			out.Components = append(out.Components, FiniteSphereCylinderComponent{Closed: true})
		}
		out.Components[j].HalfBranches = append(out.Components[j].HalfBranches, i)
		if open[i] > 0 {
			out.Components[j].Closed = false
		}
	}
	out.ConnectivityComplete = true
	return out, nil
}
