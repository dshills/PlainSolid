package geom

import (
	"context"
	"math/big"
)

// ProjectiveSurfaceHeightOrder gives a certified enclosure of A minus B
// over their exact closed common projection. When Complete is true, Sign
// is +1 or -1 for strict order everywhere, or 0 for identical lifts.
type ProjectiveSurfaceHeightOrder struct {
	Overlap         ProjectiveSurfaceGraphOverlap
	Difference      ExactParameterInterval
	Sign            int
	Complete        bool
	ExactOperations int
}

// CertifyProjectiveSurfaceHeightOrder checks all original common cells
// with one exact work limit. Inconclusive Bernstein hulls may subdivide
// within the same original cell. If exact work runs out or an original
// sample prevents strict ordering, the conservative enclosure is kept.
func CertifyProjectiveSurfaceHeightOrder(ctx context.Context, a, c *SurfacePatch, axes [2]int,
	o GeneralSurfaceHeightOptions) (r ProjectiveSurfaceHeightOrder, err error) {
	if o.MaxOperations == 0 {
		o.MaxOperations = 8_000_000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16_384
	}
	atlas, err := BuildProjectiveSurfaceHeightDifference(ctx, a, c, axes, o)
	r.ExactOperations = atlas.ExactOperations
	if err != nil {
		return r, err
	}
	b := &polynomialBudget{operations: r.ExactOperations, maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	type orderCell struct {
		p      *SurfacePatch
		bounds ExactParameterInterval
		depth  int
	}
	hull := func(p *SurfacePatch) (x ExactParameterInterval, err error) {
		for i, h := range p.net {
			if i&255 == 0 {
				if err := ctx.Err(); err != nil {
					return x, err
				}
			}
			v := new(big.Rat).Quo(h[2], h[3])
			if err := b.check(v); err != nil {
				return x, err
			}
			if x.Lower == nil || v.Cmp(x.Lower) < 0 {
				x.Lower = v
			}
			if x.Upper == nil || v.Cmp(x.Upper) > 0 {
				x.Upper = v
			}
		}
		return x, nil
	}
	extend := func(r *ExactParameterInterval, x ExactParameterInterval) {
		if r.Lower == nil || x.Lower.Cmp(r.Lower) < 0 {
			r.Lower = x.Lower
		}
		if r.Upper == nil || x.Upper.Cmp(r.Upper) > 0 {
			r.Upper = x.Upper
		}
	}
	var coarse ExactParameterInterval
	var pending []orderCell
	for _, cell := range atlas.Cells {
		p := cell.Difference
		span, err := hull(p)
		if err != nil {
			return r, err
		}
		extend(&coarse, span)
		pending = append(pending, orderCell{p: p, bounds: span})
	}
	if coarse.Lower == nil {
		return r, ErrConvergence
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	r.Overlap, r.Difference = atlas.Overlap, coarse
	if coarse.Lower.Sign() > 0 || coarse.Upper.Sign() < 0 || coarse.Lower.Sign() == 0 && coarse.Upper.Sign() == 0 {
		r.Sign, r.Complete = coarse.Lower.Sign(), true
		return r, nil
	}
	visited, sign := len(pending), 0
	var refined ExactParameterInterval
	for len(pending) > 0 {
		if err := ctx.Err(); err != nil {
			return r, err
		}
		i := len(pending) - 1
		n := pending[i]
		pending = pending[:i]
		// The four homogeneous corner coefficients are exact point
		// values. A zero or differing signs prove lack of strict order.
		p := n.p
		for _, id := range [4]int{0, p.countV - 1, (p.countU - 1) * p.countV, len(p.net) - 1} {
			s := p.net[id][2].Sign()
			if s == 0 || sign != 0 && s != sign {
				return r, ErrConvergence
			}
			sign = s
		}
		if n.bounds.Lower.Sign() > 0 || n.bounds.Upper.Sign() < 0 {
			extend(&refined, n.bounds)
			continue
		}
		if n.depth >= 64 || visited > o.MaxCells-2 {
			return r, ErrConvergence
		}
		d := p.Domain()
		u := d.UMax-d.UMin >= d.VMax-d.VMin
		lo, hi := d.VMin, d.VMax
		if u {
			lo, hi = d.UMin, d.UMax
		}
		m := lo + (hi-lo)/2
		if m <= lo || m >= hi {
			return r, ErrConvergence
		}
		left, right, err := p.split(m, u, b)
		if err != nil {
			return r, err
		}
		for _, child := range [2]*SurfacePatch{right, left} {
			span, err := hull(child)
			if err != nil {
				return r, err
			}
			pending = append(pending, orderCell{p: child, bounds: span, depth: n.depth + 1})
		}
		visited += 2
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	r.Difference, r.Sign, r.Complete = refined, sign, true
	return r, nil
}
