package trimmed

import (
	"context"
	"math/big"

	"plainsolid/geom"
)

type AlgebraicTubeLensPlaneRegion struct {
	Parameter  [2]*geom.ExactAlgebraicRoot          // original projected line units
	Curves     [2]geom.SurfaceCurveCompositionCells // original chart indices
	UpperChart int
	Clipped    [2]bool // requested endpoint is strictly interior, not a rim
}

type AlgebraicTubeLensPlaneSection struct {
	Regions             []AlgebraicTubeLensPlaneRegion
	Isolated            []*geom.ExactAlgebraicRoot
	VerticalAtSingleton *AlgebraicTubeLensVerticalInterval
	ExactOperations     int
	Complete            bool
}

// SectionVerticalPlane intersects the exact lens with the vertical plane
// over XY=origin+t*direction, clipped to the requested rational t range.
// Each region retains exact restrictions on both original surfaces and
// shares its original algebraic parameter at each rim.
func (l AlgebraicTubeLens) SectionVerticalPlane(ctx context.Context, origin, direction [2]*big.Rat, limits [2]*big.Rat,
	o AlgebraicLensLineOptions) (AlgebraicTubeLensPlaneSection, error) {
	return l.queryData().sectionProjectionPlane(ctx, origin, direction, limits, l.VerticalInterval, o)
}

// SectionProjectionPlane takes a line in ProjectionAxes order and extends it
// along ResidualAxis. The result keeps both exact boundary curves in original
// XYZ coordinates and their common line parameter in the requested units.
func (l AlgebraicProjectedTubeLens) SectionProjectionPlane(ctx context.Context, origin, direction [2]*big.Rat, limits [2]*big.Rat,
	o AlgebraicLensLineOptions) (AlgebraicTubeLensPlaneSection, error) {
	return l.queryData().sectionProjectionPlane(ctx, origin, direction, limits, l.ProjectionInterval, o)
}

