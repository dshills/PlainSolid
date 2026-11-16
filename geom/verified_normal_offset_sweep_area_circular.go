package geom

import (
	"context"
	"math/big"
)

type sweepCircularAreaProfile struct {
	rulingAxis       int
	normalSign       int
	radiusSquare     *big.Rat
	widthSquare      *big.Rat
	angleRatioSquare *big.Rat
}

func (s *VerifiedNormalOffsetSweep) circularSweepBoundaryArea(ctx context.Context, o NormalOffsetSweepAreaOptions,
	b *polynomialBudget, result *NormalOffsetSweepBoundaryArea) error {
	profile, err := s.certifySweepAreaCircularProfile(ctx, b)
	if err != nil {
		return err
	}
	goal := o.AbsoluteError
	if goal.Sign() == 0 {
		goal = big.NewRat(1, 100_000_000)
	}
	// Every area uses at most two length factors and one angle.
	// Extra precision tracks the upper magnitude of those lengths.
	scaleBits := 0
	for _, x := range [4]*big.Rat{profile.radiusSquare, profile.widthSquare, s.lower, s.upper} {
		scaleBits = max(scaleBits, x.Num().BitLen()-x.Denom().BitLen()+1)
	}
	precision := min(192, max(0, goal.Denom().BitLen()-goal.Num().BitLen())+2*scaleBits+10)
	sqrt := func(q *big.Rat) (exactInterval, error) {
		bits := min(precision, (b.maxBits-q.Num().BitLen())/2)
		lo, hi, err := rationalSqrtBounds(q, bits, b)
		return exactInterval{lo, hi}, err
	}
	radius, err := sqrt(profile.radiusSquare)
	if err != nil {
		return err
	}
	width, err := sqrt(profile.widthSquare)
	if err != nil {
		return err
	}
	ratio, err := sqrt(profile.angleRatioSquare)
	if err != nil {
		return err
	}
	half := big.NewRat(1, 2)
	if ratio.hi.Cmp(half) > 0 {
		ratio.hi = half
	}
	atanTol := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), uint(precision)))
	if err := b.check(atanTol); err != nil {
		return err
	}
	lo, err := helicalAtanSmall(ctx, ratio.lo, atanTol, 256, b)
	if err != nil {
		return err
	}
	hi, err := helicalAtanSmall(ctx, ratio.hi, atanTol, 256, b)
	if err != nil {
		return err
	}
	angle := exactInterval{lo[0], hi[1]}
	a := offsetIntervalArithmetic{b: b}
	thickness := a.sub(sweepAreaSingleton(s.upper), sweepAreaSingleton(s.lower))
	bottom := a.add(radius, sweepAreaSingleton(new(big.Rat).Mul(s.lower, big.NewRat(int64(profile.normalSign), 1))))
	top := a.add(radius, sweepAreaSingleton(new(big.Rat).Mul(s.upper, big.NewRat(int64(profile.normalSign), 1))))
	if a.err != nil {
		return a.err
	}
	// Both shifted radii must be positive for the annular-sector and
	// cylindrical area formulas over the entire original displacement.
	if bottom.hi.Sign() <= 0 || top.hi.Sign() <= 0 {
		return ErrDomain
	}
	if bottom.lo.Sign() <= 0 || top.lo.Sign() <= 0 {
		return ErrConvergence
	}
	endArea := a.mul(a.mul(a.add(bottom, top), sweepAreaSingleton(half)), a.mul(thickness, angle))
	longArea := a.mul(width, thickness)
	capBottom := a.mul(a.mul(width, bottom), angle)
	capTop := a.mul(a.mul(width, top), angle)
	longSquare := a.mul(sweepAreaSingleton(profile.widthSquare), a.square(thickness))
	if a.err != nil {
		return a.err
	}
	bounds := [6]exactInterval{endArea, endArea, longArea, longArea, capBottom, capTop}
	var areaSquares [6]*big.Rat
	if profile.rulingAxis == 1 {
		bounds[0], bounds[1], bounds[2], bounds[3] = longArea, longArea, endArea, endArea
	}
	side := (1 - profile.rulingAxis) * 2
	areaSquares[side], areaSquares[side+1] = longSquare.lo, longSquare.lo

	total := offsetZeroInterval()
	for side, interval := range bounds {
		if err := ctx.Err(); err != nil {
			return err
		}
		if result.VisitedCells >= o.MaxCells {
			return ErrConvergence
		}
		result.VisitedCells++
		face := NormalOffsetSweepFaceArea{Side: NormalOffsetSweepSide(side), Bounds: copySweepInterval(interval)}
		if square := areaSquares[side]; square != nil {
			face.SquaredArea = new(big.Rat).Set(square)
		}
		result.Faces = append(result.Faces, face)
		total = a.add(total, interval)
		if a.err != nil {
			return a.err
		}
	}
	result.Bounds = copySweepInterval(total)
	if err := ctx.Err(); err != nil {
		return err
	}
	widthError := a.sub(sweepAreaSingleton(total.hi), sweepAreaSingleton(total.lo))
	if a.err != nil {
		return a.err
	}
	if widthError.hi.Cmp(o.AbsoluteError) > 0 {
		return ErrConvergence
	}
	result.Complete = true
	return nil
}

func sweepAreaSingleton(q *big.Rat) exactInterval {
	return exactInterval{q, q}
}

