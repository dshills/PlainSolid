//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// AlgebraicGraphAffineMassOptions keeps original classification/integration
// budgets separate from exact affine interval propagation. Transform defaults
// to 4096 operations and 16384 coefficient bits; its maximums are the usual exact
// query limits. Original.MomentOrder retains its 0/1/2 meaning.
type AlgebraicGraphAffineMassOptions struct {
	Original  AlgebraicGraphWholeMassOptions
	Transform geom.ExactCurvePlaneEventOptions
}

// AlgebraicGraphAffineMass describes the exact represented nonsingular affine
// IMAGE of the proved native common, at unit density. Original remains entirely
// in original coordinates, including its boundary area and native ownership.
// All new intervals are owned and conservative; source correlations are not
// assumed independent when claiming equality. Packed tensors use XX,XY,XZ,YY,YZ,ZZ.
// This does not rebuild or classify an oblique native Boolean or transform area.
type AlgebraicGraphAffineMass struct {
	Original                                           AlgebraicGraphWholeMass
	Placement                                          geom.Affine
	AbsoluteDeterminant                                *big.Rat
	Volume                                             [2]*big.Rat
	FirstMoments, Centroid                             [3][2]*big.Rat
	RawSecondMoments, OriginInertia, CentroidalInertia [6][2]*big.Rat
	HasFirst, HasSecond, HasCentroid, HasCentroidal    bool
	TransformOperations                                int
}

