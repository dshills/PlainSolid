package geom

import (
	"context"
	"math/big"
)

// PlanarRoundedRectangle has one equal radius at all four corners. It is the
// closed Minkowski sum of a core rectangle with
// a corner disk, in an exact coordinate-axis frame. HalfWidth/HalfHeight include
// corner rounding. CornerRadius=0 gives a rectangle; equal half sizes/radius
// give a disk. The represented axes must be signed coordinate unit vectors.
type PlanarRoundedRectangle struct {
	frame                               Frame
	halfWidth, halfHeight, cornerRadius float64
}

func NewPlanarRoundedRectangle(frame Frame, halfWidth, halfHeight, cornerRadius float64) (PlanarRoundedRectangle, error) {
	if !frame.valid() || !finite(halfWidth) || !finite(halfHeight) || !finite(cornerRadius) || halfWidth <= 0 || halfHeight <= 0 || cornerRadius < 0 || cornerRadius > halfWidth || cornerRadius > halfHeight {
		return PlanarRoundedRectangle{}, ErrInvalid
	}
	seen := [3]bool{}
	for _, v := range []Vec3{frame.x, frame.y, frame.z} {
		axis := -1
		for i, q := range []float64{v.X, v.Y, v.Z} {
			if q != 0 {
				if axis >= 0 || (q != 1 && q != -1) {
					return PlanarRoundedRectangle{}, ErrConvergence
				}
				axis = i
			}
		}
		if axis < 0 || seen[axis] {
			return PlanarRoundedRectangle{}, ErrConvergence
		}
		seen[axis] = true
	}
	return PlanarRoundedRectangle{frame, halfWidth, halfHeight, cornerRadius}, nil
}
func (r PlanarRoundedRectangle) Parameters() (Frame, float64, float64, float64) {
	return r.frame, r.halfWidth, r.halfHeight, r.cornerRadius
}

type RoundedDiskRelation uint8

const (
	RoundedDiskNonCoplanar RoundedDiskRelation = iota
	RoundedDiskStrictlyCovered
	RoundedDiskCoveredTouching
	RoundedDiskOutside
	RoundedDiskOutsideTouching
	RoundedDiskOverlap
)

type RoundedRectangleDiskComparison struct {
	Complete, Coplanar    bool
	Relation              RoundedDiskRelation
	Center                [2]*big.Rat
	SquaredDistanceToCore *big.Rat
	ExactOperations       int
}

// CompareRoundedRectangleDisk classifies an ideal closed disk in the rectangle
// plane. It uses exact represented coordinates and no rounded corner samples.
// The exterior distance test is dist(center,core) versus cornerRadius+toolRadius.
// For toolRadius<cornerRadius the eroded region is core+disk(r-R); otherwise it
// is the rectangle with half sizes (halfWidth-R,halfHeight-R). The equality case
// is handled separately so a radius-r tool centered strictly inside the core
// is not falsely called tangent. NonCoplanar makes no spatial-overlap claim.
func CompareRoundedRectangleDisk(ctx context.Context, rectangle PlanarRoundedRectangle, disk PlanarCircularProfile, o SurfacePatchEvaluationOptions) (out RoundedRectangleDiskComparison, err error) {
	if ctx == nil || !disk.valid() {
		return out, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	r, e := NewPlanarRoundedRectangle(rectangle.frame, rectangle.halfWidth, rectangle.halfHeight, rectangle.cornerRadius)
	if e != nil {
		return out, e
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 100000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return out, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { out.ExactOperations = b.operations }()
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	vec := func(v Vec3) [3]*big.Rat { return [3]*big.Rat{rat(v.X), rat(v.Y), rat(v.Z)} }
	origin, center := vec(r.frame.origin), vec(disk.center)
	normal, axis := vec(disk.normal), vec(r.frame.z)
	var delta [3]*big.Rat
	offset := new(big.Rat)
	parallel := true
	for i := 0; i < 3; i++ {
		j, k := (i+1)%3, (i+2)%3
		cross := new(big.Rat).Sub(new(big.Rat).Mul(normal[j], axis[k]), new(big.Rat).Mul(normal[k], axis[j]))
		if cross.Sign() != 0 {
			parallel = false
		}
		delta[i] = new(big.Rat).Sub(center[i], origin[i])
		offset.Add(offset, new(big.Rat).Mul(axis[i], delta[i]))
		for _, q := range []*big.Rat{normal[i], center[i], origin[i], cross, delta[i]} {
			if e := b.check(q); e != nil {
				return out, e
			}
		}
	}
	if !parallel || offset.Sign() != 0 {
		out.Complete = true
		out.Relation = RoundedDiskNonCoplanar
		return out, nil
	}
	out.Coplanar = true
	for i, v := range []Vec3{r.frame.x, r.frame.y} {
		q := new(big.Rat)
		a := vec(v)
		for j := 0; j < 3; j++ {
			q.Add(q, new(big.Rat).Mul(a[j], delta[j]))
		}
		out.Center[i] = q
	}
	half := [2]*big.Rat{rat(r.halfWidth), rat(r.halfHeight)}
	corner, tool := rat(r.cornerRadius), rat(disk.radius)
	distance := new(big.Rat)
	for i, q := range out.Center {
		core := new(big.Rat).Sub(half[i], corner)
		gap := new(big.Rat).Sub(new(big.Rat).Abs(q), core)
		if gap.Sign() > 0 {
			distance.Add(distance, new(big.Rat).Mul(gap, gap))
		}
	}
	if e := b.check(distance); e != nil {
		return out, e
	}
	out.SquaredDistanceToCore = distance
	sum := new(big.Rat).Add(corner, tool)
	sum.Mul(sum, sum)
	if e := b.check(sum); e != nil {
		return out, e
	}
	if cmp := distance.Cmp(sum); cmp >= 0 {
		out.Relation = RoundedDiskOutside
		if cmp == 0 {
			out.Relation = RoundedDiskOutsideTouching
		}
		out.Complete = true
		return out, nil
	}
	covered, touch := false, false
	if tool.Cmp(corner) < 0 {
		difference := new(big.Rat).Sub(corner, tool)
		difference.Mul(difference, difference)
		if e := b.check(difference); e != nil {
			return out, e
		}
		cmp := distance.Cmp(difference)
		covered, touch = cmp <= 0, cmp == 0
	} else {
		covered = true
		for i, q := range out.Center {
			bound := new(big.Rat).Sub(half[i], tool)
			if e := b.check(bound); e != nil {
				return out, e
			}
			cmp := new(big.Rat).Abs(q).Cmp(bound)
			if cmp > 0 {
				covered = false
			}
			touch = touch || cmp == 0
		}
	}
	out.Relation = RoundedDiskOverlap
	if covered {
		out.Relation = RoundedDiskStrictlyCovered
		if touch {
			out.Relation = RoundedDiskCoveredTouching
		}
	}
	out.Complete = true
	return out, nil
}
