package placement

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// TorusCylinderWorldDistanceWitness preserves an attained analytic upper
// witness exactly. Put h=Base.HeightBase+Base.HeightSign*sqrt(Base.HeightRadicand).
// For non-axis queries its world point is
// Origin+Axis*h+RadialVector*sqrt(Base.RadialSquared/QueryRadialSquared).
// Axis is an exact unit vector, RadialVector is the rotated original XY query,
// and their dot product is zero. For Base.WholeCircle the last term instead is
// sqrt(Base.RadialSquared)*(CircleDirections[0]*cos(t)+CircleDirections[1]*sin(t)),
// all t. These exact orthonormal vectors preserve the full attained family.
// Neither a sampled point nor an exact global minimizer is claimed.
type TorusCylinderWorldDistanceWitness struct {
	Base                       geom.TorusCylinderDistanceWitness
	Origin, Axis, RadialVector [3]*big.Rat
	CircleDirections           [2][3]*big.Rat
	QueryRadialSquared         *big.Rat
}

// TorusCylinderOccurrenceDistance retains original support IDs and unchanged
// squared Euclidean distance units. World witnesses are independently owned;
// changing them cannot alter Base or the immutable original occurrence.
type TorusCylinderOccurrenceDistance struct {
	Base      geom.TorusCylinderDistance
	Witnesses [4]TorusCylinderWorldDistanceWitness
}

// BoundBoundaryDistance performs one original retained-support query. Proper
// exact rigidity proves unchanged squared bounds and complete possible-owner
// semantics. Pose mapping is separately bounded to4096-bit arithmetic, with at
// most21 vector/point transforms; Base.ExactOperations counts source work only.
// Any failure, including witness mapping after source success, is atomic.
func (s TorusCylinderOccurrence) BoundBoundaryDistance(ctx context.Context, p [3]*big.Rat, o geom.TorusCylinderDistanceOptions) (TorusCylinderOccurrenceDistance, error) {
	var zero TorusCylinderOccurrenceDistance
	q, e := s.pose.TransformPoint(ctx, p, true, 4096)
	if e != nil {
		return zero, e
	}
	base, e := s.source.BoundBoundaryDistance(ctx, q, o)
	if e != nil {
		return zero, e
	}
	x2, y2 := new(big.Rat).Mul(q[0], q[0]), new(big.Rat).Mul(q[1], q[1])
	if e := checkRigidRat(x2, 4096); e != nil {
		return zero, e
	}
	if e := checkRigidRat(y2, 4096); e != nil {
		return zero, e
	}
	rho2 := new(big.Rat).Add(x2, y2)
	if e := checkRigidRat(rho2, 4096); e != nil {
		return zero, e
	}
	clone := func(v *big.Rat) *big.Rat { return new(big.Rat).Set(v) }
	out := TorusCylinderOccurrenceDistance{Base: base}
	for i, v := range base.Supports {
		if v.Owner == 0 {
			continue
		}
		w := &out.Witnesses[i]
		w.Base = v.Witness
		w.Base.RadialSquared = clone(v.Witness.RadialSquared)
		w.Base.HeightBase = clone(v.Witness.HeightBase)
		w.Base.HeightRadicand = clone(v.Witness.HeightRadicand)
		w.QueryRadialSquared = clone(rho2)
		w.Origin, e = s.pose.TransformPoint(ctx, [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}, false, 4096)
		if e != nil {
			return zero, e
		}
		w.Axis, e = s.pose.TransformVector(ctx, [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(1, 1)}, false, 4096)
		if e != nil {
			return zero, e
		}
		w.RadialVector, e = s.pose.TransformVector(ctx, [3]*big.Rat{q[0], q[1], new(big.Rat)}, false, 4096)
		if e != nil {
			return zero, e
		}
		if w.Base.WholeCircle {
			for j := 0; j < 2; j++ {
				unit := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
				unit[j].SetInt64(1)
				w.CircleDirections[j], e = s.pose.TransformVector(ctx, unit, false, 4096)
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
