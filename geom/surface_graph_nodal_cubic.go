package geom

import (
	"context"
	"math/big"
)

// NodalCubicSection proves the complete intersection of two original affine
// projected spline graphs. The common chart has canonical coordinates
// x,y defined by ChartMaps to each owner's original UV. The whole section
// is x=t²/Scale-LoopDepth, y=t*x. Arms and loop halves are distinct exact
// restrictions. The simple factor x+LoopDepth precludes a pair of rational
// graph branches in x. All returned rationals are detached from the sources.
type NodalCubicSection struct {
	ProjectionAxes        [2]int
	ProjectionOrientation int // sign from canonical x,y to the ordered model projection
	FreeAxis              int // longitudinal axis in source A's chart
	SourceDirection       int // direction toward the two exits in source A's free axis
	Domains               [2]ParameterRectangle
	ChartMaps             [2][2][3]*big.Rat // original UV = first*x + second*y + offset
	Scale, LoopDepth      *big.Rat
	Vertices              [4]NodalCubicVertex // node, turning point, negative exit, positive exit
	Edges                 [4]NodalCubicEdge   // negative arm, two loop halves, positive arm
	Rays                  [4]NodalCubicRay    // counterclockwise from +x in canonical x,y
	Sectors               [4]int              // region between each ray and the next cyclic ray
	Regions               [3]NodalCubicRegion // exterior, loop interior, between arms
	ExactOperations       int
}

type NodalCubicVertex struct {
	UV       [2][2]*big.Rat // original chart coordinates by owner
	Point    [3]*big.Rat
	Boundary [2]uint8 // per owner, bit 0/1/2/3 for UMin/UMax/VMin/VMax
}

type NodalCubicEdge struct {
	Start, End int
	Range      [2]*big.Rat // increasing canonical t
}

type NodalCubicRay struct {
	Edge     int
	Reversed bool           // edge is followed in decreasing t away from the node
	UV       [2][2]*big.Rat // exact outgoing vector in each original owner chart
	Model    [3]*big.Rat    // corresponding world vector
}

type NodalCubicRegion struct {
	UV         [2][2]*big.Rat // exact interior witness on both owners
	HeightSide int            // sign of A-B along the omitted projection axis
}

// DiscoverProjectedGraphNodalCubic recovers a complete real nodal cubic with
// one interior loop and two exits on the same original boundary. It checks
// every homogeneous height coefficient on a common single Bézier span and
// matches the full identity y²=Scale*x²*(x+LoopDepth). The independently
// authored charts may be swapped, signed and scaled over the same footprint.
// Exact rational node and exit parameters are required. A clipped loop or a
// terminal on a transverse side is outside this section family.
func DiscoverProjectedGraphNodalCubic(ctx context.Context, a, c *SurfacePatch, axes [2]int, freeAxis int,
	o GraphPointContactOptions) (*NodalCubicSection, error) {
	if ctx == nil || a == nil || c == nil || freeAxis < 0 || freeAxis > 1 {
		return nil, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	o, err := alignedPolynomialProofOptions(o)
	if err != nil {
		return nil, err
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	order, err := modelProjectionPermutation(axes)
	if err != nil {
		return nil, err
	}
	pa, err := permuteGraphPatch(a, order, b)
	if err != nil {
		return nil, err
	}
	pc, err := permuteGraphPatch(c, order, b)
	if err != nil {
		return nil, err
	}
	remaining := b.maxOperations - b.operations
	if remaining < 1 {
		return nil, ErrConvergence
	}
	alignment, work, err := AlignAffineXYChartsWithWork(ctx, pa, pc, SurfacePatchOptions{MaxOperations: remaining, MaxCoefficientBits: b.maxBits})
	b.operations += work
	if err != nil {
		return nil, err
	}
	residual, err := orderingDifference(ctx, pa, alignment.TargetOnSource(), o.MaxCells, b)
	if err != nil {
		return nil, err
	}
	if residual.countU != residual.degreeU+1 || residual.countV != residual.degreeV+1 ||
		residual.degreeU > 6 || residual.degreeV > 6 {
		return nil, ErrConvergence
	}
	powers, err := graphBivariateCoefficients(residual, freeAxis, b)
	if err != nil {
		return nil, err
	}
	law, err := recoverNodalCubic(powers, b)
	if err != nil {
		return nil, err
	}
	return bindNodalCubicSection(ctx, [2]*SurfacePatch{a, c}, axes, order[2], freeAxis, law, alignment.mapping, b)
}

type nodalCubicLaw struct {
	center           [2]*big.Rat // normalized source free, dependent
	scale, loopDepth *big.Rat
	direction, side  int
}

func recoverNodalCubic(p [3]rationalPolynomial, b *polynomialBudget) (r nodalCubicLaw, err error) {
	if len(p[0]) != 4 || len(p[1]) > 1 || len(p[2]) != 1 {
		return r, ErrConvergence
	}
	// The dependent coefficients prove a constant quadratic of the form
	// lambda*(v-s)². Remove its constant term before solving the free cubic.
	lambda, lead := p[2][0], p[0][3]
	s := new(big.Rat)
	if len(p[1]) == 1 {
		s.Quo(p[1][0], new(big.Rat).Mul(big.NewRat(-2, 1), lambda))
	}
	constant := new(big.Rat).Sub(p[0][0], new(big.Rat).Mul(lambda, new(big.Rat).Mul(s, s)))
	constant.Quo(constant, lead)
	linear := new(big.Rat).Quo(p[0][1], lead)
	quadratic := new(big.Rat).Quo(p[0][2], lead)
	monic := rationalPolynomial{constant, linear, quadratic, big.NewRat(1, 1)}
	for _, q := range append(monic, s) {
		if err = b.check(q); err != nil {
			return r, err
		}
	}
	// For a monic cubic with one double root, the linear subresultant of
	// P and P' gives r=(9d-ac)/(2(a²-3c)). A zero denominator includes a
	// triple root and cannot describe a transverse node.
	den := new(big.Rat).Sub(new(big.Rat).Mul(quadratic, quadratic), new(big.Rat).Mul(big.NewRat(3, 1), linear))
	if err = b.check(den); err != nil {
		return r, err
	}
	if den.Sign() == 0 {
		return r, ErrDomain
	}
	x := new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(9, 1), constant), new(big.Rat).Mul(quadratic, linear))
	x.Quo(x, den.Mul(den, big.NewRat(2, 1)))
	if err = b.check(x); err != nil {
		return r, err
	}
	factor := rationalPolynomial{new(big.Rat).Neg(x), big.NewRat(1, 1)}
	factor2, err := polynomialProduct(factor, factor, b)
	if err != nil {
		return r, err
	}
	q, remainder, err := polynomialDivide(monic, factor2, b)
	if err != nil {
		return r, err
	}
	if len(remainder) != 0 || len(q) != 2 {
		return r, ErrConvergence
	}
	signedScale := new(big.Rat).Quo(new(big.Rat).Neg(lead), lambda)
	direction := signedScale.Sign()
	depth := new(big.Rat).Mul(new(big.Rat).Add(q[0], x), big.NewRat(int64(direction), 1))
	if depth.Sign() <= 0 {
		return r, ErrDomain
	}
	scale := new(big.Rat).Abs(signedScale)
	for _, value := range []*big.Rat{scale, depth} {
		if err = b.check(value); err != nil {
			return r, err
		}
	}
	return nodalCubicLaw{center: [2]*big.Rat{x, s}, scale: scale, loopDepth: depth, direction: direction, side: lambda.Sign()}, nil
}

