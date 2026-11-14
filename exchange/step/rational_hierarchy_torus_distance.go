package step

import (
	"bytes"
	"context"
	"math/big"
	"plainsolid/geom"
	"reflect"
)

// HierarchyTorusNearestSet is a unique point, a complete meridian circle, or a
// complete axis circle. Algebraic equations retain the original local query;
// no arbitrary representative is selected for nonunique sets.
type HierarchyTorusNearestSet struct {
	kind                             string // "point", "meridian-circle", "axis-circle"
	local, origin, axis, exactCenter [3]*big.Rat
	rotation                         [3][3]*big.Rat
	major, minor                     *big.Rat
	inverseRadial, tubeScale         *geom.ExactAlgebraicRoot
	exactPoint                       [3]*big.Rat
}

func (s HierarchyTorusNearestSet) Kind() string            { return s.kind }
func (s HierarchyTorusNearestSet) LocalQuery() [3]*big.Rat { return copyHierarchyRats(s.local) }
func (s HierarchyTorusNearestSet) MajorRadius() *big.Rat {
	if s.major == nil {
		return nil
	}
	return new(big.Rat).Set(s.major)
}
func (s HierarchyTorusNearestSet) MinorRadius() *big.Rat {
	if s.minor == nil {
		return nil
	}
	return new(big.Rat).Set(s.minor)
}
func (s HierarchyTorusNearestSet) InverseRadialRoot() *geom.ExactAlgebraicRoot {
	return s.inverseRadial
}
func (s HierarchyTorusNearestSet) TubeScaleRoot() *geom.ExactAlgebraicRoot { return s.tubeScale }

type HierarchyTorusCircleEnclosure struct {
	CenterLower, CenterUpper, Axis [3]*big.Rat
	RadiusLower, RadiusUpper       *big.Rat
	Operations, Subdivisions       int
}
type torusDistanceInterval struct{ lo, hi *big.Rat }

func torusDistanceConstant(q *big.Rat) torusDistanceInterval {
	return torusDistanceInterval{new(big.Rat).Set(q), new(big.Rat).Set(q)}
}
func torusDistanceAdd(a *hierarchyPointArithmetic, x, y torusDistanceInterval) torusDistanceInterval {
	return torusDistanceInterval{a.add(x.lo, y.lo), a.add(x.hi, y.hi)}
}
func torusDistanceSub(a *hierarchyPointArithmetic, x, y torusDistanceInterval) torusDistanceInterval {
	return torusDistanceInterval{a.sub(x.lo, y.hi), a.sub(x.hi, y.lo)}
}
func torusDistanceMul(a *hierarchyPointArithmetic, x, y torusDistanceInterval) torusDistanceInterval {
	q := []*big.Rat{a.mul(x.lo, y.lo), a.mul(x.lo, y.hi), a.mul(x.hi, y.lo), a.mul(x.hi, y.hi)}
	lo, hi := q[0], q[0]
	for _, r := range q[1:] {
		if r.Cmp(lo) < 0 {
			lo = r
		}
		if r.Cmp(hi) > 0 {
			hi = r
		}
	}
	return torusDistanceInterval{lo, hi}
}