func (l tubeLensQuery) sectionProjectionPlane(ctx context.Context, origin, direction [2]*big.Rat, limits [2]*big.Rat,
	interval func(context.Context, [2]*big.Rat, geom.ExactCurvePlaneEventOptions) (AlgebraicTubeLensVerticalInterval, error),
	o AlgebraicLensLineOptions) (out AlgebraicTubeLensPlaneSection, err error) {
	source := l.section.runSource()
	if ctx == nil || source == nil || origin[0] == nil || origin[1] == nil || direction[0] == nil || direction[1] == nil ||
		direction[0].Sign() == 0 && direction[1].Sign() == 0 || limits[0] == nil || limits[1] == nil ||
		limits[0].Cmp(limits[1]) > 0 || o.MaxEvents < 0 || o.MaxEvents > 4096 {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	defer func() {
		if err != nil {
			out.Regions = nil
			out.Isolated = nil
		}
	}()
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
		return out, geom.ErrInvalid
	}
	for _, v := range []*big.Rat{origin[0], origin[1], direction[0], direction[1], limits[0], limits[1]} {
		if v.Num().BitLen() > o.Roots.MaxCoefficientBits || v.Denom().BitLen() > o.Roots.MaxCoefficientBits {
			return out, geom.ErrConvergence
		}
	}
	exact := func() geom.ExactCurvePlaneEventOptions {
		p := o.Roots
		p.MaxOperations -= out.ExactOperations
		return p
	}
	patch := func() geom.SurfacePatchOptions {
		p := o.Composition.SurfacePatchOptions
		q := exact()
		p.MaxOperations = capPositiveBudget(p.MaxOperations, q.MaxOperations)
		p.MaxCoefficientBits = capPositiveBudget(p.MaxCoefficientBits, q.MaxCoefficientBits)
		return p
	}
	root := func(t *big.Rat) (*geom.ExactAlgebraicRoot, error) {
		p := exact()
		if p.MaxOperations < 1 {
			return nil, geom.ErrConvergence
		}
		r, n, e := geom.NewExactRationalRoot(ctx, t, p)
		out.ExactOperations += n
		return r, e
	}
	compare := func(a, b *geom.ExactAlgebraicRoot) (geom.ExactCurveEventOrder, error) {
		p := exact()
		if p.MaxOperations < 1 {
			return geom.ExactCurveEventOrder{}, geom.ErrConvergence
		}
		r, e := a.Compare(ctx, b, p)
		out.ExactOperations += r.Operations
		if e == nil && !r.Proven {
			e = geom.ErrConvergence
		}
		return r, e
	}
	var uv, duv [2][2]*big.Rat
	lo, hi := new(big.Rat).Set(limits[0]), new(big.Rat).Set(limits[1])
	for i, f := range source.faces {
		m, e := l.graphs[i].InverseCoefficients()
		if e != nil {
			return out, e
		}
		for j := range m {
			uv[i][j] = new(big.Rat).Add(m[j][2], new(big.Rat).Add(new(big.Rat).Mul(m[j][0], origin[0]), new(big.Rat).Mul(m[j][1], origin[1])))
			duv[i][j] = new(big.Rat).Add(new(big.Rat).Mul(m[j][0], direction[0]), new(big.Rat).Mul(m[j][1], direction[1]))
		}
		var ok bool
		lo, hi, ok = clipTubeConvexFaceLine(f, uv[i], duv[i], lo, hi)
		if !ok {
			out.Complete = true
			return out, nil
		}
	}
	location := func(t *big.Rat) (AlgebraicTubeLensVerticalInterval, error) {
		p := exact()
		if p.MaxOperations < 1 {
			return AlgebraicTubeLensVerticalInterval{}, geom.ErrConvergence
		}
		v := [2]*big.Rat{new(big.Rat).Add(origin[0], new(big.Rat).Mul(direction[0], t)), new(big.Rat).Add(origin[1], new(big.Rat).Mul(direction[1], t))}
		x, e := interval(ctx, v, p)
		out.ExactOperations += x.ExactOperations
		return x, e
	}
	start, e := root(lo)
	if e != nil {
		return out, e
	}
	if lo.Cmp(hi) == 0 {
		x, e := location(lo)
		if e != nil {
			return out, e
		}
		if x.Rim {
			out.Isolated = append(out.Isolated, start)
		} else if x.Present {
			out.VerticalAtSingleton = &x
		}
		out.Complete = true
		return out, nil
	}
	end, e := root(hi)
	if e != nil {
		return out, e
	}
	var support [2]*geom.SurfacePatch
	for i, f := range source.faces {
		p := patch()
		if p.MaxOperations < 1 {
			return out, geom.ErrConvergence
		}
		var used int
		support[i], used, e = exactLensQueryPatch(f, p)
		out.ExactOperations += used
		if e != nil {
			return out, e
		}
	}
	build := func(chart int, a, b *geom.ExactAlgebraicRoot) (geom.SurfaceCurveCompositionCells, error) {
		p := o.Composition
		p.SurfacePatchOptions = patch()
		if p.MaxOperations < 1 {
			return geom.SurfaceCurveCompositionCells{}, geom.ErrConvergence
		}
		r, e := support[chart].BuildAffineSection(ctx, uv[chart], duv[chart], a, b, p)
		out.ExactOperations += r.ExactOperations
		if e == nil && !r.Complete {
			e = geom.ErrConvergence
		}
		return r, e
	}
	section, e := build(0, start, end)
	if e != nil {
		return out, e
	}
	type node struct {
		r   *geom.ExactAlgebraicRoot
		rim bool
	}
	nodes := []node{{start, false}, {end, false}}
	for _, c := range section.Cells {
		p := o.Composition
		p.SurfacePatchOptions = patch()
		if p.MaxOperations < 1 {
			return out, geom.ErrConvergence
		}
		x, e := geom.IntersectExactComposedCurveProjectedGraph(ctx, c, support[1], l.axes, p)
		out.ExactOperations += x.ExactOperations
		if e != nil {
			return out, e
		}
		if !x.Complete {
			return out, geom.ErrConvergence
		}
		for _, s := range x.Cells {
			if s.Intersection.Coincident {
				return out, geom.ErrConvergence
			}
			for _, ev := range s.Intersection.Events {
				r := ev.Root()
				i := 0
				found := false
				for ; i < len(nodes); i++ {
					q, e := compare(r, nodes[i].r)
					if e != nil {
						return out, e
					}
					if q.Order == 0 {
						nodes[i].rim = true
						found = true
						break
					}
					if q.Order < 0 {
						break
					}
				}
				if found {
					continue
				}
				if i == 0 || i == len(nodes) || len(nodes) >= o.MaxEvents {
					return out, geom.ErrConvergence
				}
				nodes = append(nodes, node{})
				copy(nodes[i+1:], nodes[i:])
				nodes[i] = node{r, true}
			}
		}
	}
	for i, t := range []*big.Rat{lo, hi} {
		x, e := location(t)
		if e != nil {
			return out, e
		}
		j := i * (len(nodes) - 1)
		nodes[j].rim = nodes[j].rim || x.Rim
	}
	keep := make([]bool, len(nodes)-1)
	for i := range keep {
		q, e := compare(nodes[i].r, nodes[i+1].r)
		if e != nil {
			return out, e
		}
		if q.Order != -1 || q.Intervals[0][1].Cmp(q.Intervals[1][0]) >= 0 {
			return out, geom.ErrConvergence
		}
		t := new(big.Rat).Quo(new(big.Rat).Add(q.Intervals[0][1], q.Intervals[1][0]), big.NewRat(2, 1))
		x, e := location(t)
		if e != nil {
			return out, e
		}
		if !x.Complete || x.Rim {
			return out, geom.ErrConvergence
		}
		keep[i] = x.Present
	}
	for i := 0; i < len(keep); {
		if !keep[i] {
			i++
			continue
		}
		j := i + 1
		for j < len(keep) && keep[j] {
			j++
		}
		x := AlgebraicTubeLensPlaneRegion{Parameter: [2]*geom.ExactAlgebraicRoot{nodes[i].r, nodes[j].r}, UpperChart: l.upper, Clipped: [2]bool{!nodes[i].rim, !nodes[j].rim}}
		for chart := range x.Curves {
			x.Curves[chart], e = build(chart, nodes[i].r, nodes[j].r)
			if e != nil {
				return out, e
			}
		}
		out.Regions = append(out.Regions, x)
		i = j
	}
	for i, x := range nodes {
		if x.rim && !(i > 0 && keep[i-1]) && !(i < len(keep) && keep[i]) {
			out.Isolated = append(out.Isolated, x.r)
		}
	}
	out.Complete = true
	return out, nil
}

func clipTubeConvexFaceLine(f Face, a, b [2]*big.Rat, lo, hi *big.Rat) (*big.Rat, *big.Rat, bool) {
	for _, u := range f.Outer {
		x, y := ratInset(u.UVStart.X), ratInset(u.UVStart.Y)
		dx, dy := new(big.Rat).Sub(ratInset(u.UVEnd.X), x), new(big.Rat).Sub(ratInset(u.UVEnd.Y), y)
		c := new(big.Rat).Sub(new(big.Rat).Mul(dx, new(big.Rat).Sub(a[1], y)), new(big.Rat).Mul(dy, new(big.Rat).Sub(a[0], x)))
		s := new(big.Rat).Sub(new(big.Rat).Mul(dx, b[1]), new(big.Rat).Mul(dy, b[0]))
		if s.Sign() == 0 {
			if c.Sign() < 0 {
				return nil, nil, false
			}
			continue
		}
		t := new(big.Rat).Quo(new(big.Rat).Neg(c), s)
		if s.Sign() > 0 {
			lo = maxRat(lo, t)
		} else {
			hi = minRat(hi, t)
		}
		if lo.Cmp(hi) > 0 {
			return nil, nil, false
		}
	}
	return lo, hi, true
}
