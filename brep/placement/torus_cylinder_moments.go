package placement

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// TorusCylinderOccurrenceMomentOptions keeps source measure tolerances local.
// Transformed intervals can widen (especially translated raw moments); no world
// width is silently promised. Mapping limits default4096 operations/4096 bits.
type TorusCylinderOccurrenceMomentOptions struct {
	Source                                              geom.TorusCylinderMomentOptions
	MaxTransformOperations, MaxTransformCoefficientBits int
}

// TorusCylinderOccurrenceMoments is a unit-density world report. Every scalar
// is an outward rational interval; Second[i][j]=integral world_i*world_j dV.
// Inertia is about world origin or center as named. Source is the single local
// certificate; MappingOperations is an independent bounded transform ledger.
type TorusCylinderOccurrenceMoments struct {
	Source                                 geom.TorusCylinderMoments
	Volume                                 [2]*big.Rat
	First, Centroid                        [3][2]*big.Rat
	Second, InertiaOrigin, InertiaCentroid [3][3][2]*big.Rat
	MappingOperations                      int
}

// BoundMoments propagates local diagonal symmetry by exact interval tensors.
// Qworld=R Q R^T+(R F)t^T+t(R F)^T+V tt^T. Central inertia uses
// R Icenter R^T directly, retaining the source's proved parallel-axis bound.
// Proper rigidity preserves physical units and volume. No float conversion or
// rounded surrogate body is used. Source and mapping failures are atomic.
func (s TorusCylinderOccurrence) BoundMoments(ctx context.Context, o TorusCylinderOccurrenceMomentOptions) (out TorusCylinderOccurrenceMoments, err error) {
	defer func() {
		if err != nil {
			out = TorusCylinderOccurrenceMoments{}
		}
	}()
	if ctx == nil {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if o.MaxTransformOperations == 0 {
		o.MaxTransformOperations = 4096
	}
	if o.MaxTransformCoefficientBits == 0 {
		o.MaxTransformCoefficientBits = 4096
	}
	if o.MaxTransformOperations < 1 || o.MaxTransformCoefficientBits < 64 || o.MaxTransformCoefficientBits > 65536 {
		return out, geom.ErrInvalid
	}
	matrix, e := s.pose.Matrix()
	if e != nil {
		return out, e
	}
	ops := 0
	var failure error
	check := func(q *big.Rat) {
		if failure != nil {
			return
		}
		if e := ctx.Err(); e != nil {
			failure = e
			return
		}
		ops++
		if ops > o.MaxTransformOperations {
			failure = geom.ErrConvergence
			return
		}
		failure = checkRigidRat(q, o.MaxTransformCoefficientBits)
	}
	for _, row := range matrix {
		for _, q := range row {
			check(q)
		}
	}
	if failure != nil {
		return out, failure
	}
	base, e := s.source.BoundMoments(ctx, o.Source)
	if e != nil {
		return out, e
	}
	out.Source = base
	scalar := func(x, y *big.Rat, kind int) *big.Rat {
		if failure != nil {
			return new(big.Rat)
		}
		var v *big.Rat
		switch kind {
		case 0:
			v = new(big.Rat).Add(x, y)
		case 1:
			v = new(big.Rat).Sub(x, y)
		default:
			v = new(big.Rat).Mul(x, y)
		}
		check(v)
		return v
	}
	type interval = [2]*big.Rat
	point := func(q *big.Rat) interval { check(q); return interval{new(big.Rat).Set(q), new(big.Rat).Set(q)} }
	zero := func() interval { return interval{new(big.Rat), new(big.Rat)} }
	add := func(x, y interval) interval { return interval{scalar(x[0], y[0], 0), scalar(x[1], y[1], 0)} }
	scale := func(x interval, c *big.Rat) interval {
		if c.Sign() < 0 {
			return interval{scalar(x[1], c, 2), scalar(x[0], c, 2)}
		}
		return interval{scalar(x[0], c, 2), scalar(x[1], c, 2)}
	}
	copyInterval := func(x interval) interval {
		check(x[0])
		check(x[1])
		return interval{new(big.Rat).Set(x[0]), new(big.Rat).Set(x[1])}
	}
	out.Volume = copyInterval(base.Volume)
	q := [3]interval{base.SecondXY, base.SecondXY, base.SecondZ}
	for _, v := range q {
		copyInterval(v)
	}
	copyInterval(base.FirstZ)
	copyInterval(base.CentroidZ)
	for _, v := range base.InertiaCentroid {
		copyInterval(v)
	}
	for i := 0; i < 3; i++ {
		out.First[i] = add(scale(base.FirstZ, matrix[i][2]), scale(base.Volume, matrix[i][3]))
		out.Centroid[i] = add(point(matrix[i][3]), scale(base.CentroidZ, matrix[i][2]))
		for j := 0; j < 3; j++ {
			raw, central := zero(), zero()
			for k := 0; k < 3; k++ {
				coefficient := scalar(matrix[i][k], matrix[j][k], 2)
				raw = add(raw, scale(q[k], coefficient))
				central = add(central, scale(base.InertiaCentroid[k], coefficient))
			}
			cross := scalar(scalar(matrix[i][2], matrix[j][3], 2), scalar(matrix[j][2], matrix[i][3], 2), 0)
			raw = add(raw, scale(base.FirstZ, cross))
			raw = add(raw, scale(base.Volume, scalar(matrix[i][3], matrix[j][3], 2)))
			out.Second[i][j] = raw
			out.InertiaCentroid[i][j] = central
		}
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if i != j {
				out.InertiaOrigin[i][j] = scale(out.Second[i][j], big.NewRat(-1, 1))
			} else {
				out.InertiaOrigin[i][j] = zero()
				for k := 0; k < 3; k++ {
					if k != i {
						out.InertiaOrigin[i][j] = add(out.InertiaOrigin[i][j], out.Second[k][k])
					}
				}
			}
		}
	}
	if failure != nil {
		return out, failure
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	out.MappingOperations = ops
	return out, nil
}
