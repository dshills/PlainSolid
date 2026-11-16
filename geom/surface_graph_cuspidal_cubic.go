package geom

import (
	"context"
	"math/big"
)

// CuspidalCubicSection describes the complete intersection of the two original
// projected graphs. The canonical parameter t maps to x=t²/Scale, y=t³/Scale.
// ChartMaps give each original owner's UV from x,y and a constant. The
// returned rationals are independent of the source patch storage.
type CuspidalCubicSection struct {
	ProjectionAxes        [2]int
	ProjectionOrientation int // sign from canonical x,y to the ordered model projection
	FreeAxis              int // longitudinal axis in source A's chart
	SourceDirection       int // toward the original longitudinal exit side
	Domains               [2]ParameterRectangle
	ChartMaps             [2][2][3]*big.Rat
	Scale                 *big.Rat
	Vertices              [3]CuspidalCubicVertex // cusp, negative exit, positive exit
	Germs                 [2]CuspidalCubicGerm   // negative t, positive t
	Regions               [2]CuspidalCubicRegion // exterior, between the germs
	ExactOperations       int
}

type CuspidalCubicVertex struct {
	UV       [2][2]*big.Rat // original owner chart coordinates
	Point    [3]*big.Rat
	Boundary [2]uint8 // per owner, bit 0/1/2/3 for UMin/UMax/VMin/VMax
}

// CuspidalCubicGerm describes a restriction in the signed t clock. Its jets
// use q=|t|, increasing from the cusp towards the associated exit. The
// first cusp jet is zero. The second cusp jets on the two sides are equal
// and their third jets are opposite. The terminal first jet is regular.
type CuspidalCubicGerm struct {
	Range                   [2]*big.Rat // increasing signed t
	UVSecond, UVThird       [2][2]*big.Rat
	ModelSecond, ModelThird [3]*big.Rat
	TerminalUV              [2][2]*big.Rat // terminal first UV derivative in q
	TerminalModel           [3]*big.Rat    // terminal first model derivative in q
}

type CuspidalCubicRegion struct {
	UV         [2][2]*big.Rat // rational interior witness on both original owners
	HeightSide int            // exact sign of A minus B on the omitted model axis
}

// DiscoverProjectedGraphCuspidalCubic recovers the entire triple-root
// identity from the original homogeneous height coefficients. Each owner has
// a single Bézier span, with invertible affine projection onto the shared
// physical footprint. Both germs must exit on the same longitudinal side
// at rational parameters strictly between the corners. The original charts
// can be independently signed, swapped and scaled. Failure returns nil.
func DiscoverProjectedGraphCuspidalCubic(ctx context.Context, a, c *SurfacePatch, axes [2]int, freeAxis int,
	o GraphPointContactOptions) (*CuspidalCubicSection, error) {
	projection, err := projectGraphCubicSource(ctx, a, c, axes, freeAxis, o)
	if err != nil {
		return nil, err
	}
	for _, source := range [2]*SurfacePatch{a, c} {
		if source.countU != source.degreeU+1 || source.countV != source.degreeV+1 {
			return nil, ErrConvergence
		}
	}
	b := projection.budget
	coefficients, err := recoverGraphCubicCoefficients(projection.powers, b)
	if err != nil {
		return nil, err
	}
	monic := coefficients.monic
	center := new(big.Rat).Mul(big.NewRat(-1, 3), monic[2])
	if err := b.check(center); err != nil {
		return nil, err
	}
	// A triple root is forced by the quadratic coefficient of a monic
	// cubic. Verify the exact identity, including the constant and linear
	// terms. The x³ factor has odd multiplicity, so no rational graph
	// factor can describe either half of the cusp.
	factor := rationalPolynomial{new(big.Rat).Neg(center), big.NewRat(1, 1)}
	square, err := polynomialProduct(factor, factor, b)
	if err != nil {
		return nil, err
	}
	cube, err := polynomialProduct(square, factor, b)
	if err != nil {
		return nil, err
	}
	for i, value := range monic {
		if value.Cmp(cube[i]) != 0 {
			return nil, ErrConvergence
		}
	}
	direction := coefficients.signedScale.Sign()
	scale := new(big.Rat).Abs(coefficients.signedScale)
	if err := b.check(scale); err != nil {
		return nil, err
	}
	positions := [2]*big.Rat{center, coefficients.dependentCenter}
	near, far, exit, err := cuspidalCubicLimits(positions, direction, scale, b)
	if err != nil {
		return nil, err
	}
	out := &CuspidalCubicSection{
		ProjectionAxes: axes, FreeAxis: freeAxis, SourceDirection: direction, Scale: scale,
		Domains: [2]ParameterRectangle{a.Domain(), c.Domain()},
	}
	out.ChartMaps, err = graphCubicChartMaps(out.Domains[0], freeAxis, direction, positions, projection.alignment, b)
	if err != nil {
		return nil, err
	}
	if err := bindCuspidalCubicOwners(ctx, [2]*SurfacePatch{a, c}, out, projection.residualAxis, coefficients.side, near, far, exit, b); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	out.ExactOperations = b.operations
	return out, nil
}

func cuspidalCubicLimits(center [2]*big.Rat, direction int, scale *big.Rat, b *polynomialBudget) (near, far, exit *big.Rat, err error) {
	one := big.NewRat(1, 1)
	near = new(big.Rat).Set(center[0])
	far = new(big.Rat).Sub(one, near)
	if direction < 0 {
		near, far = far, near
	}
	margin := new(big.Rat).Sub(one, center[1])
	if margin.Cmp(center[1]) > 0 {
		margin.Set(center[1])
	}
	if near.Sign() <= 0 || far.Sign() <= 0 || margin.Sign() <= 0 {
		return nil, nil, nil, ErrDomain
	}
	// For x>0 the inverse is t=y/x. Both |y| and x increase with
	// |t|, so strict bounds at the far side prove complete coverage and
	// exclude transverse side and corner exits. At x=0 only y=0 is possible.
	t2 := new(big.Rat).Mul(scale, far)
	height2 := new(big.Rat).Mul(t2, new(big.Rat).Mul(far, far))
	margin.Mul(margin, margin)
	for _, q := range []*big.Rat{height2, margin} {
		if err := b.check(q); err != nil {
			return nil, nil, nil, err
		}
	}
	if height2.Cmp(margin) >= 0 {
		return nil, nil, nil, ErrDomain
	}
	lo, hi, err := rationalSqrtBounds(t2, 8, b)
	if err != nil {
		return nil, nil, nil, err
	}
	if lo.Cmp(hi) != 0 {
		return nil, nil, nil, ErrUnrepresentable
	}
	return near, far, lo, nil
}
