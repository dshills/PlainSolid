package geom

import (
	"context"
	"math/big"
	"slices"
)

// The monic residual has h[2n]=-D-p and h[2n-1][x²]=n+D*p₂,
// where D is twice the sum of the dependent center ordinates. Recover p
// and authenticate its entire synthetic-division remainder.
func divideGraphParabolaCircleProduct(law []rationalPolynomial, n int, b *polynomialBudget) (rationalPolynomial, []rationalPolynomial, int, error) {
	d := len(law) - 1
	if d != 2*n+1 || len(law[d]) != 1 || len(law[d-1]) != 3 || len(law[d-2]) > 3 {
		return nil, nil, 0, ErrConvergence
	}
	scale := law[d][0]
	sign := scale.Sign()
	for _, column := range law[:d] {
		for _, q := range column {
			q.Quo(q, scale)
			if err := b.check(q); err != nil {
				return nil, nil, 0, err
			}
		}
	}
	coefficient := new(big.Rat)
	if len(law[d-2]) == 3 {
		coefficient.Set(law[d-2][2])
	}
	sum := new(big.Rat).Sub(big.NewRat(int64(n), 1), coefficient)
	sum.Quo(sum, law[d-1][2])
	if err := b.check(sum); err != nil {
		return nil, nil, 0, err
	}
	p := rationalPolynomial{new(big.Rat).Neg(new(big.Rat).Add(law[d-1][0], sum)),
		new(big.Rat).Neg(law[d-1][1]), new(big.Rat).Neg(law[d-1][2])}
	quotient := make([]rationalPolynomial, d)
	quotient[d-1] = rationalPolynomial{big.NewRat(1, 1)}
	for i := d - 1; i > 0; i-- {
		product, err := polynomialProduct(p, quotient[i], b)
		if err != nil {
			return nil, nil, 0, err
		}
		quotient[i-1], err = graphPolynomialSum(law[i], product, 1, b)
		if err != nil {
			return nil, nil, 0, err
		}
	}
	product, err := polynomialProduct(p, quotient[0], b)
	if err != nil {
		return nil, nil, 0, err
	}
	remainder, err := graphPolynomialSum(law[0], product, 1, b)
	if err != nil {
		return nil, nil, 0, err
	}
	if len(remainder) != 0 {
		return nil, nil, 0, ErrConvergence
	}
	return p, quotient, sign, nil
}

// factorGraphCircleProduct recovers three distinct rational circle centers
// without proposed factors. Under z=x+iy, w=x-iy each monic circle is
// (z-u)(w-conj(u))-r². The highest w coefficient is the center polynomial
// A(z)=product(z-u). In the next coefficient B, evaluation at u leaves just
// -r² A'(u). The recovered circles are multiplied in the original chart
// and compared coefficient by coefficient with the entire quotient.
func factorGraphCircleProduct(ctx context.Context, quotient []rationalPolynomial, count, maxCells int,
	visits *int, budget *polynomialBudget) ([]GraphContactCircle, error) {
	if count < 1 || count > 3 || len(quotient) != 2*count+1 {
		return nil, ErrInvalid
	}
	leading, next, err := graphCircleIsotropicColumns(quotient, count, budget)
	if err != nil {
		return nil, err
	}
	centers, err := graphCircleComplexCenters(ctx, leading, maxCells, visits, budget)
	if err != nil {
		return nil, err
	}
	derivative := make([]graphCircleComplex, count)
	for i := range derivative {
		derivative[i] = graphCircleComplexZero()
		for dim := range 2 {
			derivative[i][dim].Mul(leading[i+1][dim], big.NewRat(int64(i+1), 1))
		}
	}
	circles := make([]GraphContactCircle, 0, count)
	for _, center := range centers {
		a, err := graphCircleComplexValue(derivative, center, budget)
		if err != nil {
			return nil, err
		}
		c, err := graphCircleComplexValue(next, center, budget)
		if err != nil {
			return nil, err
		}
		dim := 0
		if a[0].Sign() == 0 {
			dim = 1
		}
		if a[dim].Sign() == 0 {
			return nil, ErrDegenerate
		}
		radiusSquared := new(big.Rat).Quo(new(big.Rat).Neg(c[dim]), a[dim])
		if err := budget.check(radiusSquared); err != nil {
			return nil, err
		}
		residual := new(big.Rat).Add(c[1-dim], new(big.Rat).Mul(a[1-dim], radiusSquared))
		if err := budget.check(residual); err != nil {
			return nil, err
		}
		if radiusSquared.Sign() <= 0 || residual.Sign() != 0 {
			return nil, ErrConvergence
		}
		circles = append(circles, GraphContactCircle{Center: [2]*big.Rat(center), RadiusSquared: radiusSquared})
	}
	if err := bindGraphCircleProduct(quotient, circles, budget); err != nil {
		return nil, err
	}
	return circles, nil
}

