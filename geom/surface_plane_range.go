package geom

import (
	"context"
	"math"
	"math/big"
)

// SurfacePlaneRangeOptions bounds adaptive global extrema enclosure. Zero fields
// select absolute tolerance 1e-5, depth 24, 4096 cells, 4M operations, 16384 bits.
type SurfacePlaneRangeOptions struct {
	Tolerance                                             float64
	MaxDepth, MaxCells, MaxOperations, MaxCoefficientBits int
}

// SurfacePlaneExtrema encloses extrema of normal dot (S-origin), using the exact
// stored plane coefficients. Because normalization is floating point, this is
// the stored plane functional, not an exactly unit-normal Euclidean distance.
// Minimum and Maximum separately enclose global extrema. Witness parameters
// supply the inner bounds. StrictSign proves the whole patch on one side.
// On limits/cancellation the last established bounds survive; initially they
// are infinite. Complete means both extremum gaps meet Tolerance.
type SurfacePlaneExtrema struct {
	Minimum, Maximum               CertifiedScalarRange
	MinimumWitness, MaximumWitness Vec2
	StrictSign                     int
	Complete                       bool
	Cells, ExactOperations         int
}

// BoundSurfacePlaneRange returns rigorous global plane-functional extrema.
func BoundSurfacePlaneRange(p *SurfacePatch, plane Plane, o SurfacePlaneRangeOptions) (SurfacePlaneExtrema, error) {
	return BoundSurfacePlaneRangeContext(context.Background(), p, plane, o)
}

// BoundSurfacePlaneRangeContext checks cancellation between bounded cell steps.
func BoundSurfacePlaneRangeContext(ctx context.Context, p *SurfacePatch, plane Plane, o SurfacePlaneRangeOptions) (r SurfacePlaneExtrema, err error) {
	return boundSurfacePlaneRangeContext(ctx, p, plane, o, false)
}

