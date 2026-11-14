package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// AlgebraicGraphOccurrenceMassOptions keeps source integration and image
// propagation independently bounded. Original.MomentOrder retains 0/1/2 for
// volume/first/second moments; Transform defaults to 4096 rational operations.
type AlgebraicGraphOccurrenceMassOptions struct {
	Original  AlgebraicGraphWholeMassOptions
	Transform geom.ExactCurvePlaneEventOptions
}

// AlgebraicGraphOccurrenceMass describes unit-density image mass. Original
// retains its source-frame partition, boundary area and native owner IDs; its
// coordinates/areas are NOT world-image measurements. All image moments below
// use the exact owned Pose. Packed tensors use XX,XY,XZ,YY,YZ,ZZ. Unknown rim
// contributions remain conservative intervals; no mesh or rounded pose is used.
type AlgebraicGraphOccurrenceMass struct {
	Original                                           AlgebraicGraphWholeMass
	Pose                                               [3][4]*big.Rat
	AbsoluteDeterminant                                *big.Rat
	Volume                                             [2]*big.Rat
	FirstMoments, Centroid                             [3][2]*big.Rat
	RawSecondMoments, OriginInertia, CentroidalInertia [6][2]*big.Rat
	HasFirst, HasSecond, HasCentroid, HasCentroidal    bool
	TransformOperations                                int
}

// BoundWholeMassProperties uses one existing original whole partition, then
// the exact rational pose Jacobian and affine first/second tensor identities.
// It preserves nonbinary coefficients. Source admissibility and per-cell limits
// are unchanged; any later propagation failure publishes no partial mass.
func (o *AlgebraicGraphOccurrence) BoundWholeMassProperties(ctx context.Context, options AlgebraicGraphOccurrenceMassOptions) (out AlgebraicGraphOccurrenceMass, err error) {
	defer func() {
		if err != nil {
			out = AlgebraicGraphOccurrenceMass{TransformOperations: out.TransformOperations}
		}
	}()
	if ctx == nil || o == nil || o.query == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if options.Transform.MaxOperations == 0 {
		options.Transform.MaxOperations = 4096
	}
	options.Transform, err = graphMaterialLimits(options.Transform)
	if err != nil {
		return out, err
	}
	a := graphOccurrenceArithmetic{ctx: ctx, options: options.Transform}
	// In the selected (projection,residual) coordinate bases the pose is block
	// triangular. Permutation signs disappear in the absolute volume Jacobian.
	x, y := o.axes[0], o.axes[1]
	u, v := o.query.axes[0], o.query.axes[1]
	det := a.calc('*', a.calc('-', a.calc('*', o.pose[x][u], o.pose[y][v]), a.calc('*', o.pose[x][v], o.pose[y][u])), o.pose[o.residual][o.query.residual])
	det.Abs(det)
	out.TransformOperations = a.used
	if a.err != nil {
		return out, a.err
	}
	out.Original, err = o.query.BoundWholeMassProperties(ctx, options.Original)
	if err != nil {
		return out, err
	}
	type interval = [2]*big.Rat
	constant := func(x *big.Rat) interval { return interval{x, x} }
	zero := func() interval { return interval{new(big.Rat), new(big.Rat)} }
	add := func(x, y interval) interval { return interval{a.calc('+', x[0], y[0]), a.calc('+', x[1], y[1])} }
	sub := func(x, y interval) interval { return interval{a.calc('-', x[0], y[1]), a.calc('-', x[1], y[0])} }
	mul := func(x, y interval) (r interval) {
		for _, p := range x {
			for _, q := range y {
				z := a.calc('*', p, q)
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
	scale := func(x interval, s *big.Rat) interval { return mul(x, constant(s)) }
	out.Volume = scale(out.Original.Whole.Volume, det)
	if options.Original.MomentOrder >= 1 {
		for i := 0; i < 3; i++ {
			sum := scale(out.Original.Whole.Volume, o.pose[i][3])
			for j := 0; j < 3; j++ {
				sum = add(sum, scale(out.Original.Moments.FirstMoments[j], o.pose[i][j]))
			}
			out.FirstMoments[i] = scale(sum, det)
		}
		out.HasFirst = true
		if out.Original.Moments.HasCentroid {
			for i := 0; i < 3; i++ {
				sum := constant(o.pose[i][3])
				for j := 0; j < 3; j++ {
					sum = add(sum, scale(out.Original.Moments.Centroid[j], o.pose[i][j]))
				}
				out.Centroid[i] = sum
			}
			out.HasCentroid = true
		}
	}
	pairs := [6][2]int{{0, 0}, {0, 1}, {0, 2}, {1, 1}, {1, 2}, {2, 2}}
	inertia := func(q [6]interval) (r [6]interval) {
		r[0] = add(q[3], q[5])
		r[3] = add(q[0], q[5])
		r[5] = add(q[0], q[3])
		for _, i := range []int{1, 2, 4} {
			r[i] = sub(zero(), q[i])
		}
		return r
	}
	if options.Original.MomentOrder == 2 {
		var h [4][4]interval
		for i, p := range pairs {
			h[p[0]][p[1]] = out.Original.Inertia.RawSecondMoments[i]
			h[p[1]][p[0]] = h[p[0]][p[1]]
		}
		for i := 0; i < 3; i++ {
			h[i][3] = out.Original.Moments.FirstMoments[i]
			h[3][i] = h[i][3]
		}
		h[3][3] = out.Original.Whole.Volume
		for k, p := range pairs {
			sum := zero()
			for i := 0; i < 4; i++ {
				for j := 0; j < 4; j++ {
					sum = add(sum, scale(h[i][j], a.calc('*', o.pose[p[0]][i], o.pose[p[1]][j])))
				}
			}
			out.RawSecondMoments[k] = scale(sum, det)
		}
		out.OriginInertia = inertia(out.RawSecondMoments)
		out.HasSecond = true
		if out.Volume[0].Sign() > 0 {
			reciprocal := interval{a.calc('/', big.NewRat(1, 1), out.Volume[1]), a.calc('/', big.NewRat(1, 1), out.Volume[0])}
			var central [6]interval
			for k, p := range pairs {
				central[k] = sub(out.RawSecondMoments[k], mul(mul(out.FirstMoments[p[0]], out.FirstMoments[p[1]]), reciprocal))
			}
			out.CentroidalInertia = inertia(central)
			out.HasCentroidal = true
		}
	}
	out.TransformOperations = a.used
	if a.err != nil {
		return out, a.err
	}
	out.Pose = o.Pose()
	out.AbsoluteDeterminant = new(big.Rat).Set(det)
	return out, nil
}