// enclose retains independent root intervals, which can overestimate correlated
// expressions but never shrink the true range. All refinement work is charged.
func (s HierarchyTorusNearestSet) enclose(ctx context.Context, width *big.Rat, o geom.ExactCurvePlaneEventOptions, circle bool) (HierarchySphereNearestEnclosure, HierarchyTorusCircleEnclosure, error) {
	var point HierarchySphereNearestEnclosure
	var ring HierarchyTorusCircleEnclosure
	fail := func(e error) (HierarchySphereNearestEnclosure, HierarchyTorusCircleEnclosure, error) {
		return HierarchySphereNearestEnclosure{}, HierarchyTorusCircleEnclosure{}, e
	}
	if ctx == nil || width == nil || width.Sign() <= 0 || s.major == nil || s.minor == nil {
		return fail(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return fail(e)
	}
	if circle != (s.kind != "point") {
		return fail(geom.ErrDomain)
	}
	if s.kind != "point" && s.kind != "axis-circle" && s.kind != "meridian-circle" {
		return fail(geom.ErrInvalid)
	}
	opts, e := sphereDistanceRootOptions(o)
	if e != nil {
		return fail(e)
	}
	a := hierarchyPointArithmetic{bits: opts.MaxCoefficientBits}
	a.check(width)
	a.check(s.major)
	a.check(s.minor)
	for _, v := range [][3]*big.Rat{s.local, s.origin, s.axis} {
		for _, q := range v {
			a.check(q)
		}
	}
	for _, row := range s.rotation {
		for _, q := range row {
			a.check(q)
		}
	}
	if a.err != nil {
		return fail(a.err)
	}
	if s.kind == "meridian-circle" {
		ring.CenterLower = copyHierarchyRats(s.exactCenter)
		ring.CenterUpper = copyHierarchyRats(s.exactCenter)
		ring.Axis = copyHierarchyRats(s.axis)
		ring.RadiusLower = new(big.Rat).Set(s.minor)
		ring.RadiusUpper = new(big.Rat).Set(s.minor)
		for _, q := range ring.CenterLower {
			a.check(q)
		}
		if a.err != nil {
			return fail(a.err)
		}
		return point, ring, nil
	}
	if s.kind == "point" && s.exactPoint[0] != nil {
		point.Lower = copyHierarchyRats(s.exactPoint)
		point.Upper = copyHierarchyRats(s.exactPoint)
		for _, q := range point.Lower {
			a.check(q)
		}
		if a.err != nil {
			return fail(a.err)
		}
		return point, ring, nil
	}
	if s.tubeScale == nil || s.kind == "point" && s.inverseRadial == nil {
		return fail(geom.ErrInvalid)
	}
	operations, subdivisions := 0, 0
	target := new(big.Rat).Set(width)
	if target.Cmp(big.NewRat(1, 1)) > 0 {
		target.SetInt64(1)
	}
	rootInterval := func(root *geom.ExactAlgebraicRoot) (torusDistanceInterval, error) {
		q := opts
		q.MaxOperations -= operations
		q.MaxSubdivisions -= subdivisions
		if q.MaxOperations < 1 || q.MaxSubdivisions < 1 {
			return torusDistanceInterval{}, geom.ErrConvergence
		}
		r, e := root.Enclose(ctx, target, q)
		operations += r.Operations
		subdivisions += r.Subdivisions
		if e != nil {
			return torusDistanceInterval{}, e
		}
		if !r.Converged {
			return torusDistanceInterval{}, geom.ErrConvergence
		}
		return torusDistanceInterval{r.Interval.Lower, r.Interval.Upper}, nil
	}
	for attempt := 0; attempt < 256; attempt++ {
		if e = ctx.Err(); e != nil {
			return fail(e)
		}
		operations += 512
		if operations >= opts.MaxOperations {
			return fail(geom.ErrConvergence)
		}
		lambda, e := rootInterval(s.tubeScale)
		if e != nil {
			return fail(e)
		}
		var local [3]torusDistanceInterval
		radius := torusDistanceConstant(s.minor)
		if circle {
			local[0] = torusDistanceConstant(new(big.Rat))
			local[1] = torusDistanceConstant(new(big.Rat))
			local[2] = torusDistanceMul(&a, lambda, torusDistanceConstant(s.local[2]))
			radius = torusDistanceMul(&a, torusDistanceConstant(s.major), torusDistanceSub(&a, torusDistanceConstant(big.NewRat(1, 1)), lambda))
		} else {
			beta, e := rootInterval(s.inverseRadial)
			if e != nil {
				return fail(e)
			}
			b := torusDistanceMul(&a, torusDistanceConstant(s.major), beta)
			factor := torusDistanceSub(&a, torusDistanceAdd(&a, b, lambda), torusDistanceMul(&a, b, lambda))
			local[0] = torusDistanceMul(&a, torusDistanceConstant(s.local[0]), factor)
			local[1] = torusDistanceMul(&a, torusDistanceConstant(s.local[1]), factor)
			local[2] = torusDistanceMul(&a, torusDistanceConstant(s.local[2]), lambda)
		}
		var world [3]torusDistanceInterval
		ok := true
		for i := range world {
			world[i] = torusDistanceConstant(s.origin[i])
			for j := 0; j < 3; j++ {
				world[i] = torusDistanceAdd(&a, world[i], torusDistanceMul(&a, torusDistanceConstant(s.rotation[i][j]), local[j]))
			}
			if a.sub(world[i].hi, world[i].lo).Cmp(width) > 0 {
				ok = false
			}
		}
		if circle && (radius.lo.Sign() <= 0 || a.sub(radius.hi, radius.lo).Cmp(width) > 0) {
			ok = false
		}
		if a.err != nil {
			return fail(a.err)
		}
		if ok {
			if circle {
				for i := range world {
					ring.CenterLower[i] = new(big.Rat).Set(world[i].lo)
					ring.CenterUpper[i] = new(big.Rat).Set(world[i].hi)
				}
				ring.Axis = copyHierarchyRats(s.axis)
				ring.RadiusLower = new(big.Rat).Set(radius.lo)
				ring.RadiusUpper = new(big.Rat).Set(radius.hi)
				ring.Operations, ring.Subdivisions = operations, subdivisions
			} else {
				for i := range world {
					point.Lower[i] = new(big.Rat).Set(world[i].lo)
					point.Upper[i] = new(big.Rat).Set(world[i].hi)
				}
				point.Operations, point.Subdivisions = operations, subdivisions
			}
			return point, ring, nil
		}
		target = a.mul(target, big.NewRat(1, 2))
		if a.err != nil {
			return fail(a.err)
		}
	}
	return fail(geom.ErrConvergence)
}

// EnclosePoint returns a certified unique point box, or ErrDomain for a circle.
func (s HierarchyTorusNearestSet) EnclosePoint(ctx context.Context, width *big.Rat, o geom.ExactCurvePlaneEventOptions) (HierarchySphereNearestEnclosure, error) {
	p, _, e := s.enclose(ctx, width, o, false)
	return p, e
}

// EncloseCircle returns a full nearest circle's center/radius enclosures and
// exact proper axis, or ErrDomain for a unique point.
func (s HierarchyTorusNearestSet) EncloseCircle(ctx context.Context, width *big.Rat, o geom.ExactCurvePlaneEventOptions) (HierarchyTorusCircleEnclosure, error) {
	_, r, e := s.enclose(ctx, width, o, true)
	return r, e
}

type HierarchyTorusDistanceLeaf struct {
	ProductID, SourceProductID, SourceBodyID, SourceFaceID int64
	Path                                                   []int64
	Supported                                              bool
	UnsupportedReason                                      string
	Location                                               AnalyticPointLocation
	NearestBoundary                                        HierarchyTorusNearestSet
	boundary, material                                     *geom.ExactAlgebraicRoot
}

func (l HierarchyTorusDistanceLeaf) BoundarySquaredDistance() *geom.ExactAlgebraicRoot {
	return l.boundary
}
func (l HierarchyTorusDistanceLeaf) MaterialSquaredDistance() *geom.ExactAlgebraicRoot {
	return l.material
}

type RationalHierarchyTorusDistance struct {
	Leaves                   []HierarchyTorusDistanceLeaf
	LengthUnit               float64
	Operations, Subdivisions int
}
type RationalHierarchyTorusDistanceOptions struct {
	MaxOperations, MaxCoefficientBits, MaxSubdivisions int
	Source                                             Limits
}

// DistanceRationalHierarchyTori proves ring-torus distance per occurrence.
func DistanceRationalHierarchyTori(ctx context.Context, m RationalSphericalHierarchy, p [3]*big.Rat, o RationalHierarchyTorusDistanceOptions) (RationalHierarchyTorusDistance, error) {
	return distanceHierarchyTori(ctx, m, p, o, false)
}

// DistanceOCCTRationalHierarchyTori uses the explicit OCCT frame convention.
func DistanceOCCTRationalHierarchyTori(ctx context.Context, m OCCTRationalSphericalHierarchy, p [3]*big.Rat, o RationalHierarchyTorusDistanceOptions) (RationalHierarchyTorusDistance, error) {
	return distanceHierarchyTori(ctx, RationalSphericalHierarchy(m), p, o, true)
}

// TorusDistance retains source ancestry from the affine-pcurve hierarchy.
func (m AffinePCurveHierarchy) TorusDistance(ctx context.Context, p [3]*big.Rat, o RationalHierarchyTorusDistanceOptions) (RationalHierarchyTorusDistance, error) {
	return distanceHierarchyTori(ctx, m.model, p, o, m.vendor)
}
func distanceHierarchyTori(ctx context.Context, m RationalSphericalHierarchy, point [3]*big.Rat, o RationalHierarchyTorusDistanceOptions, vendor bool) (RationalHierarchyTorusDistance, error) {
	bad := func(e error) (RationalHierarchyTorusDistance, error) { return RationalHierarchyTorusDistance{}, e }
	if ctx == nil {
		return bad(geom.ErrInvalid)
	}
	if e := ctx.Err(); e != nil {
		return bad(e)
	}
	opts, e := sphereDistanceRootOptions(geom.ExactCurvePlaneEventOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions})
	if e != nil {
		return bad(e)
	}
	a := hierarchyPointArithmetic{bits: opts.MaxCoefficientBits}
	for _, q := range point {
		if q == nil {
			return bad(geom.ErrInvalid)
		}
		a.check(q)
	}
	if a.err != nil {
		return bad(a.err)
	}
	point = copyHierarchyRats(point)
	actual, c, e := rationalSphericalHierarchyEnvelopeFor(ctx, m, vendor)
	if e != nil {
		return bad(e)
	}
	proof, e := readRationalSphericalHierarchy(ctx, bytes.NewBufferString(actual.Source), o.Source, actual.LocalProof.Proof.Tolerance, vendor)
	if e != nil {
		return bad(e)
	}
	expected, d, e := rationalSphericalHierarchyEnvelopeFor(ctx, proof, vendor)
	if e != nil {
		return bad(e)
	}
	if !reflect.DeepEqual(actual, expected) || !reflect.DeepEqual(c, d) {
		return bad(geom.ErrInvalid)
	}
	out := RationalHierarchyTorusDistance{LengthUnit: proof.LengthUnit}
	remaining := func() (geom.ExactCurvePlaneEventOptions, error) {
		q := opts
		q.MaxOperations -= out.Operations
		q.MaxSubdivisions -= out.Subdivisions
		if q.MaxOperations < 1 || q.MaxSubdivisions < 1 {
			return q, geom.ErrConvergence
		}
		return q, nil
	}
	rational := func(q *big.Rat) (*geom.ExactAlgebraicRoot, error) {
		opts, e := remaining()
		if e != nil {
			return nil, e
		}
		r, n, e := geom.NewExactRationalRoot(ctx, q, opts)
		out.Operations += n
		return r, e
	}
	extremeRoot := func(coeff []*big.Rat, upper *big.Rat, largest bool) (*geom.ExactAlgebraicRoot, error) {
		o, e := remaining()
		if e != nil {
			return nil, e
		}
		r, e := geom.IsolateExactPolynomialRoots(ctx, coeff, new(big.Rat), upper, o)
		out.Operations += r.Operations
		out.Subdivisions += r.Subdivisions
		if e != nil {
			return nil, e
		}
		if len(r.Roots) == 0 {
			return nil, geom.ErrConvergence
		}
		selected := r.Roots[0].Root
		for _, v := range r.Roots[1:] {
			o, e = remaining()
			if e != nil {
				return nil, e
			}
			cmp, e := v.Root.Compare(ctx, selected, o)
			out.Operations += cmp.Operations
			out.Subdivisions += cmp.Subdivisions
			if e != nil {
				return nil, e
			}
			if !cmp.Proven {
				return nil, geom.ErrConvergence
			}
			if (!largest && cmp.Order < 0) || (largest && cmp.Order > 0) {
				selected = v.Root
			}
		}
		return selected, nil
	}
	for _, u := range proof.Instances {
		if e = ctx.Err(); e != nil {
			return bad(e)
		}
		p := proof.Products[u.ProductID]
		leaf := HierarchyTorusDistanceLeaf{ProductID: u.ProductID, SourceProductID: p.SourceProductID, SourceBodyID: p.SourceBodyID, Path: append([]int64(nil), u.Path...), Location: AnalyticPointUnsupported}
		if p.Torus == nil {
			leaf.UnsupportedReason = "ring-torus distance only; other material family not evaluated"
			out.Leaves = append(out.Leaves, leaf)
			continue
		}
		out.Operations += 512
		if _, e = remaining(); e != nil {
			return bad(e)
		}
		torus := p.Torus
		leaf.SourceFaceID = torus.SourceFaceID
		R, r := new(big.Rat).SetFloat64(torus.MajorRadius), new(big.Rat).SetFloat64(torus.MinorRadius)
		R2, r2 := a.mul(R, R), a.mul(r, r)
		frameAxes := [3][3]*big.Rat{mappedRatVec(torus.Frame.XAxis()), mappedRatVec(torus.Frame.YAxis()), mappedRatVec(torus.Frame.ZAxis())}
		local := a.inverse(a.inverse(point, u.pose.Origin, [3][3]*big.Rat{u.pose.X, u.pose.Y, u.pose.Z}), mappedRatVec(torus.Frame.Origin()), frameAxes)
		x, y, z := local[0], local[1], local[2]
		s := a.add(a.mul(x, x), a.mul(y, y))
		A := a.add(a.add(s, a.mul(z, z)), R2)
		E := a.sub(A, r2)
		fourR2s := a.mul(big.NewRat(4, 1), a.mul(R2, s))
		F := a.sub(a.mul(E, E), fourR2s)
		set := HierarchyTorusNearestSet{kind: "point", local: copyHierarchyRats(local), major: new(big.Rat).Set(R), minor: new(big.Rat).Set(r)}
		matrix := u.Matrix()
		fo := mappedRatVec(torus.Frame.Origin())
		for i := 0; i < 3; i++ {
			set.origin[i] = new(big.Rat).Set(matrix[i][3])
			for j := 0; j < 3; j++ {
				set.origin[i] = a.add(set.origin[i], a.mul(matrix[i][j], fo[j]))
				set.rotation[i][j] = new(big.Rat)
				for k := 0; k < 3; k++ {
					set.rotation[i][j] = a.add(set.rotation[i][j], a.mul(matrix[i][k], frameAxes[j][k]))
				}
			}
			set.axis[i] = new(big.Rat).Set(set.rotation[i][2])
		}
		if a.err != nil {
			return bad(a.err)
		}
		leaf.Location = AnalyticPointOutside
		if F.Sign() < 0 {
			leaf.Location = AnalyticPointInside
		} else if F.Sign() == 0 {
			leaf.Location = AnalyticPointBoundary
		}
		if z.Sign() == 0 && s.Cmp(R2) == 0 {
			set.kind = "meridian-circle"
			set.exactCenter = copyHierarchyRats(point)
			azimuth := [3]*big.Rat{a.check(new(big.Rat).Quo(new(big.Rat).Neg(y), R)), a.check(new(big.Rat).Quo(x, R)), new(big.Rat)}
			for i := range set.axis {
				set.axis[i] = new(big.Rat)
				for j := 0; j < 3; j++ {
					set.axis[i] = a.add(set.axis[i], a.mul(set.rotation[i][j], azimuth[j]))
				}
			}
			leaf.boundary, e = rational(r2)
		} else if F.Sign() == 0 {
			set.exactPoint = copyHierarchyRats(point)
			leaf.boundary, e = rational(new(big.Rat))
		} else if s.Sign() == 0 {
			set.kind = "axis-circle"
			sum := a.add(A, r2)
			leaf.boundary, e = extremeRoot([]*big.Rat{a.mul(E, E), a.mul(big.NewRat(-2, 1), sum), big.NewRat(1, 1)}, sum, false)
			if e == nil {
				upper := a.add(big.NewRat(1, 1), a.check(new(big.Rat).Quo(r2, A)))
				set.tubeScale, e = extremeRoot([]*big.Rat{new(big.Rat).Neg(r2), new(big.Rat), A}, upper, false)
			}
		} else {
			B := a.add(A, r2)
			C := a.add(a.mul(E, E), fourR2s)
			D := a.mul(big.NewRat(4, 1), fourR2s)
			coeff := []*big.Rat{a.mul(F, F), a.add(a.mul(big.NewRat(-4, 1), a.mul(B, C)), a.mul(big.NewRat(2, 1), a.mul(D, E))), a.sub(a.add(a.mul(big.NewRat(4, 1), a.mul(B, B)), a.mul(big.NewRat(2, 1), C)), D), a.mul(big.NewRat(-4, 1), B), big.NewRat(1, 1)}
			U := a.add(A, a.mul(big.NewRat(2, 1), a.mul(R, a.add(s, big.NewRat(1, 1)))))
			upper := a.add(a.add(U, r2), a.mul(big.NewRat(2, 1), a.mul(r, a.add(U, big.NewRat(1, 1)))))
			if a.err != nil {
				return bad(a.err)
			}
			leaf.boundary, e = extremeRoot(coeff, upper, false)
			if e == nil {
				upper = a.add(big.NewRat(1, 1), a.check(new(big.Rat).Inv(s)))
				set.inverseRadial, e = extremeRoot([]*big.Rat{big.NewRat(-1, 1), new(big.Rat), s}, upper, false)
			}
			if e == nil {
				delta := a.sub(a.mul(A, A), fourR2s)
				if delta.Sign() <= 0 {
					return bad(geom.ErrInvalid)
				}
				upper = a.add(big.NewRat(1, 1), a.check(new(big.Rat).Quo(a.mul(r2, U), delta)))
				if a.err != nil {
					return bad(a.err)
				}
				set.tubeScale, e = extremeRoot([]*big.Rat{a.mul(r2, r2), new(big.Rat), a.mul(big.NewRat(-2, 1), a.mul(A, r2)), new(big.Rat), delta}, upper, true)
			}
		}
		if e != nil {
			return bad(e)
		}
		if a.err != nil {
			return bad(a.err)
		}
		leaf.NearestBoundary = set
		leaf.material = leaf.boundary
		if leaf.Location == AnalyticPointInside {
			leaf.material, e = rational(new(big.Rat))
			if e != nil {
				return bad(e)
			}
		}
		leaf.Supported = true
		out.Leaves = append(out.Leaves, leaf)
	}
	return out, nil
}
