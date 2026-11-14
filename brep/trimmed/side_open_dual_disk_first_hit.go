package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

// FirstBoundaryHit reports the first isolated or continuous boundary contact
// of the original cut. Normals and point bounds use world coordinates; support
// axis names retain the original stock-local face ancestry. The rational world line is origin+t*direction on the
// closed range [minT,maxT]. No mesh or closest-point tolerance is used.
func (c SideOpenDualDiskCut) FirstBoundaryHit(ctx context.Context, origin, direction [3]*big.Rat,
	minT, maxT *big.Rat, o geom.ToroidLineOptions) (geom.BoxDualCircleFirstHit, error) {
	return firstSideOpenDualDiskHit(ctx, c.source, origin, direction, minT, maxT, o)
}

// FirstBoundaryHit is the unequal-radius counterpart of the exact
// finite-line query on SideOpenDualDiskCut.
func (c UnequalSideOpenDualDiskCut) FirstBoundaryHit(ctx context.Context, origin, direction [3]*big.Rat,
	minT, maxT *big.Rat, o geom.ToroidLineOptions) (geom.BoxDualCircleFirstHit, error) {
	return firstSideOpenDualDiskHit(ctx, c.source, origin, direction, minT, maxT, o)
}

func firstSideOpenDualDiskHit(ctx context.Context, s *sideOpenDualDiskSource, origin, direction [3]*big.Rat,
	minT, maxT *big.Rat, o geom.ToroidLineOptions) (geom.BoxDualCircleFirstHit, error) {
	if ctx == nil || s == nil {
		return geom.BoxDualCircleFirstHit{}, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return geom.BoxDualCircleFirstHit{}, e
	}
	for i := range origin {
		if origin[i] == nil || direction[i] == nil {
			return geom.BoxDualCircleFirstHit{}, geom.ErrInvalid
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
	hit, err := geom.FirstBoxDualCircleBoundaryHitRational(ctx,
		[3]*big.Rat{ratInset(s.min.X), ratInset(s.min.Y), new(big.Rat)}, [3]*big.Rat{ratInset(s.max.X), ratInset(s.max.Y), ratInset(s.height)},
		[2]*big.Rat{ratInset(s.discs[0].Center.X), ratInset(s.discs[1].Center.X)}, [2]*big.Rat{ratInset(s.discs[0].Radius), ratInset(s.discs[1].Radius)},
		ratInset(s.depth), localOrigin, localDirection, minT, maxT, o)
	if err != nil || !hit.HasHit {
		return hit, err
	}
	// Signed coordinate frames are certified by construction. Transform exact
	// affine normal coefficients and interval endpoints without normalization.
	axes := [3]geom.Vec3{s.frame.XAxis(), s.frame.YAxis(), s.frame.ZAxis()}
	point := hit.PointBounds
	for i, a := range axes {
		for j, sign := range [3]float64{a.X, a.Y, a.Z} {
			if sign == 0 {
				continue
			}
			transform := func(iv geom.ExactParameterInterval, shift *big.Rat) geom.ExactParameterInterval {
				lo := new(big.Rat).Mul(iv.Lower, ratInset(sign))
				hi := new(big.Rat).Mul(iv.Upper, ratInset(sign))
				if sign < 0 {
					lo, hi = hi, lo
				}
				lo.Add(lo, shift)
				hi.Add(hi, shift)
				return geom.ExactParameterInterval{Lower: lo, Upper: hi}
			}
			hit.PointBounds[j] = transform(point[i], ratInset([3]float64{f.X, f.Y, f.Z}[j]))
		}
	}
	for k, n := range hit.Normals {
		for i, a := range axes {
			for j, sign := range [3]float64{a.X, a.Y, a.Z} {
				if sign == 0 {
					continue
				}
				hit.Normals[k].Constant[j] = new(big.Rat).Mul(n.Constant[i], ratInset(sign))
				hit.Normals[k].Linear[j] = new(big.Rat).Mul(n.Linear[i], ratInset(sign))
				lo := new(big.Rat).Mul(n.Bounds[i].Lower, ratInset(sign))
				hi := new(big.Rat).Mul(n.Bounds[i].Upper, ratInset(sign))
				if sign < 0 {
					lo, hi = hi, lo
				}
				hit.Normals[k].Bounds[j] = geom.ExactParameterInterval{Lower: lo, Upper: hi}
			}
		}
	}
	return hit, nil

}
