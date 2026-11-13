package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

type AlgebraicLensVolumeOptions struct {
	Patch              geom.SurfacePatchOptions
	MaxDepth, MaxCells int
}

// AlgebraicLensVolumeBounds contains certified rational volume intervals for
// the negative and positive material halves, respectively. ResolutionReached
// means the requested subdivision depth was exhausted without a work error;
// the bound width is explicit and is not implied by the depth.
type AlgebraicLensVolumeBounds struct {
	Halves            [2]geom.ExactParameterInterval
	VisitedCells      int
	ResolutionReached bool
}

// BoundVolumes integrates the exact vertical height difference using bounded
// positive-weight control hulls. Exact interval tests on the conic and plane
// discard or retain each rectangle; a partially cut rectangle only adds a
// safe upper bound. No triangulation or numeric quadrature is used.
func (s AlgebraicLensPlaneSplit) BoundVolumes(ctx context.Context, o AlgebraicLensVolumeOptions) (AlgebraicLensVolumeBounds, error) {
	var empty AlgebraicLensVolumeBounds
	if ctx == nil || len(s.source.curves) != 2 || s.contact[0] == nil || o.MaxDepth < 0 || o.MaxDepth > 16 || o.MaxCells < 0 || o.MaxCells > 262144 {
		return empty, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 9
	}
	if o.MaxCells == 0 {
		o.MaxCells = 32768
	}
	var patch [2]*geom.SurfacePatch
	for i, f := range s.source.subdivision.source.faces {
		p, e := exactFaceSupportPatch(f, o.Patch)
		if e != nil {
			return empty, e
		}
		patch[i] = p
	}
	difference, e := geom.NewGeneralSurfaceHeightDifferencePatch(ctx, patch[0], patch[1], geom.GeneralSurfaceHeightOptions{
		MaxPoles: o.Patch.MaxPoles, MaxOperations: o.Patch.MaxOperations, MaxCoefficientBits: o.Patch.MaxCoefficientBits,
	})
	if e != nil {
		return empty, e
	}
	out := AlgebraicLensVolumeBounds{}
	for i := range out.Halves {
		out.Halves[i] = geom.ExactParameterInterval{Lower: new(big.Rat), Upper: new(big.Rat)}
	}
	line := s.source.curves[0].Partition.Line
	center, minimum := lensConicCenter(s.contact)
	type cell struct {
		p     *geom.SurfacePatch
		depth int
	}
	queue := []cell{{difference, 0}}
	for len(queue) > 0 {
		if e := ctx.Err(); e != nil {
			return empty, e
		}
		if out.VisitedCells >= o.MaxCells {
			return empty, geom.ErrConvergence
		}
		x := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		out.VisitedCells++
		d := x.p.Domain()
		u := geom.ExactParameterInterval{Lower: ratInset(d.UMin), Upper: ratInset(d.UMax)}
		v := geom.ExactParameterInterval{Lower: ratInset(d.VMin), Upper: ratInset(d.VMax)}
		conic := lensCenteredQuadraticBox(s.contact, center, minimum, u, v)
		if conic.Lower.Sign() > 0 {
			continue
		}
		plane := lensLinearBox(line, u, v)
		active := [2]bool{plane.Lower.Sign() <= 0, plane.Upper.Sign() >= 0}
		if x.depth < o.MaxDepth {
			axis := 0
			if d.VMax-d.VMin > d.UMax-d.UMin {
				axis = 1
			}
			left, right := d, d
			if axis == 0 {
				mid := d.UMin/2 + d.UMax/2
				left.UMax = mid
				right.UMin = mid
			} else {
				mid := d.VMin/2 + d.VMax/2
				left.VMax = mid
				right.VMin = mid
			}
			for _, q := range [2]geom.ParameterRectangle{right, left} {
				p, e := x.p.Trim(q)
				if e != nil {
					return empty, e
				}
				queue = append(queue, cell{p, x.depth + 1})
			}
			continue
		}
		b, e := x.p.Bounds()
		if e != nil {
			return empty, e
		}
		area := new(big.Rat).Mul(new(big.Rat).Sub(u.Upper, u.Lower), new(big.Rat).Sub(v.Upper, v.Lower))
		zlo, zhi := ratInset(max(0, b.Min.Z)), ratInset(max(0, b.Max.Z))
		for i, a := range active {
			if !a {
				continue
			}
			q := &out.Halves[i]
			q.Upper.Add(q.Upper, new(big.Rat).Mul(area, zhi))
			insidePlane := i == 0 && plane.Upper.Sign() <= 0 || i == 1 && plane.Lower.Sign() >= 0
			if insidePlane && conic.Upper.Sign() <= 0 {
				q.Lower.Add(q.Lower, new(big.Rat).Mul(area, zlo))
			}
		}
	}
	out.ResolutionReached = true
	return out, nil
}