func boundSurfacePlaneRangeContext(ctx context.Context, p *SurfacePatch, plane Plane, o SurfacePlaneRangeOptions, curveOnly bool) (r SurfacePlaneExtrema, err error) {
	r.Minimum = CertifiedScalarRange{Min: math.Inf(-1), Max: math.Inf(1)}
	r.Maximum = r.Minimum
	if ctx == nil || p == nil || len(p.net) == 0 || !plane.origin.IsFinite() || !plane.normal.IsFinite() || plane.normal == (Vec3{}) {
		return r, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.Tolerance == 0 {
		o.Tolerance = 1e-5
	}
	if o.MaxDepth == 0 {
		o.MaxDepth = 24
	}
	if o.MaxCells == 0 {
		o.MaxCells = 4096
	}
	if o.MaxOperations == 0 {
		o.MaxOperations = 4000000
	}
	if o.MaxCoefficientBits == 0 {
		o.MaxCoefficientBits = 16384
	}
	if !finite(o.Tolerance) || o.Tolerance <= 0 || o.MaxDepth < 1 || o.MaxDepth > 128 || o.MaxCells < 1 || o.MaxOperations < 1 || o.MaxCoefficientBits < 64 {
		return r, ErrInvalid
	}
	b := &polynomialBudget{maxOperations: o.MaxOperations, maxBits: o.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	eval := func(point [3]*big.Rat) (*big.Rat, error) {
		value := new(big.Rat)
		for axis := 0; axis < 3; axis++ {
			delta := new(big.Rat).Sub(point[axis], new(big.Rat).SetFloat64(coordinate(plane.origin, axis)))
			value.Add(value, new(big.Rat).Mul(delta, new(big.Rat).SetFloat64(coordinate(plane.normal, axis))))
			if e := b.check(value); e != nil {
				return nil, e
			}
		}
		return value, nil
	}
	type cell struct {
		p      *SurfacePatch
		depth  int
		lo, hi *big.Rat
	}
	var minSample, maxSample *big.Rat
	minUV, maxUV := Vec2{}, Vec2{}
	classify := func(p *SurfacePatch, depth int) (cell, error) {
		c := cell{p: p, depth: depth}
		for _, h := range p.net {
			point := [3]*big.Rat{new(big.Rat).Quo(h[0], h[3]), new(big.Rat).Quo(h[1], h[3]), new(big.Rat).Quo(h[2], h[3])}
			value, e := eval(point)
			if e != nil {
				return c, e
			}
			if c.lo == nil || value.Cmp(c.lo) < 0 {
				c.lo = value
			}
			if c.hi == nil || value.Cmp(c.hi) > 0 {
				c.hi = value
			}
		}
		d := p.Domain()
		midpoint := func(a, c float64) float64 {
			x := new(big.Rat).Add(new(big.Rat).SetFloat64(a), new(big.Rat).SetFloat64(c))
			x.Quo(x, big.NewRat(2, 1))
			f, _ := x.Float64()
			return f
		}
		for _, uv := range []Vec2{{d.UMin, d.VMin}, {d.UMax, d.VMax}, {d.UMin, d.VMax}, {d.UMax, d.VMin}, {midpoint(d.UMin, d.UMax), midpoint(d.VMin, d.VMax)}} {
			point, e := exactPatchPointAt(p, uv.X, uv.Y, b)
			if e != nil {
				return c, e
			}
			value, e := eval(point)
			if e != nil {
				return c, e
			}
			if minSample == nil || value.Cmp(minSample) < 0 {
				minSample = value
				minUV = uv
			}
			if maxSample == nil || value.Cmp(maxSample) > 0 {
				maxSample = value
				maxUV = uv
			}
		}
		return c, nil
	}
	c, e := classify(p, 0)
	if e != nil {
		return r, e
	}
	cells := []cell{c}
	tolerance := new(big.Rat).SetFloat64(o.Tolerance)
	for {
		lo, hi := cells[0].lo, cells[0].hi
		for _, c := range cells {
			if c.lo.Cmp(lo) < 0 {
				lo = c.lo
			}
			if c.hi.Cmp(hi) > 0 {
				hi = c.hi
			}
		}
		r.Minimum = certifiedRange(exactInterval{lo, minSample})
		r.Maximum = certifiedRange(exactInterval{maxSample, hi})
		r.MinimumWitness, r.MaximumWitness = minUV, maxUV
		r.Cells = len(cells)
		if lo.Sign() > 0 {
			r.StrictSign = 1
		} else if hi.Sign() < 0 {
			r.StrictSign = -1
		} else {
			r.StrictSign = 0
		}
		if e := ctx.Err(); e != nil {
			return r, e
		}
		if finite(r.Minimum.Min) && finite(r.Minimum.Max) && finite(r.Maximum.Min) && finite(r.Maximum.Max) && new(big.Rat).Sub(new(big.Rat).SetFloat64(r.Minimum.Max), new(big.Rat).SetFloat64(r.Minimum.Min)).Cmp(tolerance) <= 0 && new(big.Rat).Sub(new(big.Rat).SetFloat64(r.Maximum.Max), new(big.Rat).SetFloat64(r.Maximum.Min)).Cmp(tolerance) <= 0 {
			r.Complete = true
			return r, nil
		}
		if len(cells) >= o.MaxCells {
			return r, ErrConvergence
		}
		best := -1
		gap := new(big.Rat)
		for i, c := range cells {
			if c.depth >= o.MaxDepth {
				continue
			}
			g := new(big.Rat).Sub(minSample, c.lo)
			h := new(big.Rat).Sub(c.hi, maxSample)
			if h.Cmp(g) > 0 {
				g = h
			}
			if best < 0 || g.Cmp(gap) > 0 {
				best = i
				gap = g
			}
		}
		if best < 0 {
			return r, ErrConvergence
		}
		c := cells[best]
		t, u, _ := patchSplitChoice(c.p)
		if curveOnly {
			u = true
			if knot, ok := internalPatchKnot(c.p.knotsU, c.p.degreeU, c.p.countU); ok {
				t = knot
			} else {
				d := c.p.Domain()
				t = d.UMin + (d.UMax-d.UMin)/2
			}
		}
		d := c.p.Domain()
		if (u && (t <= d.UMin || t >= d.UMax)) || (!u && (t <= d.VMin || t >= d.VMax)) {
			cells[best].depth = o.MaxDepth
			continue
		}
		a, z, e := c.p.split(t, u, b)
		if e != nil {
			return r, e
		}
		left, e := classify(a, c.depth+1)
		if e != nil {
			return r, e
		}
		right, e := classify(z, c.depth+1)
		if e != nil {
			return r, e
		}
		cells[best] = left
		cells = append(cells, right)
	}
}
