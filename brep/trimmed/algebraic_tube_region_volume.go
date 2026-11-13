package trimmed

import (
	"context"
	"errors"
	"math/big"

	"plainsolid/geom"
)

type AlgebraicTubeRegionVolumeBounds struct {
	Regions                       []geom.ExactParameterInterval
	VisitedCells, ExactOperations int
}

// BoundVolumes encloses each region's volume over exactly aligned
// rectangular source charts. A cell is assigned only when its whole
// rectangle is separated from all possible rim images and its exact
// center is located. Boundary cells contribute conservative upper
// bounds to every region with the compatible height sign.
func (r AlgebraicTubeRegions) BoundVolumes(ctx context.Context, o AlgebraicTubeLensVolumeOptions) (out AlgebraicTubeRegionVolumeBounds, err error) {
	if ctx == nil || r.graphs[0] == nil || o.MaxDepth < 0 || o.MaxDepth > 18 || o.MaxCells < 0 || o.MaxCells > 262144 {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	defer func() {
		if err != nil {
			out.Regions = nil
		}
	}()
	if o.MaxDepth == 0 {
		o.MaxDepth = 8
	}
	if o.MaxCells == 0 {
		o.MaxCells = 32768
	}
	if o.Patch.MaxOperations == 0 {
		o.Patch.MaxOperations = 128000000
	}
	if o.Patch.MaxCoefficientBits == 0 {
		o.Patch.MaxCoefficientBits = 16384
	}
	if o.Patch.MaxOperations < 1 || o.Patch.MaxCoefficientBits < 64 {
		return out, geom.ErrInvalid
	}
	limits := func() geom.SurfacePatchOptions { p := o.Patch; p.MaxOperations -= out.ExactOperations; return p }
	var ps [2]*geom.SurfacePatch
	for i, f := range r.section.partitionSource.faces {
		d, e := faceUVBounds(f)
		if e != nil {
			return out, e
		}
		if !splineDistanceRectangle(f, d) {
			return out, geom.ErrConvergence
		}
		p := limits()
		if p.MaxOperations < 1 {
			return out, geom.ErrConvergence
		}
		s, e := geom.NewSurfacePatch(facespline(f), p)
		if e != nil {
			return out, e
		}
		out.ExactOperations += s.ExactOperations()
		ps[i], e = s.Trim(d)
		if e != nil {
			return out, e
		}
		out.ExactOperations += ps[i].ExactOperations()
	}
	p := limits()
	if p.MaxOperations < 1 {
		return out, geom.ErrConvergence
	}
	m, e := geom.AlignAffineXYCharts(ctx, ps[0], ps[1], p)
	if e != nil {
		return out, e
	}
	out.ExactOperations += m.ExactOperations()
	p = limits()
	if p.MaxOperations < 1 {
		return out, geom.ErrConvergence
	}
	delta, e := geom.NewGeneralSurfaceHeightDifferencePatch(ctx, ps[0], m.TargetOnSource(), geom.GeneralSurfaceHeightOptions{
		MaxPoles: p.MaxPoles, MaxOperations: p.MaxOperations, MaxCoefficientBits: p.MaxCoefficientBits})
	if e != nil {
		return out, e
	}
	out.ExactOperations += delta.ExactOperations()
	c, e := r.graphs[0].InverseCoefficients()
	if e != nil {
		return out, e
	}
	det := new(big.Rat).Abs(new(big.Rat).Sub(new(big.Rat).Mul(c[0][0], c[1][1]), new(big.Rat).Mul(c[0][1], c[1][0])))
	jac := new(big.Rat).Inv(det)
	var rims []geom.UVBox
	for _, q := range r.partitions {
		for _, s := range q.Faces[0].Cut {
			b, e := s.Segment.TrimA().BoundsUV()
			if e != nil {
				return out, e
			}
			rims = append(rims, b)
		}
	}
	type cell struct {
		p      *geom.SurfacePatch
		depth  int
		owner  int
		bounds geom.ExactParameterInterval
	}
	queue := []cell{{p: delta, owner: -2}}
	out.Regions = make([]geom.ExactParameterInterval, len(r.regions))
	for i := range out.Regions {
		out.Regions[i] = geom.ExactParameterInterval{Lower: new(big.Rat), Upper: new(big.Rat)}
	}
	for len(queue) > 0 {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		if out.VisitedCells >= o.MaxCells {
			return out, geom.ErrConvergence
		}
		x := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		out.VisitedCells++
		d := x.p.Domain()
		b, e := x.p.Bounds()
		if e != nil {
			return out, e
		}
		if !finite(b.Min.Z) || !finite(b.Max.Z) {
			return out, geom.ErrConvergence
		}
		z := lensNestedInterval(x.bounds, ratInset(b.Min.Z), ratInset(b.Max.Z))
		if x.owner == -2 {
			separate := true
			for _, q := range rims {
				if q.Min.X <= d.UMax && q.Max.X >= d.UMin && q.Min.Y <= d.VMax && q.Max.Y >= d.VMin {
					separate = false
					break
				}
			}
			if separate {
				p := limits()
				if p.MaxOperations < 1 {
					return out, geom.ErrConvergence
				}
				u := new(big.Rat).Quo(new(big.Rat).Add(ratInset(d.UMin), ratInset(d.UMax)), big.NewRat(2, 1))
				v := new(big.Rat).Quo(new(big.Rat).Add(ratInset(d.VMin), ratInset(d.VMax)), big.NewRat(2, 1))
				at, e := r.graphs[0].EvaluateRational(ctx, [2]*big.Rat{u, v}, geom.SurfacePatchEvaluationOptions{MaxOperations: p.MaxOperations, MaxCoefficientBits: p.MaxCoefficientBits})
				out.ExactOperations += at.ExactOperations
				if e != nil {
					return out, e
				}
				p = limits()
				if p.MaxOperations < 1 {
					return out, geom.ErrConvergence
				}
				here, e := r.VerticalInterval(ctx, [2]*big.Rat{at.Point[0], at.Point[1]}, geom.ExactCurvePlaneEventOptions{
					MaxOperations: p.MaxOperations, MaxCoefficientBits: p.MaxCoefficientBits})
				out.ExactOperations += here.ExactOperations
				if e != nil && !errors.Is(e, geom.ErrConvergence) {
					return out, e
				}
				if limits().MaxOperations < 0 {
					return out, geom.ErrConvergence
				}
				if e == nil && here.Complete && !here.Rim {
					x.owner = here.Region
				}
			}
		}
		if x.owner == -1 {
			continue
		}
		if x.depth < o.MaxDepth && z.Lower.Cmp(z.Upper) != 0 {
			l, h := d, d
			if d.UMax-d.UMin >= d.VMax-d.VMin {
				t := d.UMin/2 + d.UMax/2
				l.UMax, h.UMin = t, t
			} else {
				t := d.VMin/2 + d.VMax/2
				l.VMax, h.VMin = t, t
			}
			for _, d := range [2]geom.ParameterRectangle{h, l} {
				if limits().MaxOperations < 1 {
					return out, geom.ErrConvergence
				}
				p, e := x.p.Trim(d)
				if e != nil {
					return out, e
				}
				out.ExactOperations += p.ExactOperations()
				if limits().MaxOperations < 0 {
					return out, geom.ErrConvergence
				}
				queue = append(queue, cell{p, x.depth + 1, x.owner, z})
			}
			continue
		}
		a := new(big.Rat).Mul(jac, new(big.Rat).Mul(new(big.Rat).Sub(ratInset(d.UMax), ratInset(d.UMin)),
			new(big.Rat).Sub(ratInset(d.VMax), ratInset(d.VMin))))
		for i, q := range r.regions {
			if x.owner >= 0 && x.owner != i {
				continue
			}
			lo, hi := z.Lower, z.Upper
			if q.UpperChart == 1 {
				lo, hi = new(big.Rat).Neg(hi), new(big.Rat).Neg(lo)
			}
			if hi.Sign() > 0 {
				out.Regions[i].Upper.Add(out.Regions[i].Upper, new(big.Rat).Mul(a, hi))
			}
			if x.owner == i && lo.Sign() > 0 {
				out.Regions[i].Lower.Add(out.Regions[i].Lower, new(big.Rat).Mul(a, lo))
			}
		}
	}
	return out, nil
}