func (s *VerifiedNormalOffsetSweep) certifySweepAreaCircularProfile(ctx context.Context, b *polynomialBudget) (
	out sweepCircularAreaProfile, err error) {
	p := s.source
	w := sweepLineWork{sweep: s, budget: b}
	a := boundedRatArithmetic{b: b}
	for axis, degree := range [2]int{p.degreeV, p.degreeU} {
		if degree != 2 {
			continue
		}
		first, last := p.net[0], p.net[(p.countU-1)*p.countV]
		edge := SurfaceEdgeUMin
		if axis == 1 {
			last, edge = p.net[p.countV-1], SurfaceEdgeVMin
		}
		if first[3].Cmp(last[3]) != 0 {
			continue
		}
		for i := range w.direction {
			w.direction[i] = a.sub(last[i], first[i])
		}
		if a.err != nil {
			return out, a.err
		}
		ruling, proved, err := w.perpendicularRuling(ctx, axis)
		if err != nil {
			return out, err
		}
		if !proved {
			continue
		}
		profile, err := circularSweepAreaGeometry(ctx, p, edge, ruling, b)
		if err != nil {
			return out, err
		}
		profile.rulingAxis = axis
		return profile, nil
	}
	return out, ErrDomain
}

func circularSweepAreaGeometry(ctx context.Context, source *SurfacePatch, edge SurfaceEdge, ruling [3]*big.Rat,
	b *polynomialBudget) (out sweepCircularAreaProfile, err error) {
	jet, err := offsetEdgePowers(ctx, source, edge, false, nil, b)
	if err != nil {
		return out, err
	}
	h := jet[0]
	a := boundedRatArithmetic{b: b}
	var points [3][3]*big.Rat
	for j, parameter := range [3]*big.Rat{new(big.Rat), big.NewRat(1, 2), big.NewRat(1, 1)} {
		weight, err := polynomialValue(h[3], parameter, b)
		if err != nil {
			return out, err
		}
		for i := range points[j] {
			value, err := polynomialValue(h[i], parameter, b)
			if err != nil {
				return out, err
			}
			points[j][i] = a.quo(value, weight)
		}
	}
	var toMid, toEnd [3]*big.Rat
	for i, x := range points[0] {
		toMid[i], toEnd[i] = a.sub(points[1][i], x), a.sub(points[2][i], x)
	}
	if a.err != nil {
		return out, a.err
	}
	inverse, invertible, err := inverseExact3([3][3]*big.Rat{toMid, toEnd, ruling}, b)
	if err != nil {
		return out, err
	}
	if !invertible {
		return out, ErrDomain
	}
	rhs := [3]*big.Rat{
		a.quo(sweepLineDot(&a, toMid, toMid), big.NewRat(2, 1)),
		a.quo(sweepLineDot(&a, toEnd, toEnd), big.NewRat(2, 1)),
		new(big.Rat),
	}
	var center, start, finish [3]*big.Rat
	for i, row := range inverse {
		relative := sweepLineDot(&a, row, rhs)
		center[i] = a.add(points[0][i], relative)
		start[i], finish[i] = a.sub(new(big.Rat), relative), a.sub(toEnd[i], relative)
	}
	out.radiusSquare = sweepLineDot(&a, start, start)
	out.widthSquare = sweepLineDot(&a, ruling, ruling)
	dot := sweepLineDot(&a, start, finish)
	if a.err != nil {
		return out, a.err
	}
	if dot.Sign() <= 0 {
		return out, ErrDomain
	}

	// Prove the entire homogeneous profile lies on the circle and in
	// the open half-plane facing its first radius. Sweep injectivity
	// then identifies exactly the short arc connecting the endpoints.
	var radial [3]rationalPolynomial
	for i := range radial {
		cw, err := polynomialProduct(h[3], rationalPolynomial{center[i]}, b)
		if err != nil {
			return out, err
		}
		radial[i], err = graphPolynomialSum(h[i], cw, -1, b)
		if err != nil {
			return out, err
		}
	}
	r2, err := sweepLinePolynomialDot(radial, radial, b)
	if err != nil {
		return out, err
	}
	w2, err := polynomialProduct(h[3], h[3], b)
	if err != nil {
		return out, err
	}
	want, err := polynomialProduct(w2, rationalPolynomial{out.radiusSquare}, b)
	if err != nil {
		return out, err
	}
	residual, err := graphPolynomialSum(r2, want, -1, b)
	if err != nil {
		return out, err
	}
	if len(residual) != 0 {
		return out, ErrDomain
	}
	radialDot, err := sweepLinePolynomialDot(radial,
		[3]rationalPolynomial{{start[0]}, {start[1]}, {start[2]}}, b)
	if err != nil {
		return out, err
	}
	halfPlane, err := polynomialBernsteinInterval(radialDot, exactInterval{new(big.Rat), big.NewRat(1, 1)}, b)
	if err != nil {
		return out, err
	}
	if halfPlane.lo.Sign() <= 0 {
		return out, ErrDomain
	}

	// With rational endpoints on the same circle the squared tangent
	// of the angle is (r^4 - (a·b)^2)/(a·b)^2.
	dotSquare := a.mul(dot, dot)
	out.angleRatioSquare = a.quo(a.sub(a.mul(out.radiusSquare, out.radiusSquare), dotSquare), dotSquare)
	if a.err != nil {
		return out, a.err
	}
	if out.angleRatioSquare.Sign() <= 0 || out.angleRatioSquare.Cmp(big.NewRat(1, 4)) > 0 {
		return out, ErrDomain
	}
	normal, err := offsetNormalPowers(ctx, jet, b)
	if err != nil {
		return out, err
	}
	var firstNormal [3]*big.Rat
	for i := range firstNormal {
		firstNormal[i], err = polynomialValue(normal[i], new(big.Rat), b)
		if err != nil {
			return out, err
		}
	}
	sign := sweepLineDot(&a, firstNormal, start)
	if a.err != nil {
		return out, a.err
	}
	out.normalSign = sign.Sign()
	if out.normalSign == 0 {
		return out, ErrDegenerate
	}
	return out, nil
}