func bindGraphCircleProduct(quotient []rationalPolynomial, circles []GraphContactCircle, b *polynomialBudget) error {
	one := big.NewRat(1, 1)
	product := []rationalPolynomial{{one}}
	for _, circle := range circles {
		x, y := circle.Center[0], circle.Center[1]
		constant := new(big.Rat).Add(new(big.Rat).Mul(x, x), new(big.Rat).Mul(y, y))
		factor := [3]rationalPolynomial{
			{constant.Sub(constant, circle.RadiusSquared), new(big.Rat).Mul(big.NewRat(-2, 1), x), one},
			{new(big.Rat).Mul(big.NewRat(-2, 1), y)},
			{one},
		}
		next := make([]rationalPolynomial, len(product)+2)
		for i, a := range product {
			for j, c := range factor {
				p, err := polynomialProduct(a, c, b)
				if err != nil {
					return err
				}
				next[i+j], err = graphPolynomialSum(next[i+j], p, 1, b)
				if err != nil {
					return err
				}
			}
		}
		product = next
	}
	for i, column := range product {
		if !slices.EqualFunc(column, quotient[i], func(a, c *big.Rat) bool { return a.Cmp(c) == 0 }) {
			return ErrConvergence
		}
	}
	return nil
}

type graphCircleComplex [2]*big.Rat

func graphCircleComplexZero() graphCircleComplex {
	return graphCircleComplex{new(big.Rat), new(big.Rat)}
}

func (z graphCircleComplex) addProduct(a, c graphCircleComplex, scale *big.Rat, b *polynomialBudget) error {
	realPart := new(big.Rat).Sub(new(big.Rat).Mul(a[0], c[0]), new(big.Rat).Mul(a[1], c[1]))
	imaginaryPart := new(big.Rat).Add(new(big.Rat).Mul(a[0], c[1]), new(big.Rat).Mul(a[1], c[0]))
	for dim, value := range [2]*big.Rat{realPart, imaginaryPart} {
		value.Mul(value, scale)
		if err := b.check(value); err != nil {
			return err
		}
		z[dim].Add(z[dim], value)
		if err := b.check(z[dim]); err != nil {
			return err
		}
	}
	return nil
}

func graphCircleIsotropicColumns(q []rationalPolynomial, count int, b *polynomialBudget) (
	leading, next []graphCircleComplex, err error) {
	var columns [2][]graphCircleComplex
	for column := range columns {
		wPower := count - column
		out := make([]graphCircleComplex, 2*count-wPower+1)
		for i := range out {
			out[i] = graphCircleComplexZero()
		}
		for j, coefficients := range q {
			for i, value := range coefficients {
				if value.Sign() == 0 {
					continue
				}
				if i+j > 2*count {
					return nil, nil, ErrConvergence
				}
				if i+j < wPower {
					continue
				}
				// x=(z+w)/2, y=i(w-z)/2. Collect the two indicated w
				// powers without expanding the other complex columns.
				for xw := max(0, wPower-j); xw <= min(i, wPower); xw++ {
					yw := wPower - xw
					factor := new(big.Int).Mul(new(big.Int).Binomial(int64(i), int64(xw)), new(big.Int).Binomial(int64(j), int64(yw)))
					if (j-yw+j/2)%2 != 0 {
						factor.Neg(factor)
					}
					term := new(big.Rat).SetFrac(factor, new(big.Int).Lsh(big.NewInt(1), uint(i+j)))
					term.Mul(term, value)
					if err := b.check(term); err != nil {
						return nil, nil, err
					}
					target := out[i+j-wPower][j%2]
					target.Add(target, term)
					if err := b.check(target); err != nil {
						return nil, nil, err
					}
				}
			}
		}
		columns[column] = out
	}
	leading, next = columns[0], columns[1]
	if leading[count][0].Cmp(big.NewRat(1, 1)) != 0 || leading[count][1].Sign() != 0 ||
		next[count+1][0].Sign() != 0 || next[count+1][1].Sign() != 0 {
		return nil, nil, ErrConvergence
	}
	return leading, next[:count+1], nil
}