func lensLinearBox(q [3]*big.Rat, u, v geom.ExactParameterInterval) geom.ExactParameterInterval {
	r := geom.ExactParameterInterval{Lower: new(big.Rat).Set(q[2]), Upper: new(big.Rat).Set(q[2])}
	for i, p := range [2]geom.ExactParameterInterval{u, v} {
		addLensBox(&r, q[i], p)
	}
	return r
}
func lensConicCenter(q [6]*big.Rat) (center [2]*big.Rat, minimum *big.Rat) {
	det := new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(4, 1), new(big.Rat).Mul(q[2], q[4])), new(big.Rat).Mul(q[5], q[5]))
	center[0] = new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(q[5], q[3]), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(q[4], q[1]))), det)
	center[1] = new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(q[5], q[1]), new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(q[2], q[3]))), det)
	minimum = new(big.Rat).Add(q[0], new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).Mul(q[1], center[0]), new(big.Rat).Mul(q[3], center[1])), big.NewRat(2, 1)))
	return center, minimum
}
func lensCenteredQuadraticBox(q [6]*big.Rat, center [2]*big.Rat, c *big.Rat, u, v geom.ExactParameterInterval) geom.ExactParameterInterval {
	u = geom.ExactParameterInterval{Lower: new(big.Rat).Sub(u.Lower, center[0]), Upper: new(big.Rat).Sub(u.Upper, center[0])}
	v = geom.ExactParameterInterval{Lower: new(big.Rat).Sub(v.Lower, center[1]), Upper: new(big.Rat).Sub(v.Upper, center[1])}
	r := geom.ExactParameterInterval{Lower: new(big.Rat).Set(c), Upper: new(big.Rat).Set(c)}
	product := func(a, b geom.ExactParameterInterval) geom.ExactParameterInterval {
		x := new(big.Rat).Mul(a.Lower, b.Lower)
		lo, hi := x, x
		for _, t := range []*big.Rat{new(big.Rat).Mul(a.Lower, b.Upper), new(big.Rat).Mul(a.Upper, b.Lower), new(big.Rat).Mul(a.Upper, b.Upper)} {
			lo, hi = minRat(lo, t), maxRat(hi, t)
		}
		return geom.ExactParameterInterval{Lower: lo, Upper: hi}
	}
	square := func(a geom.ExactParameterInterval) geom.ExactParameterInterval {
		x, y := new(big.Rat).Mul(a.Lower, a.Lower), new(big.Rat).Mul(a.Upper, a.Upper)
		lo := minRat(x, y)
		if a.Lower.Sign() <= 0 && a.Upper.Sign() >= 0 {
			lo = new(big.Rat)
		}
		return geom.ExactParameterInterval{Lower: lo, Upper: maxRat(x, y)}
	}
	addLensBox(&r, q[2], square(u))
	addLensBox(&r, q[4], square(v))
	addLensBox(&r, q[5], product(u, v))
	return r
}
func addLensBox(r *geom.ExactParameterInterval, k *big.Rat, p geom.ExactParameterInterval) {
	a, b := new(big.Rat).Mul(k, p.Lower), new(big.Rat).Mul(k, p.Upper)
	if a.Cmp(b) > 0 {
		a, b = b, a
	}
	r.Lower.Add(r.Lower, a)
	r.Upper.Add(r.Upper, b)
}