// BoundAffineWholeMassProperties integrates one original whole partition and
// propagates m'=|det A|(A m+t V), Q'=|det A|(A Q A^T+A m t^T+
// t m^T A^T+t t^T V). Any nonsingular represented affine map is accepted,
// including reflection and maps mixing the old projection fibers. Centroidal
// inertia uses interval parallel-axis subtraction only with positive volume.
// Cancellation, coefficient and operation failures are atomic.
func (q *AlgebraicGraphMaterialQuery) BoundAffineWholeMassProperties(ctx context.Context, m geom.Affine, o AlgebraicGraphAffineMassOptions) (out AlgebraicGraphAffineMass, err error) {
	defer func() {
		if err != nil {
			out = AlgebraicGraphAffineMass{TransformOperations: out.TransformOperations}
		}
	}()
	if ctx == nil || q == nil || !m.IsFinite() {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if o.Transform.MaxOperations == 0 {
		o.Transform.MaxOperations = 4096
	}
	o.Transform, err = graphMaterialLimits(o.Transform)
	if err != nil {
		return out, err
	}
	var failure error
	calc := func(op byte, a, b *big.Rat) *big.Rat {
		r := new(big.Rat)
		if failure != nil {
			return r
		}
		if e := ctx.Err(); e != nil {
			failure = e
			return r
		}
		if out.TransformOperations >= o.Transform.MaxOperations || !graphPolynomialBits(a, o.Transform.MaxCoefficientBits) || !graphPolynomialBits(b, o.Transform.MaxCoefficientBits) {
			failure = geom.ErrConvergence
			return r
		}
		out.TransformOperations++
		switch op {
		case '+':
			r.Add(a, b)
		case '-':
			r.Sub(a, b)
		case '*':
			r.Mul(a, b)
		case '/':
			if b.Sign() == 0 {
				failure = geom.ErrConvergence
				return r
			}
			r.Quo(a, b)
		}
		if !graphPolynomialBits(r, o.Transform.MaxCoefficientBits) {
			failure = geom.ErrConvergence
		}
		return r
	}
	var a [3][4]*big.Rat
	for i := range a {
		for j := range a[i] {
			a[i][j] = ratInset(m[i][j])
		}
	}
	minor := func(i, j int) *big.Rat { return calc('-', calc('*', a[1][i], a[2][j]), calc('*', a[1][j], a[2][i])) }
	determinant := calc('+', calc('-', calc('*', a[0][0], minor(1, 2)), calc('*', a[0][1], minor(0, 2))), calc('*', a[0][2], minor(0, 1)))
	if failure != nil {
		return out, failure
	}
	if determinant.Sign() == 0 {
		return out, geom.ErrConvergence
	}
	determinant.Abs(determinant)
	out.Original, err = q.BoundWholeMassProperties(ctx, o.Original)
	if err != nil {
		return out, err
	}
	type interval = [2]*big.Rat
	singleton := func(x *big.Rat) interval { return interval{x, x} }
	zero := func() interval { return interval{new(big.Rat), new(big.Rat)} }
	add := func(x, y interval) interval { return interval{calc('+', x[0], y[0]), calc('+', x[1], y[1])} }
	sub := func(x, y interval) interval { return interval{calc('-', x[0], y[1]), calc('-', x[1], y[0])} }
	mul := func(x, y interval) interval {
		var r interval
		for _, u := range x {
			for _, v := range y {
				z := calc('*', u, v)
				if r[0] == nil || z.Cmp(r[0]) < 0 {
					r[0] = z
				}
				if r[1] == nil || z.Cmp(r[1]) > 0 {
					r[1] = z
				}
			}
		}
		return r
	}
	scale := func(x interval, s *big.Rat) interval { return mul(x, singleton(s)) }
	out.Volume = scale(out.Original.Whole.Volume, determinant)
	if o.Original.MomentOrder >= 1 {
		first := out.Original.Moments.FirstMoments
		for i := 0; i < 3; i++ {
			v := scale(out.Original.Whole.Volume, a[i][3])
			for j := 0; j < 3; j++ {
				v = add(v, scale(first[j], a[i][j]))
			}
			out.FirstMoments[i] = scale(v, determinant)
		}
		out.HasFirst = true
		if out.Original.Moments.HasCentroid {
			for i := 0; i < 3; i++ {
				v := singleton(a[i][3])
				for j := 0; j < 3; j++ {
					v = add(v, scale(out.Original.Moments.Centroid[j], a[i][j]))
				}
				out.Centroid[i] = v
			}
			out.HasCentroid = true
		}
	}
	pairs := [6][2]int{{0, 0}, {0, 1}, {0, 2}, {1, 1}, {1, 2}, {2, 2}}
	inertia := func(raw [6]interval) (r [6]interval) {
		r[0] = add(raw[3], raw[5])
		r[3] = add(raw[0], raw[5])
		r[5] = add(raw[0], raw[3])
		for _, i := range []int{1, 2, 4} {
			r[i] = sub(zero(), raw[i])
		}
		return r
	}
	if o.Original.MomentOrder == 2 {
		var homogeneous [4][4]interval
		for k, p := range pairs {
			homogeneous[p[0]][p[1]] = out.Original.Inertia.RawSecondMoments[k]
			homogeneous[p[1]][p[0]] = homogeneous[p[0]][p[1]]
		}
		for i := 0; i < 3; i++ {
			homogeneous[i][3] = out.Original.Moments.FirstMoments[i]
			homogeneous[3][i] = homogeneous[i][3]
		}
		homogeneous[3][3] = out.Original.Whole.Volume
		for k, p := range pairs {
			v := zero()
			for i := 0; i < 4; i++ {
				for j := 0; j < 4; j++ {
					v = add(v, scale(homogeneous[i][j], calc('*', a[p[0]][i], a[p[1]][j])))
				}
			}
			out.RawSecondMoments[k] = scale(v, determinant)
		}
		out.OriginInertia = inertia(out.RawSecondMoments)
		out.HasSecond = true
		if out.Volume[0].Sign() > 0 {
			reciprocal := interval{calc('/', big.NewRat(1, 1), out.Volume[1]), calc('/', big.NewRat(1, 1), out.Volume[0])}
			var central [6]interval
			for k, p := range pairs {
				central[k] = sub(out.RawSecondMoments[k], mul(mul(out.FirstMoments[p[0]], out.FirstMoments[p[1]]), reciprocal))
			}
			out.CentroidalInertia = inertia(central)
			out.HasCentroidal = true
		}
	}
	if failure != nil {
		return out, failure
	}
	out.Placement = m
	out.AbsoluteDeterminant = new(big.Rat).Set(determinant)
	return out, nil
}