func graphCircleComplexCenters(ctx context.Context, a []graphCircleComplex, maxCells int,
	visits *int, b *polynomialBudget) ([]graphCircleComplex, error) {
	resultant, err := graphCircleRealCenterCandidates(a, b)
	if err != nil {
		return nil, err
	}
	free, err := graphCircleUnitRationalRoots(ctx, resultant, maxCells, visits, b)
	if err != nil {
		return nil, err
	}
	centers := make([]graphCircleComplex, 0, len(a)-1)
	for _, x := range free {
		realPart, imaginaryPart, err := graphCircleCenterLine(a, x, b)
		if err != nil {
			return nil, err
		}
		gcd, err := polynomialGCD(realPart, imaginaryPart, b)
		if err != nil {
			return nil, err
		}
		if len(gcd) < 2 {
			continue
		}
		dependent, err := graphCircleUnitRationalRoots(ctx, gcd, maxCells, visits, b)
		if err != nil {
			return nil, err
		}
		for _, y := range dependent {
			centers = append(centers, graphCircleComplex{new(big.Rat).Set(x), y})
		}
	}
	if len(centers) != len(a)-1 {
		return nil, ErrConvergence
	}
	slices.SortFunc(centers, func(a, c graphCircleComplex) int {
		if order := a[0].Cmp(c[0]); order != 0 {
			return order
		}
		return a[1].Cmp(c[1])
	})
	return centers, nil
}

// Recover the real candidate factor product over all (u_i+conj(u_j))/2
// using Newton sums. Its diagonal contains every free center coordinate.
// Off-diagonal real candidates (including repeated free coordinates) are
// filtered by gcd(Re A(x+iy), Im A(x+iy)) on the exact original line.
func graphCircleRealCenterCandidates(a []graphCircleComplex, b *polynomialBudget) (rationalPolynomial, error) {
	n := len(a) - 1
	degree := n * n
	powers := make([]graphCircleComplex, degree+1)
	powers[0] = graphCircleComplex{big.NewRat(int64(n), 1), new(big.Rat)}
	minusOne := big.NewRat(-1, 1)
	one := big.NewRat(1, 1)
	for k := 1; k <= degree; k++ {
		v := graphCircleComplexZero()
		for j := 1; j <= min(k-1, n); j++ {
			if err := v.addProduct(a[n-j], powers[k-j], minusOne, b); err != nil {
				return nil, err
			}
		}
		if k <= n {
			for dim := range v {
				v[dim].Sub(v[dim], new(big.Rat).Mul(big.NewRat(int64(k), 1), a[n-k][dim]))
				if err := b.check(v[dim]); err != nil {
					return nil, err
				}
			}
		}
		powers[k] = v
	}
	candidates := make(rationalPolynomial, degree+1)
	candidates[degree] = one
	moments := make(rationalPolynomial, degree+1)
	for k := 1; k <= degree; k++ {
		v := graphCircleComplexZero()
		divisor := new(big.Int).Lsh(big.NewInt(1), uint(k))
		for j := 0; j <= k; j++ {
			conjugate := graphCircleComplex{powers[k-j][0], new(big.Rat).Neg(powers[k-j][1])}
			factor := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(k), int64(j)), divisor)
			if err := v.addProduct(powers[j], conjugate, factor, b); err != nil {
				return nil, err
			}
		}
		if v[1].Sign() != 0 {
			return nil, ErrConvergence
		}
		moments[k] = v[0]
		coefficient := new(big.Rat)
		for j := 1; j <= k; j++ {
			coefficient.Sub(coefficient, new(big.Rat).Mul(candidates[degree-k+j], moments[j]))
			if err := b.check(coefficient); err != nil {
				return nil, err
			}
		}
		coefficient.Quo(coefficient, big.NewRat(int64(k), 1))
		if err := b.check(coefficient); err != nil {
			return nil, err
		}
		candidates[degree-k] = coefficient
	}
	return candidates, nil
}

