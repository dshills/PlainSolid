package trimmed

import (
	"context"
	"fmt"
	"math/big"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// AlgebraicGraphLineMaterial is the exact retained material on a finite line.
// Parameters are in the caller's original units. Before and After refer to the
// open intervals within the requested range. Singleton contact intervals
// represent isolated boundary points. Hits retain native face and rim owners.
type AlgebraicGraphLineMaterial struct {
	Complete        bool
	Intervals       []geom.ExactParameterInterval
	Hits            []AlgebraicGraphFiberHit
	BoundarySpans   []AlgebraicGraphLineFaceSpan
	ExactOperations int
}

// IntersectObliqueLineRational clips a line with a nonzero projected direction
// against an exact prepared graph common. It proves roots on both original
// graphs, discards buried support contacts using native loop membership, and
// classifies intervals only after all their possible endpoints are accounted
// for. Retained face overlaps are explicit. Nonrational support roots and
// coincidence with a shared rim fail atomically here; use the bounded API
// for isolated algebraic contacts or the normal API for fixed projection.
func (q *AlgebraicGraphMaterialQuery) IntersectObliqueLineRational(ctx context.Context, origin, direction [3]*big.Rat,
	limits [2]*big.Rat, o geom.ToroidLineOptions) (out AlgebraicGraphLineMaterial, err error) {
	bounded, e := q.intersectObliqueLineBounds(ctx, origin, direction, limits, o, false)
	out.ExactOperations, out.Complete, out.BoundarySpans = bounded.ExactOperations, bounded.Complete, bounded.BoundarySpans
	if e != nil {
		return out, e
	}
	for _, i := range bounded.Intervals {
		out.Intervals = append(out.Intervals, geom.ExactParameterInterval{Lower: i.Start.Lower, Upper: i.End.Upper})
	}
	for _, h := range bounded.Hits {
		out.Hits = append(out.Hits, AlgebraicGraphFiberHit{Parameter: h.Parameter.Lower, Point: h.Point,
			Faces: h.Faces, RimEdges: h.RimEdges, Before: h.Before, After: h.After})
	}
	return out, nil
}

// AlgebraicGraphLineBoundary encloses one retained boundary event. Point is
// populated only for an exact rational singleton. Each original coordinate
// otherwise has a closed rational enclosure in PointBounds.
type AlgebraicGraphLineBoundary struct {
	Parameter       geom.ExactParameterInterval
	Root            *geom.ExactAlgebraicRoot // original t, present for nonsingleton contacts
	Point           [3]*big.Rat
	PointBounds     [3]geom.ExactParameterInterval
	Faces, RimEdges []int
	Before, After   bool
}

// AlgebraicGraphLineSpan encloses each exact endpoint independently. The span
// is certified material; uncertainty in one endpoint stays inside its bound.
type AlgebraicGraphLineSpan struct{ Start, End geom.ExactParameterInterval }

// AlgebraicGraphLineFaceSpan is a positive-length boundary overlap on one
// retained original face. Its open interior does not lie on a shared rim.
type AlgebraicGraphLineFaceSpan struct {
	AlgebraicGraphLineSpan
	Face int
}

type AlgebraicGraphLineBounds struct {
	Complete        bool
	Intervals       []AlgebraicGraphLineSpan
	Hits            []AlgebraicGraphLineBoundary
	BoundarySpans   []AlgebraicGraphLineFaceSpan
	ExactOperations int
}

// BoundObliqueLineRational additionally certifies isolated algebraic contacts
// when their brackets are pairwise disjoint and clear of complete rim covers.
// A candidate on a possible rim is accepted only when an exact coincident
// face interval approaches it and one original rim edge can own it.
// Other unresolved contacts fail atomically. Before and After are closed material
// membership of the adjoining open intervals within the requested range.
func (q *AlgebraicGraphMaterialQuery) BoundObliqueLineRational(ctx context.Context, origin, direction [3]*big.Rat,
	limits [2]*big.Rat, o geom.ToroidLineOptions) (AlgebraicGraphLineBounds, error) {
	return q.intersectObliqueLineBounds(ctx, origin, direction, limits, o, true)
}

func (q *AlgebraicGraphMaterialQuery) intersectObliqueLineBounds(ctx context.Context, origin, direction [3]*big.Rat,
	limits [2]*big.Rat, o geom.ToroidLineOptions, allowBrackets bool) (out AlgebraicGraphLineBounds, err error) {
	defer func() {
		if err != nil {
			out = AlgebraicGraphLineBounds{ExactOperations: out.ExactOperations}
		}
	}()
	if ctx == nil || q == nil || q.graphs[0] == nil || !finite(o.ParameterTolerance) || o.ParameterTolerance <= 0 ||
		o.MaxSubdivisions < 0 || o.MaxSubdivisions > 65536 || o.Surface.MaxPoles < 0 || o.Surface.MaxPoles > 65536 {
		return out, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	if o.Surface.MaxOperations == 0 {
		o.Surface.MaxOperations = 64000000
	}
	exact, e := graphMaterialLimits(geom.ExactCurvePlaneEventOptions{MaxOperations: o.Surface.MaxOperations, MaxCoefficientBits: o.Surface.MaxCoefficientBits})
	if e != nil {
		return out, e
	}
	check := func(v *big.Rat) error {
		if v == nil {
			return geom.ErrInvalid
		}
		out.ExactOperations++
		if out.ExactOperations > exact.MaxOperations || v.Num().BitLen() > exact.MaxCoefficientBits || v.Denom().BitLen() > exact.MaxCoefficientBits {
			return geom.ErrConvergence
		}
		return ctx.Err()
	}
	remaining := func() int { return exact.MaxOperations - out.ExactOperations }
	for _, vals := range [][]*big.Rat{origin[:], direction[:], limits[:]} {
		for _, v := range vals {
			if e := check(v); e != nil {
				return out, e
			}
		}
	}
	if limits[0].Cmp(limits[1]) > 0 || direction[q.axes[0]].Sign() == 0 && direction[q.axes[1]].Sign() == 0 {
		return out, geom.ErrInvalid
	}
	pointAt := func(t *big.Rat) (p [3]*big.Rat, err error) {
		for i := range p {
			p[i] = new(big.Rat).Mul(t, direction[i])
			if e := check(p[i]); e != nil {
				return p, e
			}
			p[i].Add(p[i], origin[i])
			if e := check(p[i]); e != nil {
				return p, e
			}
		}
		return p, nil
	}
	locate := func(t *big.Rat) (p [3]*big.Rat, loc AlgebraicGraphMaterialPoint, err error) {
		p, err = pointAt(t)
		if err != nil {
			return p, loc, err
		}
		o := exact
		o.MaxOperations = remaining()
		if o.MaxOperations < 1 {
			return p, loc, geom.ErrConvergence
		}
		loc, err = q.LocateRational(ctx, p, o)
		out.ExactOperations += loc.ExactOperations
		if err == nil && !loc.Complete {
			err = geom.ErrConvergence
		}
		return p, loc, err
	}
	type lineEvent struct {
		t     geom.ExactParameterInterval
		root  *geom.ExactAlgebraicRoot
		chart int // chart support for a nonsingleton event
	}
	single := func(t *big.Rat) lineEvent {
		return lineEvent{t: geom.ExactParameterInterval{Lower: new(big.Rat).Set(t), Upper: new(big.Rat).Set(t)}, chart: -1}
	}
	var knots []lineEvent
	var coincidence [2][]geom.ExactParameterInterval
	for chart, g := range q.graphs {
		coef, e := g.InverseCoefficients()
		if e != nil {
			return out, e
		}
		var base, slope [2]*big.Rat
		for i, c := range coef {
			base[i], slope[i] = new(big.Rat).Set(c[2]), new(big.Rat)
			for j, axis := range q.axes {
				base[i].Add(base[i], new(big.Rat).Mul(c[j], origin[axis]))
				slope[i].Add(slope[i], new(big.Rat).Mul(c[j], direction[axis]))
				for _, v := range []*big.Rat{base[i], slope[i]} {
					if e := check(v); e != nil {
						return out, e
					}
				}
			}
		}
		lo, hi := new(big.Rat).Set(limits[0]), new(big.Rat).Set(limits[1])
		d := q.patches[chart].Domain()
		ranges := [2][2]float64{{d.UMin, d.UMax}, {d.VMin, d.VMax}}
		for i, bounds := range ranges {
			a, b := ratInset(bounds[0]), ratInset(bounds[1])
			if slope[i].Sign() == 0 {
				if base[i].Cmp(a) < 0 || base[i].Cmp(b) > 0 {
					lo.SetInt64(1)
					hi.SetInt64(0)
					break
				}
				continue
			}
			a.Quo(a.Sub(a, base[i]), slope[i])
			b.Quo(b.Sub(b, base[i]), slope[i])
			if e := check(a); e != nil {
				return out, e
			}
			if e := check(b); e != nil {
				return out, e
			}
			if a.Cmp(b) > 0 {
				a, b = b, a
			}
			if a.Cmp(lo) > 0 {
				lo.Set(a)
			}
			if b.Cmp(hi) < 0 {
				hi.Set(b)
			}
		}
		if lo.Cmp(hi) > 0 {
			continue
		}
		at := func(t *big.Rat) (uv [2]*big.Rat, err error) {
			for i := range uv {
				uv[i] = new(big.Rat).Add(base[i], new(big.Rat).Mul(t, slope[i]))
				if e := check(uv[i]); e != nil {
					return uv, e
				}
			}
			return uv, nil
		}
		start, e := at(lo)
		if e != nil {
			return out, e
		}
		first, e := pointAt(lo)
		if e != nil {
			return out, e
		}
		if lo.Cmp(hi) == 0 {
			if remaining() < 1 {
				return out, geom.ErrConvergence
			}
			jet, e := g.EvaluateRational(ctx, start, geom.SurfacePatchEvaluationOptions{MaxOperations: remaining(), MaxCoefficientBits: exact.MaxCoefficientBits})
			out.ExactOperations += jet.ExactOperations
			if e != nil {
				return out, e
			}
			if jet.Point[q.residual].Cmp(first[q.residual]) == 0 {
				knots = append(knots, single(lo))
			}
			continue
		}
		end, e := at(hi)
		if e != nil {
			return out, e
		}
		last, e := pointAt(hi)
		if e != nil {
			return out, e
		}
		if remaining() < 1 {
			return out, geom.ErrConvergence
		}
		pieces, e := q.patches[chart].AffineParameterCurvePieces(ctx, start, end, geom.SurfaceAffineCurveOptions{TubeAffineLineOptions: geom.TubeAffineLineOptions{
			MaxOperations: remaining(), MaxCoefficientBits: exact.MaxCoefficientBits, MaxPoles: o.Surface.MaxPoles}})
		out.ExactOperations += pieces.ExactOperations
		if e != nil {
			return out, e
		}
		if !pieces.Complete || !pieces.Continuous || remaining() < 1 {
			return out, geom.ErrConvergence
		}
		width := new(big.Rat).Sub(hi, lo)
		tol := new(big.Rat).Quo(ratInset(o.ParameterTolerance), width)
		line := o
		line.Surface.MaxOperations, line.Surface.MaxCoefficientBits = remaining(), exact.MaxCoefficientBits
		roots, e := geom.IntersectAffineGraphCurveHeightRational(ctx, pieces, q.residual, first[q.residual], last[q.residual], tol, line)
		out.ExactOperations += roots.ExactOperations
		if e != nil {
			return out, e
		}
		mapParameter := func(p *big.Rat) (*big.Rat, error) {
			t := new(big.Rat).Add(lo, new(big.Rat).Mul(width, p))
			return t, check(t)
		}
		for _, span := range roots.Coincident {
			a, e := mapParameter(span.Lower)
			if e != nil {
				return out, e
			}
			b, e := mapParameter(span.Upper)
			if e != nil {
				return out, e
			}
			coincidence[chart] = append(coincidence[chart], geom.ExactParameterInterval{Lower: a, Upper: b})
			knots = append(knots, single(a), single(b))
		}
		for _, hit := range roots.Hits {
			if !allowBrackets && hit.Parameter.Min.Cmp(hit.Parameter.Max) != 0 {
				return out, geom.ErrConvergence
			}
			r := lineEvent{chart: chart}
			for i, p := range []*big.Rat{hit.Parameter.Min, hit.Parameter.Max} {
				t, e := mapParameter(p)
				if e != nil {
					return out, e
				}
				if i == 0 {
					r.t.Lower = t
				} else {
					r.t.Upper = t
				}
			}
			if r.t.Lower.Cmp(r.t.Upper) != 0 {
				options := exact
				options.MaxOperations = remaining()
				if options.MaxOperations < 1 || hit.Root == nil {
					return out, geom.ErrConvergence
				}
				mapped, used, e := hit.Root.AffineMap(ctx, lo, width, options)
				out.ExactOperations += used
				if e != nil {
					return out, e
				}
				r.root = mapped
			}
			knots = append(knots, r)
		}
	}
	knots = append(knots, single(limits[0]), single(limits[1]))
	order := func() { sort.Slice(knots, func(i, j int) bool { return knots[i].t.Lower.Cmp(knots[j].t.Lower) < 0 }) }
	order()
	// The two supports can isolate distinct roots in initially overlapping
	// tolerance brackets. Narrow the original identities instead of merging
	// them or selecting a numerical order.
	for attempt := 0; allowBrackets && attempt < 24; attempt++ {
		narrowed := false
		for i := 1; i < len(knots); i++ {
			a, b := &knots[i-1], &knots[i]
			if a.t.Upper.Cmp(b.t.Lower) < 0 || a.root == nil && b.root == nil {
				continue
			}
			for _, k := range []*lineEvent{a, b} {
				if k.root == nil {
					continue
				}
				width := new(big.Rat).Sub(k.t.Upper, k.t.Lower)
				if width.Sign() == 0 {
					continue
				}
				width.Quo(width, big.NewRat(16, 1))
				if e := check(width); e != nil {
					return out, e
				}
				opts := exact
				opts.MaxOperations, opts.MaxSubdivisions = remaining(), o.MaxSubdivisions
				if opts.MaxOperations < 1 {
					return out, geom.ErrConvergence
				}
				n, e := k.root.Enclose(ctx, width, opts)
				out.ExactOperations += n.Operations
				if e != nil {
					return out, e
				}
				if !n.Converged {
					return out, geom.ErrConvergence
				}
				k.t = n.Interval
				narrowed = true
			}
		}
		if !narrowed {
			break
		}
		order()
	}
	unique := knots[:0]
	for _, t := range knots {
		if len(unique) > 0 {
			p := unique[len(unique)-1].t
			if p.Upper.Cmp(t.t.Lower) >= 0 {
				if p.Lower.Cmp(p.Upper) == 0 && t.t.Lower.Cmp(t.t.Upper) == 0 && p.Upper.Cmp(t.t.Lower) == 0 {
					continue
				}
				return out, geom.ErrConvergence
			}
		}
		unique = append(unique, t)
	}
	inside := make([]bool, len(unique)+1)
	spanFace := make([]int, len(inside)) // native cap face + 1; zero means no coincident cell
	for i := 1; i < len(unique); i++ {
		mid := new(big.Rat).Quo(new(big.Rat).Add(unique[i-1].t.Upper, unique[i].t.Lower), big.NewRat(2, 1))
		if e := check(mid); e != nil {
			return out, e
		}
		_, loc, e := locate(mid)
		if e != nil {
			return out, e
		}
		if loc.Location == parametric.Boundary {
			if len(loc.BoundaryFaces) != 1 || len(loc.RimEdges) != 0 {
				return out, geom.ErrConvergence
			}
			face := loc.BoundaryFaces[0]
			certified := false
			for chart, spans := range coincidence {
				if loc.Faces[chart] != face {
					continue
				}
				for _, span := range spans {
					if span.Lower.Cmp(unique[i-1].t.Upper) <= 0 && span.Upper.Cmp(unique[i].t.Lower) >= 0 {
						certified = true
						break
					}
				}
			}
			if !certified {
				return out, geom.ErrConvergence
			}
			spanFace[i] = face + 1
			start, end := unique[i-1].t, unique[i].t
			out.BoundarySpans = append(out.BoundarySpans, AlgebraicGraphLineFaceSpan{AlgebraicGraphLineSpan{start, end}, face})
		}
		inside[i] = loc.Location != parametric.Outside
	}
	for i, k := range unique {
		t := k.t
		before, after := inside[i], inside[i+1]
		h := AlgebraicGraphLineBoundary{Parameter: t, Root: k.root, Before: before, After: after}
		var location parametric.Location
		if t.Lower.Cmp(t.Upper) == 0 {
			p, loc, e := locate(t.Lower)
			if e != nil {
				return out, e
			}
			location, h.Point, h.Faces, h.RimEdges = loc.Location, p, loc.BoundaryFaces, loc.RimEdges
			// Artificial finite endpoints in the interior of a retained
			// face overlap are represented by the span, not isolated hits.
			if location == parametric.Boundary && len(h.RimEdges) == 0 && (i == 0 || i == len(unique)-1) {
				for _, span := range out.BoundarySpans {
					if span.Start.Lower.Cmp(t.Lower) == 0 || span.End.Upper.Cmp(t.Upper) == 0 {
						location = parametric.Inside
						break
					}
				}
			}

			for j, v := range p {
				h.PointBounds[j] = geom.ExactParameterInterval{Lower: new(big.Rat).Set(v), Upper: new(big.Rat).Set(v)}
			}
		} else {
			lo, e := pointAt(t.Lower)
			if e != nil {
				return out, e
			}
			hi, e := pointAt(t.Upper)
			if e != nil {
				return out, e
			}
			for j := range lo {
				a, b := lo[j], hi[j]
				if a.Cmp(b) > 0 {
					a, b = b, a
				}
				h.PointBounds[j] = geom.ExactParameterInterval{Lower: a, Upper: b}
			}
			// Coincidence against the other support can yield a shared
			// irrational rim. Require a retained open incident interval
			// and unambiguous original native section-edge ancestry.
			var rimCandidates []int
			rimCandidates, e = q.obliqueBracketPossibleRims(ctx, lo, hi, check)
			if e != nil {
				return out, e
			}
			if len(rimCandidates) != 0 {
				if len(rimCandidates) != 1 || spanFace[i] == 0 && spanFace[i+1] == 0 ||
					spanFace[i] != 0 && spanFace[i+1] != 0 && spanFace[i] != spanFace[i+1] {
					return out, fmt.Errorf("graph line bracket lacks a unique retained rim owner: %w", geom.ErrConvergence)
				}
				other := k.chart ^ 1
				coincident := false
				for _, span := range coincidence[other] {
					if span.Lower.Cmp(t.Lower) <= 0 && span.Upper.Cmp(t.Upper) >= 0 {
						coincident = true
						break
					}
				}
				if !coincident {
					return out, geom.ErrConvergence
				}
				face := max(spanFace[i], spanFace[i+1]) - 1
				for _, region := range q.regions {
					if region.faces[other] == face {
						h.Faces = []int{region.faces[0], region.faces[1]}
						break
					}
				}
				if len(h.Faces) != 2 {
					return out, geom.ErrConvergence
				}
				location, h.RimEdges = parametric.Boundary, rimCandidates
			} else {
				// With the complete rim excluded, projected loop membership
				// is constant across this exact graph contact bracket.
				m := new(big.Rat).Quo(new(big.Rat).Add(t.Lower, t.Upper), big.NewRat(2, 1))
				if e := check(m); e != nil {
					return out, e
				}
				point, e := pointAt(m)
				if e != nil {
					return out, e
				}
				opts := exact
				opts.MaxOperations = remaining()
				if opts.MaxOperations < 1 {
					return out, geom.ErrConvergence
				}
				fiber, e := q.locateRationalGraphMaterial(ctx, point, opts, true)
				out.ExactOperations += fiber.ExactOperations
				if e != nil {
					return out, e
				}
				if !fiber.Complete {
					return out, geom.ErrConvergence
				}
				if fiber.Faces[k.chart] >= 0 {
					location, h.Faces = parametric.Boundary, []int{fiber.Faces[k.chart]}
				}
			}
		}
		if location == parametric.Boundary {
			out.Hits = append(out.Hits, h)
		} else if location == parametric.Outside && (before || after) {
			return out, geom.ErrConvergence
		}
		if location != parametric.Outside && !before {
			out.Intervals = append(out.Intervals, AlgebraicGraphLineSpan{Start: t, End: t})
		}
		if before {
			if len(out.Intervals) == 0 {
				return out, geom.ErrConvergence
			}
			out.Intervals[len(out.Intervals)-1].End = t
		}
	}
	if e := ctx.Err(); e != nil {
		return out, e
	}
	out.Complete = true
	return out, nil
}

// Projection is exactly affine in chart 0. The parameter box between its two
// mapped endpoints contains the full projected bracket, regardless of slope.
func (q *AlgebraicGraphMaterialQuery) obliqueBracketPossibleRims(ctx context.Context, endpoints0, endpoints1 [3]*big.Rat, check func(*big.Rat) error) ([]int, error) {
	coeff, e := q.graphs[0].InverseCoefficients()
	if e != nil {
		return nil, e
	}
	var bounds [2][2]*big.Rat
	for i, end := range [2][3]*big.Rat{endpoints0, endpoints1} {
		for j, c := range coeff {
			x := new(big.Rat).Set(c[2])
			for k, axis := range q.axes {
				x.Add(x, new(big.Rat).Mul(c[k], end[axis]))
				if e := check(x); e != nil {
					return nil, e
				}
			}
			bounds[j][i] = x
		}
	}
	for j, b := range bounds {
		if b[0].Cmp(b[1]) > 0 {
			bounds[j][0], bounds[j][1] = b[1], b[0]
		}
	}
	mayTouch := func(cell geom.ParameterRectangle) (bool, error) {
		if e := ctx.Err(); e != nil {
			return false, e
		}
		for _, v := range []*big.Rat{bounds[0][0], bounds[0][1], bounds[1][0], bounds[1][1]} {
			if e := check(v); e != nil {
				return false, e
			}
		}
		return !(bounds[0][1].Cmp(ratInset(cell.UMin)) < 0 || bounds[0][0].Cmp(ratInset(cell.UMax)) > 0 ||
			bounds[1][1].Cmp(ratInset(cell.VMin)) < 0 || bounds[1][0].Cmp(ratInset(cell.VMax)) > 0), nil
	}
	var candidates []int
	for i, loop := range q.loops {
		for j, piece := range loop {
			if len(q.windowTubeBoxes) != 0 {
				for _, cell := range q.windowTubeBoxes[i][j] {
					found, e := mayTouch(cell)
					if e != nil {
						return nil, e
					}
					if found {
						candidates = append(candidates, q.edges[i][j])
						break
					}
				}
				continue
			}
			b, e := piece.Segment.TrimA().BoundsUV()
			if e != nil {
				return nil, e
			}
			found, e := mayTouch(geom.ParameterRectangle{UMin: b.Min.X, UMax: b.Max.X, VMin: b.Min.Y, VMax: b.Max.Y})
			if e != nil {
				return nil, e
			}
			if found {
				candidates = append(candidates, q.edges[i][j])
			}
		}
	}
	return candidates, nil
}
