//go:build research

package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// IntersectLineBoundaryTransitions reports exact ordered boundary events and
// independent material germs for the immutable original cut. World coordinates
// are transformed exactly through its certified signed coordinate frame; t is
// unchanged, including nonbinary rational origins, directions, and range caps.
// Support axes remain source-local ancestry. A missing Before/After means the
// side lies beyond the finite requested range, not material exterior.
// No mesh, sampled display curve, or nearest-point tolerance is involved.
func (c SideOpenDualDiskCut) IntersectLineBoundaryTransitions(ctx context.Context, origin, direction [3]*big.Rat,
	minT, maxT *big.Rat, o geom.ToroidLineOptions) (geom.BoxDualCircleBoundaryTransitions, error) {
	return intersectSideOpenDualDiskTransitions(ctx, c.source, origin, direction, minT, maxT, o)
}

// IntersectLineBoundaryTransitions is the unequal-radius counterpart of the exact
// finite-line query on SideOpenDualDiskCut.
func (c UnequalSideOpenDualDiskCut) IntersectLineBoundaryTransitions(ctx context.Context, origin, direction [3]*big.Rat,
	minT, maxT *big.Rat, o geom.ToroidLineOptions) (geom.BoxDualCircleBoundaryTransitions, error) {
	return intersectSideOpenDualDiskTransitions(ctx, c.source, origin, direction, minT, maxT, o)
}

func intersectSideOpenDualDiskTransitions(ctx context.Context, s *sideOpenDualDiskSource, origin, direction [3]*big.Rat,
	minT, maxT *big.Rat, o geom.ToroidLineOptions) (geom.BoxDualCircleBoundaryTransitions, error) {
	if ctx == nil || s == nil {
		return geom.BoxDualCircleBoundaryTransitions{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return geom.BoxDualCircleBoundaryTransitions{}, e
	}
	for i := range origin {
		if origin[i] == nil || direction[i] == nil {
			return geom.BoxDualCircleBoundaryTransitions{}, geom.ErrInvalid
		}
	}
	var localOrigin, localDirection [3]*big.Rat
	f := s.frame.Origin()
	for i, a := range [3]geom.Vec3{s.frame.XAxis(), s.frame.YAxis(), s.frame.ZAxis()} {
		for j, sign := range [3]float64{a.X, a.Y, a.Z} {
			if sign != 0 {
				axis := ratInset(sign)
				localOrigin[i] = new(big.Rat).Sub(origin[j], ratInset([3]float64{f.X, f.Y, f.Z}[j]))
				localOrigin[i].Mul(localOrigin[i], axis)
				localDirection[i] = new(big.Rat).Mul(direction[j], axis)
				break
			}
		}
	}
	return geom.IntersectBoxDualCircleBoundaryTransitionsRational(ctx,
		[3]*big.Rat{ratInset(s.min.X), ratInset(s.min.Y), new(big.Rat)}, [3]*big.Rat{ratInset(s.max.X), ratInset(s.max.Y), ratInset(s.height)},
		[2]*big.Rat{ratInset(s.discs[0].Center.X), ratInset(s.discs[1].Center.X)}, [2]*big.Rat{ratInset(s.discs[0].Radius), ratInset(s.discs[1].Radius)},
		ratInset(s.depth), localOrigin, localDirection, minT, maxT, o)
}