func graphCircleCenterLine(a []graphCircleComplex, x *big.Rat, b *polynomialBudget) (rationalPolynomial, rationalPolynomial, error) {
	var p [2]rationalPolynomial
	for dim := range p {
		p[dim] = make(rationalPolynomial, len(a))
		for j := range p[dim] {
			p[dim][j] = new(big.Rat)
		}
	}
	powers := make(rationalPolynomial, len(a))
	powers[0] = big.NewRat(1, 1)
	for k := 1; k < len(a); k++ {
		powers[k] = new(big.Rat).Mul(powers[k-1], x)
		if err := b.check(powers[k]); err != nil {
			return nil, nil, err
		}
	}
	for k, coefficient := range a {
		for j := 0; j <= k; j++ {
			factor := new(big.Rat).Mul(powers[k-j], new(big.Rat).SetInt(new(big.Int).Binomial(int64(k), int64(j))))
			for dim := range coefficient {
				// Multiplication by i^j rotates both coefficient parts.
				phase := (dim + j) % 4
				value := new(big.Rat).Mul(coefficient[dim], factor)
				if phase >= 2 {
					value.Neg(value)
				}
				target := p[phase%2][j]
				target.Add(target, value)
				if err := b.check(target); err != nil {
					return nil, nil, err
				}
			}
		}
	}
	return trimPolynomial(p[0]), trimPolynomial(p[1]), nil
}

func graphCircleComplexValue(p []graphCircleComplex, z graphCircleComplex, b *polynomialBudget) (graphCircleComplex, error) {
	v := graphCircleComplexZero()
	one := big.NewRat(1, 1)
	for i := len(p) - 1; i >= 0; i-- {
		next := graphCircleComplex{new(big.Rat).Set(p[i][0]), new(big.Rat).Set(p[i][1])}
		if err := next.addProduct(v, z, one, b); err != nil {
			return graphCircleComplex{}, err
		}
		v = next
	}
	return v, nil
}

func graphCircleUnitRationalRoots(ctx context.Context, p rationalPolynomial, maxCells int, visits *int,
	b *polynomialBudget) ([]*big.Rat, error) {
	if *visits >= maxCells {
		return nil, ErrConvergence
	}
	var roots []*big.Rat
	var width *big.Rat
	zero, one := new(big.Rat), big.NewRat(1, 1)
	splits, err := isolatePolynomialRoots(p, zero, one, b, (maxCells-*visits-1)/2, func(lo, hi *big.Rat, _ int) (bool, error) {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		x, err := recoverRationalPolynomialRoot(p, lo, hi, b)
		if err != nil {
			return false, err
		}
		if x != nil {
			if x.Sign() <= 0 || x.Cmp(one) >= 0 {
				return false, ErrDomain
			}
			roots = append(roots, x)
			return true, nil
		}
		if width == nil {
			width, err = graphRationalRootWidth(p, b)
			if err != nil {
				return false, err
			}
		}
		gap := new(big.Rat).Sub(hi, lo)
		if err := b.check(gap); err != nil {
			return false, err
		}
		if gap.Cmp(width) < 0 {
			return false, ErrUnrepresentable
		}
		return false, nil
	})
	*visits += 1 + 2*splits
	if err != nil {
		return nil, err
	}
	return roots, nil
}
func graphCircleStockProjection(circle GraphContactCircle, b *polynomialBudget) (*big.Rat, [2]*big.Rat, error) {
	var projection [2]*big.Rat
	radius, err := graphCircleSquareRoot(circle.RadiusSquared, b)
	if err != nil {
		return nil, projection, err
	}
	one := big.NewRat(1, 1)
	for dim, coordinate := range circle.Center {
		lo, hi := new(big.Rat).Sub(coordinate, radius), new(big.Rat).Add(coordinate, radius)
		for _, value := range [2]*big.Rat{lo, hi} {
			if err := b.check(value); err != nil {
				return nil, projection, err
			}
		}
		if lo.Sign() <= 0 || hi.Cmp(one) >= 0 {
			return nil, projection, ErrConvergence
		}
		if dim == 0 {
			projection = [2]*big.Rat{lo, hi}
		}
	}
	return radius, projection, nil
}
