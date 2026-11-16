package geom

import (
	"context"
	"math/big"
	"sort"
)

// ProjectiveParallelContact retains one exact complete section across
// the shared footprint. Station is measured on Axis in A's normalized
// overlap coordinates, independently of the original UV clock on B.
type ProjectiveParallelContact struct {
	Contour int
	Station *big.Rat
	Pieces  []ProjectiveContactSplineUse
}

// ProjectiveHeightBand is the region between two parallel sections or
// between a section and the footprint boundary. Sign is A minus B
// throughout the open band. Witness retains the two exact original charts.
type ProjectiveHeightBand struct {
	Common  ExactSurfaceRectangle
	Sign    int
	Witness ProjectiveContactHeight
}

// ProjectiveContactBands certifies a complete footprint partition. Its
// bands and sections are ordered by increasing station on Axis.
type ProjectiveContactBands struct {
	Axis            int
	Sections        []ProjectiveParallelContact
	Bands           []ProjectiveHeightBand
	ExactOperations int
	Complete        bool
}

// BuildParallelContactBands proves a strictly ordered family of at least
// two exact straight contours spanning the same opposite footprint sides.
// A complete regular graph without holes and one original witness per
// band prove the sign on every open region. No band is published if a
// contour, original clock or work limit cannot be proved.
func (g ProjectiveSurfaceContactGraph) BuildParallelContactBands(ctx context.Context,
	o SurfacePatchOptions) (r ProjectiveContactBands, err error) {
	if ctx == nil || g.proof == nil {
		return r, ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	p := g.proof
	limits, err := patchOptions(o)
	if err != nil {
		return r, err
	}
	if len(p.contours) < 2 || len(p.contours) > limits.MaxPoles || len(p.holes) != 0 {
		return r, ErrConvergence
	}
	b := &polynomialBudget{maxOperations: limits.MaxOperations, maxBits: limits.MaxCoefficientBits}
	defer func() { r.ExactOperations = b.operations }()
	left := func() (SurfacePatchOptions, error) {
		q := limits
		q.MaxOperations -= b.operations
		if q.MaxOperations < 1 {
			return q, ErrConvergence
		}
		return q, nil
	}
	points := make(map[int][2]*big.Rat)
	end := func(use ProjectiveContactSplineUse, at int) (q [2]*big.Rat, err error) {
		id := use.Vertices[at]
		if q, ok := points[id]; ok {
			return q, nil
		}
		if use.Reversed {
			at = 1 - at
		}
		for axis, x := range p.arcs[use.Arc].ends[at] {
			q[axis], err = projectiveSplineRationalEndpoint(ctx, x.root, b)
			if err != nil {
				return q, err
			}
		}
		points[id] = q
		return q, nil
	}
	zero, one := new(big.Rat), big.NewRat(1, 1)
	fixed := -1
	var sections []ProjectiveParallelContact
	for i, contour := range p.contours {
		if contour.Closed {
			return r, ErrConvergence
		}
		o, err := left()
		if err != nil {
			return r, err
		}
		s, err := g.BuildContourSplines(ctx, i, o)
		b.operations += s.ExactOperations
		if err != nil {
			return r, err
		}
		a, err := end(s.Uses[0], 0)
		if err != nil {
			return r, err
		}
		c, err := end(s.Uses[len(s.Uses)-1], 1)
		if err != nil {
			return r, err
		}
		axis := 0
		if a[0].Cmp(c[0]) != 0 {
			axis = 1
		}
		f := 1 - axis
		up := a[f].Sign() == 0 && c[f].Cmp(one) == 0
		down := a[f].Cmp(one) == 0 && c[f].Sign() == 0
		if a[axis].Cmp(c[axis]) != 0 || a[axis].Sign() <= 0 || a[axis].Cmp(one) >= 0 ||
			fixed != -1 && axis != fixed || !up && !down {
			return r, ErrConvergence
		}
		fixed = axis
		for _, use := range s.Uses {
			x, err := end(use, 0)
			if err != nil {
				return r, err
			}
			y, err := end(use, 1)
			if err != nil {
				return r, err
			}
			if x[axis].Cmp(a[axis]) != 0 || y[axis].Cmp(a[axis]) != 0 ||
				up && x[f].Cmp(y[f]) >= 0 || down && x[f].Cmp(y[f]) <= 0 {
				return r, ErrConvergence
			}
		}
		sections = append(sections, ProjectiveParallelContact{i, new(big.Rat).Set(a[axis]), s.Uses})
	}
	sort.Slice(sections, func(i, j int) bool { return sections[i].Station.Cmp(sections[j].Station) < 0 })
	var bands []ProjectiveHeightBand
	minimum := zero
	arithmetic := boundedRatArithmetic{b: b}
	for i := 0; i <= len(sections); i++ {
		maximum := one
		if i < len(sections) {
			maximum = sections[i].Station
		}
		if minimum.Cmp(maximum) >= 0 {
			return r, ErrConvergence
		}
		seed := [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}
		seed[fixed] = arithmetic.mul(arithmetic.add(minimum, maximum), big.NewRat(1, 2))
		if arithmetic.err != nil {
			return r, arithmetic.err
		}
		o, err := left()
		if err != nil {
			return r, err
		}
		h, err := g.ClassifyCommonHeightRational(ctx, seed,
			SurfacePatchEvaluationOptions{MaxOperations: o.MaxOperations, MaxCoefficientBits: o.MaxCoefficientBits})
		b.operations += h.ExactOperations
		if err != nil {
			return r, err
		}
		if h.Sign == 0 || i > 0 && h.Sign == bands[i-1].Sign {
			return r, ErrConvergence
		}
		span := [2]ExactParameterInterval{}
		span[1-fixed] = ExactParameterInterval{new(big.Rat), big.NewRat(1, 1)}
		span[fixed] = ExactParameterInterval{new(big.Rat).Set(minimum), new(big.Rat).Set(maximum)}
		bands = append(bands, ProjectiveHeightBand{ExactSurfaceRectangle{span[0], span[1]}, h.Sign, h})
		minimum = maximum
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	r.Axis, r.Sections, r.Bands, r.Complete = fixed, sections, bands, true
	return r, nil
}
