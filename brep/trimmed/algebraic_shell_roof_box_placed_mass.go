//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// BoundMass maps the original column proof through a proper rigid occurrence.
// Its budget covers both the column proof and exact interval transport. Raw
// second moments are transformed directly, preserving their source bounds.
func (q PlacedRoofMaterialQuery) BoundMass(ctx context.Context, o MixedRoofMassOptions) (MixedRoofMass, error) {
	var zero MixedRoofMass
	var err error
	o.SurfacePatchEvaluationOptions, err = q.source.evaluationOptions(ctx, o.SurfacePatchEvaluationOptions)
	if err != nil {
		return zero, err
	}
	r, err := q.pose.Matrix()
	if err != nil {
		return zero, err
	}
	if _, err := placedMassBits(o.MaxCoefficientBits, r); err != nil {
		return zero, err
	}
	local, err := q.source.BoundMass(ctx, o)
	if err != nil {
		return zero, err
	}
	w := mixedRoofLineWork{o: MixedRoofLineOptions{SurfacePatchEvaluationOptions: o.SurfacePatchEvaluationOptions}, operations: local.ExactOperations}
	var failure error
	check := func(x *big.Rat) *big.Rat {
		if failure != nil {
			return new(big.Rat)
		}
		if e := ctx.Err(); e != nil {
			failure = e
			return new(big.Rat)
		}
		_, failure = w.rat(x)
		return x
	}
	accumulate := func(out *geom.ExactParameterInterval, x geom.ExactParameterInterval, coefficient *big.Rat) {
		if failure != nil || coefficient.Sign() == 0 {
			return
		}
		if coefficient.Sign() < 0 {
			x.Lower, x.Upper = x.Upper, x.Lower
		}
		a := check(new(big.Rat).Mul(x.Lower, coefficient))
		b := check(new(big.Rat).Mul(x.Upper, coefficient))
		out.Lower = check(new(big.Rat).Add(out.Lower, a))
		out.Upper = check(new(big.Rat).Add(out.Upper, b))
	}
	var rotated [3]geom.ExactParameterInterval
	out := MixedRoofMass{Volume: local.Volume, VisitedCells: local.VisitedCells, EnclosedCells: local.EnclosedCells}
	for i := range 3 {
		rotated[i] = mixedRoofZeroInterval()
		if local.Centroid[i].Lower != nil {
			out.Centroid[i] = geom.ExactParameterInterval{Lower: new(big.Rat).Set(r[i][3]), Upper: new(big.Rat).Set(r[i][3])}
		}
		for j := range 3 {
			accumulate(&rotated[i], local.FirstMoment[j], r[i][j])
			if local.Centroid[j].Lower != nil {
				accumulate(&out.Centroid[i], local.Centroid[j], r[i][j])
			}
		}
		out.FirstMoment[i] = mixedRoofCopyInterval(rotated[i])
		accumulate(&out.FirstMoment[i], local.Volume, r[i][3])
	}
	for i := range 3 {
		for j := i; j < 3; j++ {
			x := mixedRoofZeroInterval()
			for a := range 3 {
				for b := range 3 {
					c := check(new(big.Rat).Mul(r[i][a], r[j][b]))
					accumulate(&x, local.SecondMoment[a][b], c)
				}
			}
			accumulate(&x, rotated[i], r[j][3])
			accumulate(&x, rotated[j], r[i][3])
			accumulate(&x, local.Volume, check(new(big.Rat).Mul(r[i][3], r[j][3])))
			if i == j {
				if x.Lower.Sign() < 0 {
					x.Lower.SetInt64(0)
				}
				out.SecondMoment[i][i] = x
			} else {
				out.SecondMoment[i][j], out.SecondMoment[j][i] = x, mixedRoofCopyInterval(x)
				v := geom.ExactParameterInterval{Lower: check(new(big.Rat).Neg(x.Upper)), Upper: check(new(big.Rat).Neg(x.Lower))}
				out.Inertia[i][j], out.Inertia[j][i] = v, mixedRoofCopyInterval(v)
			}
		}
	}
	for i := range 3 {
		a, b := out.SecondMoment[(i+1)%3][(i+1)%3], out.SecondMoment[(i+2)%3][(i+2)%3]
		out.Inertia[i][i] = geom.ExactParameterInterval{Lower: check(new(big.Rat).Add(a.Lower, b.Lower)), Upper: check(new(big.Rat).Add(a.Upper, b.Upper))}
	}
	if failure != nil {
		return zero, failure
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	out.ExactOperations = w.operations
	return out, nil
}
