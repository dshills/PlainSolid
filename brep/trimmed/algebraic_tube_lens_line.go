package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

type AlgebraicTubeLensLineEvent struct {
	Parameter    *geom.ExactAlgebraicRoot
	BoundaryMask uint8 // upper=1, lower=2, rim=4, requested endpoint=8
}

type AlgebraicTubeLensLineInterval struct {
	Start, End   AlgebraicTubeLensLineEvent
	BoundaryMask uint8 // original support owners throughout this interval
}

type AlgebraicTubeLensLineSection struct {
	Intervals       []AlgebraicTubeLensLineInterval
	Isolated        []AlgebraicTubeLensLineEvent
	ExactOperations int
	Complete        bool
}

type tubeLensQueryGraph interface {
	InverseCoefficients() ([2][3]*big.Rat, error)
	InvertRational(context.Context, [2]*big.Rat, geom.SurfacePatchEvaluationOptions) (geom.AffineXYLocation, error)
	EvaluateRational(context.Context, [2]*big.Rat, geom.SurfacePatchEvaluationOptions) (geom.RationalSurfacePatchJet, error)
}

type tubeLensQuery struct {
	section AlgebraicTrimmedPairSection
	faces   [2]AlgebraicTubeFacePartition
	graphs  [2]tubeLensQueryGraph
	axes    [2]int
	upper   int
}

func (l AlgebraicTubeLens) queryData() tubeLensQuery {
	return tubeLensQuery{l.section, l.faces, [2]tubeLensQueryGraph{l.graphs[0], l.graphs[1]}, [2]int{0, 1}, l.upper}
}

func (l AlgebraicProjectedTubeLens) queryData() tubeLensQuery {
	return tubeLensQuery{l.section, l.faces, [2]tubeLensQueryGraph{l.graphs[0], l.graphs[1]}, l.axes, l.upper}
}

// ClipMaterialLine returns the closed intersection of a finite model line
// and the algebraic tube lens in original line parameter units. Exact graph
// contact and rational material witnesses determine incidence, including
// intervals lying on either original graph support.
func (l AlgebraicTubeLens) ClipMaterialLine(ctx context.Context, origin, direction geom.Vec3, limits [2]*big.Rat,
	o AlgebraicLensLineOptions) (AlgebraicTubeLensLineSection, error) {
	return l.queryData().clipMaterialLine(ctx, origin, direction, limits, o)
}

// ClipMaterialLine intersects a finite original XYZ line with this lens.
// Boundary masks refer to the upper and lower supports along ResidualAxis.
func (l AlgebraicProjectedTubeLens) ClipMaterialLine(ctx context.Context, origin, direction geom.Vec3, limits [2]*big.Rat,
	o AlgebraicLensLineOptions) (AlgebraicTubeLensLineSection, error) {
	return l.queryData().clipMaterialLine(ctx, origin, direction, limits, o)
}

