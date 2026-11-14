package placement

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// ObliqueCylinderTorusCavityOccurrenceMoments preserves the local exact
// coefficients and propagates full world tensors with bounded interval algebra.
type ObliqueCylinderTorusCavityOccurrenceMoments struct {
	Source                                 geom.ObliqueCylinderTorusCavityMoments
	Volume                                 [2]*big.Rat
	First, Centroid                        [3][2]*big.Rat
	Second, InertiaOrigin, InertiaCentroid [3][3][2]*big.Rat
	MappingOperations                      int
}

// BoundMoments propagates the full local tensor by exact interval tensors.
// Qworld=R Q R^T+(R F)t^T+t(R F)^T+V tt^T. Central inertia uses
// R Icenter R^T directly, retaining the source's proved parallel-axis bound.
// Proper rigidity preserves physical units and volume. No float conversion or
// rounded surrogate body is used. Source and mapping failures are atomic.
func BoundObliqueCylinderTorusCavityMoments(ctx context.Context, source geom.ObliqueCylinderTorusCavityMaterial, pose RationalRigid, o TorusCylinderOccurrenceMomentOptions) (out ObliqueCylinderTorusCavityOccurrenceMoments, err error) {
	defer func() {
		if err != nil {
			out = ObliqueCylinderTorusCavityOccurrenceMoments{}
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
	matrix, e := pose.Matrix()
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
	base, e := source.BoundMoments(ctx, o.Source)
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
	for i := 0; i < 3; i++ {
		out.First[i] = scale(base.Volume, matrix[i][3])
		out.Centroid[i] = point(matrix[i][3])
		for k := 0; k < 3; k++ {
			out.First[i] = add(out.First[i], scale(base.First[k], matrix[i][k]))
			out.Centroid[i] = add(out.Centroid[i], scale(base.Centroid[k], matrix[i][k]))
		}
		for j := 0; j < 3; j++ {
			raw, central := zero(), zero()
			for k := 0; k < 3; k++ {
				for l := 0; l < 3; l++ {
					coefficient := scalar(matrix[i][k], matrix[j][l], 2)
					raw = add(raw, scale(base.Second[k][l], coefficient))
					central = add(central, scale(base.InertiaCentroid[k][l], coefficient))
				}
				cross := scalar(scalar(matrix[i][k], matrix[j][3], 2), scalar(matrix[j][k], matrix[i][3], 2), 0)
				raw = add(raw, scale(base.First[k], cross))
			}
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
