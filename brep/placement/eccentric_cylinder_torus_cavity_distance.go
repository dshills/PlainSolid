package placement

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// EccentricCylinderTorusCavityOccurrenceDistance retains original support IDs and unchanged
// squared Euclidean distance units. World witnesses are independently owned;
// changing them cannot alter Base or the immutable original occurrence.
type EccentricCylinderTorusCavityOccurrenceDistance struct {
	Base      geom.EccentricCylinderTorusCavityDistance
	Witnesses [4]TorusCylinderWorldDistanceWitness
}

// BoundBoundaryDistance performs one original retained-support query. Proper
// exact rigidity proves unchanged squared bounds and complete possible-owner
// semantics. Pose mapping is separately bounded to4096-bit arithmetic, with at
// most21 vector/point transforms; Base.ExactOperations counts source work only.
// Any failure, including witness mapping after source success, is atomic.
func BoundEccentricCylinderTorusCavityDistance(ctx context.Context, source geom.EccentricCylinderTorusCavityMaterial, pose RationalRigid, p [3]*big.Rat, o geom.TorusCylinderDistanceOptions) (EccentricCylinderTorusCavityOccurrenceDistance, error) {
	var zero EccentricCylinderTorusCavityOccurrenceDistance
	q, e := pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return zero, e
	}
	base, e := source.BoundBoundaryDistance(ctx, q, o)
	if e != nil {
		return zero, e
	}
	clone := func(v *big.Rat) *big.Rat { return new(big.Rat).Set(v) }
	out := EccentricCylinderTorusCavityOccurrenceDistance{Base: base}
	for i, v := range base.Supports {
		if v.Owner == 0 {
			continue
		}
		radial := [3]*big.Rat{new(big.Rat).Sub(q[0], base.ChartOrigins[i][0]), new(big.Rat).Sub(q[1], base.ChartOrigins[i][1]), new(big.Rat)}
		for _, v := range radial {
			if e := checkRigidRat(v, 4096); e != nil {
				return zero, e
			}
		}
		rho2 := new(big.Rat).Add(new(big.Rat).Mul(radial[0], radial[0]), new(big.Rat).Mul(radial[1], radial[1]))
		if e := checkRigidRat(rho2, 4096); e != nil {
			return zero, e
		}
		w := &out.Witnesses[i]
		w.Base = v.Witness
		w.Base.RadialSquared = clone(v.Witness.RadialSquared)
		w.Base.HeightBase = clone(v.Witness.HeightBase)
		w.Base.HeightRadicand = clone(v.Witness.HeightRadicand)
		w.QueryRadialSquared = clone(rho2)
		w.Origin, e = pose.TransformPoint(ctx, [3]*big.Rat{base.ChartOrigins[i][0], base.ChartOrigins[i][1], new(big.Rat)}, false, 4096)
		if e != nil {
			return zero, e
		}
		w.Axis, e = pose.TransformVector(ctx, [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, false, 4096)
		if e != nil {
			return zero, e
		}
		w.RadialVector, e = pose.TransformVector(ctx, radial, false, 4096)
		if e != nil {
			return zero, e
		}
		if w.Base.WholeCircle {
			for j := 0; j < 2; j++ {
				unit := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
				unit[j].SetInt64(1)
				w.CircleDirections[j], e = pose.TransformVector(ctx, unit, false, 4096)
				if e != nil {
					return zero, e
				}
			}
		}
	}
	if e := ctx.Err(); e != nil {
		return zero, e
	}
	return out, nil
}