func (l tubeLensQuery) clipMaterialLine(ctx context.Context, origin, direction geom.Vec3, limits [2]*big.Rat,
	o AlgebraicLensLineOptions) (r AlgebraicTubeLensLineSection, err error) {
	if ctx == nil || l.section.partitionSource == nil || !origin.IsFinite() || !direction.IsFinite() || direction == (geom.Vec3{}) ||
		limits[0] == nil || limits[1] == nil || limits[0].Cmp(limits[1]) > 0 ||
		o.MaxEvents < 0 || o.MaxEvents > 4096 {
		return r, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return r, e
	}
	if o.Roots.MaxOperations == 0 {
		o.Roots.MaxOperations = 32000000
	}
	if o.Roots.MaxCoefficientBits == 0 {
		o.Roots.MaxCoefficientBits = 16384
	}
	if o.MaxEvents == 0 {
		o.MaxEvents = 256
	}
	if o.Roots.MaxOperations < 1 || o.Roots.MaxCoefficientBits < 64 || o.Roots.MaxSubdivisions < 0 || o.MaxEvents < 2 {
		return r, geom.ErrInvalid
	}
	for _, q := range limits {
		if q.Num().BitLen() > o.Roots.MaxCoefficientBits || q.Denom().BitLen() > o.Roots.MaxCoefficientBits {
			return r, geom.ErrConvergence
		}
	}
	exact := func() geom.ExactCurvePlaneEventOptions {
		q := o.Roots
		q.MaxOperations -= r.ExactOperations
		return q
	}
	surface := func() geom.SurfacePatchOptions {
		q := o.Composition.SurfacePatchOptions
		p := exact()
		if q.MaxOperations == 0 || q.MaxOperations > p.MaxOperations {
			q.MaxOperations = p.MaxOperations
		}
		if q.MaxCoefficientBits == 0 || q.MaxCoefficientBits > p.MaxCoefficientBits {
			q.MaxCoefficientBits = p.MaxCoefficientBits
		}
		return q
	}
	compare := func(a, b *geom.ExactAlgebraicRoot) (geom.ExactCurveEventOrder, error) {
		q := exact()
		if q.MaxOperations < 1 {
			return geom.ExactCurveEventOrder{}, geom.ErrConvergence
		}
		c, e := a.Compare(ctx, b, q)
		r.ExactOperations += c.Operations
		if e == nil && !c.Proven {
			e = geom.ErrConvergence
		}
		return c, e
	}
	makeRoot := func(t *big.Rat) (*geom.ExactAlgebraicRoot, error) {
		q := exact()
		if q.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		root, used, e := geom.NewExactRationalRoot(ctx, t, q)
		r.ExactOperations += used
		return root, e
	}
	x, d := [3]*big.Rat{ratInset(origin.X), ratInset(origin.Y), ratInset(origin.Z)}, [3]*big.Rat{ratInset(direction.X), ratInset(direction.Y), ratInset(direction.Z)}
	axis, other := l.axes[0], l.axes[1]
	residual := 3 - axis - other
	at := func(t *big.Rat) (p [3]*big.Rat) {
		for i := range p {
			p[i] = new(big.Rat).Add(x[i], new(big.Rat).Mul(d[i], t))
		}
		return p
	}
	// Clip the query against both convex source trims using exact
	// linear inequalities in original line units. The entire lens
	// lies in their projected intersection.
	lo, hi := new(big.Rat).Set(limits[0]), new(big.Rat).Set(limits[1])
	for chart, f := range l.section.partitionSource.faces {
		m, e := l.graphs[chart].InverseCoefficients()
		if e != nil {
			return r, e
		}
		var a, b [2]*big.Rat
		for i := range a {
			a[i] = new(big.Rat).Add(m[i][2], new(big.Rat).Add(new(big.Rat).Mul(m[i][0], x[axis]), new(big.Rat).Mul(m[i][1], x[other])))
			b[i] = new(big.Rat).Add(new(big.Rat).Mul(m[i][0], d[axis]), new(big.Rat).Mul(m[i][1], d[other]))
		}
		var ok bool
		lo, hi, ok = clipTubeConvexFaceLine(f, a, b, lo, hi)
		if !ok {
			r.Complete = true
			return r, nil
		}
	}
	signs := func(t *big.Rat) (out [2]int, err error) {
		p := at(t)
		child := func() geom.SurfacePatchEvaluationOptions {
			q := exact()
			return geom.SurfacePatchEvaluationOptions{MaxOperations: q.MaxOperations, MaxCoefficientBits: q.MaxCoefficientBits}
		}
		for i, g := range l.graphs {
			q := child()
			if q.MaxOperations < 1 {
				return out, geom.ErrConvergence
			}
			x, e := g.InvertRational(ctx, [2]*big.Rat{p[axis], p[other]}, q)
			r.ExactOperations += x.Operations
			if e != nil {
				return out, e
			}
			if !x.Inside {
				return out, geom.ErrConvergence
			}
			q = child()
			if q.MaxOperations < 1 {
				return out, geom.ErrConvergence
			}
			z, e := g.EvaluateRational(ctx, x.UV, q)
			r.ExactOperations += z.ExactOperations
			if e != nil {
				return out, e
			}
			if i == l.upper {
				out[0] = z.Point[residual].Cmp(p[residual])
			} else {
				out[1] = p[residual].Cmp(z.Point[residual])
			}
		}
		return out, nil
	}
	maskOf := func(s [2]int) uint8 {
		var mask uint8
		if s[0] == 0 {
			mask |= 1
		}
		if s[1] == 0 {
			mask |= 2
		}
		if mask == 3 {
			mask |= 4
		}
		return mask
	}
	endMask := func(t *big.Rat) uint8 {
		if t.Cmp(limits[0]) == 0 || t.Cmp(limits[1]) == 0 {
			return 8
		}
		return 0
	}
	a, e := makeRoot(lo)
	if e != nil {
		return r, e
	}
	if lo.Cmp(hi) == 0 {
		s, e := signs(lo)
		if e != nil {
			return r, e
		}
		if s[0] >= 0 && s[1] >= 0 {
			r.Isolated = []AlgebraicTubeLensLineEvent{{a, maskOf(s) | endMask(lo)}}
		}
		r.Complete = true
		return r, nil
	}
	z, e := makeRoot(hi)
	if e != nil {
		return r, e
	}
	nodes := []AlgebraicTubeLensLineEvent{{a, endMask(lo)}, {z, endMask(hi)}}
	p, q := at(lo), at(hi)
	if surface().MaxOperations < 1 {
		return r, geom.ErrConvergence
	}
	c, e := geom.NewExactHomogeneousBezier(ctx, [][4]*big.Rat{{p[0], p[1], p[2], big.NewRat(1, 1)},
		{q[0], q[1], q[2], big.NewRat(1, 1)}}, surface())
	if e != nil {
		return r, e
	}
	r.ExactOperations += c.ExactOperations()
	if surface().MaxOperations < 1 {
		return r, geom.ErrConvergence
	}
	cell, e := geom.NewExactComposedBezierCell(ctx, c, surface())
	if e != nil {
		return r, e
	}
	width := new(big.Rat).Sub(hi, lo)
	for chart, f := range l.faces {
		options := o.Composition
		options.SurfacePatchOptions = surface()
		if options.MaxOperations < 1 {
			return r, geom.ErrConvergence
		}
		s, e := geom.NewSurfacePatch(f.Support, options.SurfacePatchOptions)
		if e != nil {
			return r, e
		}
		r.ExactOperations += s.ExactOperations()
		options.SurfacePatchOptions = surface()
		if options.MaxOperations < 1 {
			return r, geom.ErrConvergence
		}
		h, e := geom.IntersectExactComposedCurveProjectedGraph(ctx, cell, s, l.axes, options)
		r.ExactOperations += h.ExactOperations
		if e != nil {
			return r, e
		}
		if !h.Complete {
			return r, geom.ErrConvergence
		}
		mask := uint8(2)
		if chart == l.upper {
			mask = 1
		}
		for _, span := range h.Cells {
			events := span.Intersection.Events
			if span.Intersection.Coincident {
				events = []*geom.ExactComposedCurveEvent{span.Intersection.Boundary[0], span.Intersection.Boundary[1]}
			}
			for _, hit := range events {
				if hit == nil {
					return r, geom.ErrConvergence
				}
				opts := exact()
				if opts.MaxOperations < 1 {
					return r, geom.ErrConvergence
				}
				root, used, e := hit.Root().AffineMap(ctx, lo, width, opts)
				r.ExactOperations += used
				if e != nil {
					return r, e
				}
				i := 0
				equal := false
				for ; i < len(nodes); i++ {
					c, e := compare(root, nodes[i].Parameter)
					if e != nil {
						return r, e
					}
					if c.Order == 0 {
						nodes[i].BoundaryMask |= mask
						equal = true
						break
					}
					if c.Order < 0 {
						break
					}
				}
				if equal {
					continue
				}
				if i == 0 || i == len(nodes) || len(nodes) >= o.MaxEvents {
					return r, geom.ErrConvergence
				}
				nodes = append(nodes, AlgebraicTubeLensLineEvent{})
				copy(nodes[i+1:], nodes[i:])
				nodes[i] = AlgebraicTubeLensLineEvent{root, mask}
			}
		}
	}
	var endpointSigns [2][2]int
	for i, t := range []*big.Rat{lo, hi} {
		k := 0
		if i == 1 {
			k = len(nodes) - 1
		}
		s, e := signs(t)
		if e != nil {
			return r, e
		}
		endpointSigns[i] = s
		nodes[k].BoundaryMask |= maskOf(s)
	}
	retained := make([]bool, len(nodes)-1)
	intervalSigns := make([][2]int, len(retained))
	intervalMasks := make([]uint8, len(retained))
	for i := range retained {
		q, e := compare(nodes[i].Parameter, nodes[i+1].Parameter)
		if e != nil {
			return r, e
		}
		if q.Order != -1 || q.Intervals[0][1].Cmp(q.Intervals[1][0]) >= 0 {
			return r, geom.ErrConvergence
		}
		m := new(big.Rat).Quo(new(big.Rat).Add(q.Intervals[0][1], q.Intervals[1][0]), big.NewRat(2, 1))
		s, e := signs(m)
		if e != nil {
			return r, e
		}
		intervalSigns[i] = s
		intervalMasks[i] = maskOf(s)
		retained[i] = s[0] >= 0 && s[1] >= 0
		// A zero interval residual is an exact coincident span. Its
		// support also owns both limits by continuity, including a
		// shared rim root from the opposite support.
		nodes[i].BoundaryMask |= intervalMasks[i]
		nodes[i+1].BoundaryMask |= intervalMasks[i]
	}
	for i := range nodes {
		if nodes[i].BoundaryMask&3 == 3 {
			nodes[i].BoundaryMask |= 4
		}
	}
	for i := 0; i < len(retained); {
		if !retained[i] {
			i++
			continue
		}
		j := i + 1
		for j < len(retained) && retained[j] && intervalMasks[j] == intervalMasks[i] {
			j++
		}
		r.Intervals = append(r.Intervals, AlgebraicTubeLensLineInterval{nodes[i], nodes[j], intervalMasks[i]})
		i = j
	}
	for i, n := range nodes {
		if i > 0 && retained[i-1] || i < len(retained) && retained[i] {
			continue
		}
		s := endpointSigns[0]
		if i == len(nodes)-1 {
			s = endpointSigns[1]
		} else if i > 0 {
			s = intervalSigns[i-1]
			for j := range s {
				if n.BoundaryMask&(1<<j) != 0 {
					s[j] = 0
				} else if s[j] != intervalSigns[i][j] {
					return r, geom.ErrConvergence
				}
			}
		}
		if s[0] >= 0 && s[1] >= 0 {
			r.Isolated = append(r.Isolated, n)
		}
	}
	r.Complete = true
	return r, nil
}