// The parametrization is injective except at the two preimages of the node.
// For x!=0 the inverse is t=y/x. The far side caps |t| at tau. Between
// the preimages ±alpha, x<0 gives one loop whose maximum |y|² is
// 4*k*a³/27. At t=0 the turning point is regular because dy/dt=-a.
// Beyond either preimage, x and |y| increase with |t|.
func nodalCubicLimits(law nodalCubicLaw, b *polynomialBudget) (near, far, alpha, tau *big.Rat, err error) {
	one := big.NewRat(1, 1)
	near = new(big.Rat).Set(law.center[0])
	far = new(big.Rat).Sub(one, near)
	if law.direction < 0 {
		near, far = far, near
	}
	y := law.center[1]
	margin := new(big.Rat).Sub(one, y)
	if margin.Cmp(y) > 0 {
		margin.Set(y)
	}
	if near.Cmp(law.loopDepth) <= 0 || far.Sign() <= 0 || margin.Sign() <= 0 {
		return nil, nil, nil, nil, ErrDomain
	}
	margin.Mul(margin, margin)
	a, k := law.loopDepth, law.scale
	nodeSquared := new(big.Rat).Mul(k, a)
	exitSquared := new(big.Rat).Mul(k, new(big.Rat).Add(far, a))
	loopHeightSquared := new(big.Rat).Mul(nodeSquared, new(big.Rat).Mul(a, a))
	loopHeightSquared.Mul(loopHeightSquared, big.NewRat(4, 27))
	exitHeightSquared := new(big.Rat).Mul(exitSquared, new(big.Rat).Mul(far, far))
	for _, q := range []*big.Rat{margin, loopHeightSquared, exitHeightSquared} {
		if err := b.check(q); err != nil {
			return nil, nil, nil, nil, err
		}
	}
	if loopHeightSquared.Cmp(margin) >= 0 || exitHeightSquared.Cmp(margin) >= 0 {
		return nil, nil, nil, nil, ErrDomain
	}
	var roots [2]*big.Rat
	for i, q := range []*big.Rat{nodeSquared, exitSquared} {
		lo, hi, err := rationalSqrtBounds(q, 8, b)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if lo.Cmp(hi) != 0 {
			return nil, nil, nil, nil, ErrUnrepresentable
		}
		roots[i] = lo
	}
	return near, far, roots[0], roots[1], nil
}
