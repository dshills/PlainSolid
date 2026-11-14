package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/placement"
	"plainsolid/geom"
)

// PlacedNativeObliqueTorusCavityDistance retains original native IDs and exact
// symbolic world witnesses. Base is in original stock coordinates; squared
// Euclidean bounds and owner masks are invariant under the proved proper pose.
type PlacedNativeObliqueTorusCavityDistance struct {
	Base      NativeObliqueTorusCavityDistance
	Witnesses [4]placement.TorusCylinderWorldDistanceWitness
}

// BoundPlacedBoundaryDistance performs exactly one prepared source query.
// The independent whole occurrence never replaces the local torus frame.
// Pose/witness mapping has a separate 4096-bit limit and at most 21 transforms;
// Base.ExactOperations counts the shared source distance ledger only.
func (p *PreparedObliqueTorusCavity) BoundPlacedBoundaryDistance(ctx context.Context, pose placement.RationalRigid, point [3]*big.Rat, o geom.TorusCylinderDistanceOptions) (PlacedNativeObliqueTorusCavityDistance, error) {
	var zero PlacedNativeObliqueTorusCavityDistance
	local, e := pose.TransformPoint(ctx, point, true, 4096)
	if e != nil {
		return zero, e
	}
	base, e := p.BoundBoundaryDistance(ctx, local, o)
	if e != nil {
		return zero, e
	}
	out := PlacedNativeObliqueTorusCavityDistance{Base: base}
	check := func(q *big.Rat) error {
		if q == nil || q.Num().BitLen() > 4096 || q.Denom().BitLen() > 4096 {
			return geom.ErrConvergence
		}
		return nil
	}
	for i, support := range base.Supports {
		q := local
		var frame [3][4]*big.Rat
		for j := 0; j < 3; j++ {
			for k := 0; k < 4; k++ {
				frame[j][k] = new(big.Rat)
				if j == k {
					frame[j][k].SetInt64(1)
				}
			}
		}
		if i == 0 {
			frame = base.TorusFrame
			q = base.TorusQuery
		}
		for _, v := range q {
			if e := check(v); e != nil {
				return zero, e
			}
		}
		for _, row := range frame {
			for _, v := range row {
				if e := check(v); e != nil {
					return zero, e
				}
			}
		}
		w := &out.Witnesses[i]
		w.Base = support.Witness
		w.Base.RadialSquared = new(big.Rat).Set(support.Witness.RadialSquared)
		w.Base.HeightBase = new(big.Rat).Set(support.Witness.HeightBase)
		w.Base.HeightRadicand = new(big.Rat).Set(support.Witness.HeightRadicand)
		w.QueryRadialSquared = new(big.Rat).Add(new(big.Rat).Mul(q[0], q[0]), new(big.Rat).Mul(q[1], q[1]))
		if e := check(w.QueryRadialSquared); e != nil {
			return zero, e
		}
		var origin, axis, radial [3]*big.Rat
		for j := 0; j < 3; j++ {
			origin[j] = frame[j][3]
			axis[j] = frame[j][2]
			radial[j] = new(big.Rat).Add(new(big.Rat).Mul(frame[j][0], q[0]), new(big.Rat).Mul(frame[j][1], q[1]))
			if e := check(radial[j]); e != nil {
				return zero, e
			}
		}
		w.Origin, e = pose.TransformPoint(ctx, origin, false, 4096)
		if e != nil {
			return zero, e
		}
		w.Axis, e = pose.TransformVector(ctx, axis, false, 4096)
		if e != nil {
			return zero, e
		}
		w.RadialVector, e = pose.TransformVector(ctx, radial, false, 4096)
		if e != nil {
			return zero, e
		}
		if w.Base.WholeCircle {
			for k := 0; k < 2; k++ {
				var v [3]*big.Rat
				for j := range v {
					v[j] = frame[j][k]
				}
				w.CircleDirections[k], e = pose.TransformVector(ctx, v, false, 4096)
				if e != nil {
					return zero, e
				}
			}
		}
	}
	return out, nil
}
