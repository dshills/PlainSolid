package trimmed

import (
	"context"
	"errors"
	"math/big"
	"sort"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// SharedBaseGraphEnvelopeHit is a proved envelope contact in original line
// parameter. Sides use the unit-chart boundary order bottom, right, top,
// left. TopFaces identify the original graphs (A=0, B=1).
type SharedBaseGraphEnvelopeHit struct {
	Parameter geom.ToroidLineRoot
	Base      bool
	TopFaces  []int
	Sides     []int
}

// IntersectEnvelopeLine proves all isolated boundary contacts of a finite
// nonvertical line with a shared-base union or intersection. Original graph
// roots are tested against the other top's exact height, then classified
// with unit-domain wall contacts. A line in a side wall, coincident support
// intervals, and unresolved overlapping algebraic brackets return
// ErrConvergence when bounded exact root comparison cannot resolve them.
// Use IntersectEnvelopeVerticalLine for vertical queries.
func (p SharedBaseSplineGraphMaterial) IntersectEnvelopeLine(ctx context.Context, origin, direction geom.Vec3,
	minT, maxT float64, op parametric.BooleanOp, o geom.ToroidLineOptions) ([]SharedBaseGraphEnvelopeHit, error) {
	return p.intersectEnvelopeLine(ctx, origin, direction, minT, maxT, op, o, false)
}

func (p SharedBaseSplineGraphMaterial) intersectEnvelopeLine(ctx context.Context, origin, direction geom.Vec3,
	minT, maxT float64, op parametric.BooleanOp, o geom.ToroidLineOptions, allowSide bool) ([]SharedBaseGraphEnvelopeHit, error) {
	if direction.X == 0 && direction.Y == 0 || op != parametric.Union && op != parametric.Intersection {
		return nil, geom.ErrInvalid
	}
	r, e := p.SectionOriginalGraphsLine(ctx, origin, direction, minT, maxT, o)
	if e != nil {
		return nil, e
	}
	if r.Clip == nil {
		return nil, nil
	}
	side := graphEnvelopeLineSide(origin, direction)
	if side >= 0 && !allowSide {
		return nil, geom.ErrConvergence
	}
	for _, x := range r.Graph {
		if len(x.Coincident) > 0 {
			return nil, geom.ErrConvergence
		}
	}
	evaluation := geom.SurfacePatchEvaluationOptions{MaxOperations: o.Surface.MaxOperations, MaxCoefficientBits: o.Surface.MaxCoefficientBits}
	x, d := [3]*big.Rat{ratInset(origin.X), ratInset(origin.Y), ratInset(origin.Z)},
		[3]*big.Rat{ratInset(direction.X), ratInset(direction.Y), ratInset(direction.Z)}
	point := func(t *big.Rat) [3]*big.Rat {
		var y [3]*big.Rat
		for i := range y {
			y[i] = new(big.Rat).Add(x[i], new(big.Rat).Mul(t, d[i]))
		}
		return y
	}
	refine := geom.ExactCurvePlaneEventOptions{MaxOperations: o.Surface.MaxOperations,
		MaxCoefficientBits: o.Surface.MaxCoefficientBits, MaxSubdivisions: o.MaxSubdivisions}
	if refine.MaxOperations == 0 {
		refine.MaxOperations = 8000000
	}
	if refine.MaxSubdivisions == 0 {
		refine.MaxSubdivisions = 8192
	}
	spend := func(work, subdivisions int) {
		refine.MaxOperations -= work
		refine.MaxSubdivisions -= subdivisions
	}
	width := new(big.Rat).Sub(r.Clip.Upper, r.Clip.Lower)
	normalize := func(t *big.Rat) *big.Rat { return new(big.Rat).Quo(new(big.Rat).Sub(t, r.Clip.Lower), width) }
	original := func(t *big.Rat) *big.Rat { return new(big.Rat).Add(r.Clip.Lower, new(big.Rat).Mul(t, width)) }
	type event struct {
		h     geom.PiecewiseGraphLineHit
		proof *geom.ExactGraphLineRoot
	}
	var top [2][]event
	for i := range top {
		for _, h := range r.Graph[i+1].Hits {
			top[i] = append(top[i], event{h: h})
		}
	}
	prove := func(i int, e *event) error {
		if e.proof != nil {
			return nil
		}
		if width.Sign() == 0 || len(e.h.Pieces) == 0 || refine.MaxOperations < 1 || refine.MaxSubdivisions < 1 {
			return geom.ErrConvergence
		}
		q := e.h.Parameter
		p, err := geom.NewExactGraphLineRoot(ctx, r.pieces[i+1].Pieces[e.h.Pieces[0]],
			r.clipHeight[0], r.clipHeight[1], geom.ExactParameterInterval{Lower: normalize(q.Min), Upper: normalize(q.Max)}, refine)
		e.proof = p
		if err == nil {
			spend(p.ExactOperations(), p.Subdivisions())
		}
		return err
	}
	// Reconcile the two complete event sets. Exact algebraic identity
	// decides common roots; strict order refines different brackets.
	var hits []SharedBaseGraphEnvelopeHit
	for _, h := range r.Graph[0].Hits {
		hits = append(hits, SharedBaseGraphEnvelopeHit{Parameter: h.Parameter, Base: true})
	}
	common := [2][]bool{make([]bool, len(top[0])), make([]bool, len(top[1]))}
	for i, j := 0, 0; i < len(top[0]) && j < len(top[1]); {
		a, b := &top[0][i], &top[1][j]
		p, q := &a.h.Parameter, &b.h.Parameter
		if p.Max.Cmp(q.Min) < 0 {
			i++
			continue
		}
		if q.Max.Cmp(p.Min) < 0 {
			j++
			continue
		}
		order := 0
		if p.Min.Cmp(p.Max) != 0 || q.Min.Cmp(q.Max) != 0 || p.Min.Cmp(q.Min) != 0 {
			if e := prove(0, a); e != nil {
				return nil, e
			}
			if e := prove(1, b); e != nil {
				return nil, e
			}
			if refine.MaxOperations < 1 || refine.MaxSubdivisions < 1 {
				return nil, geom.ErrConvergence
			}
			c, e := geom.CompareExactGraphLineRoots(ctx, a.proof, b.proof, refine)
			if e != nil {
				return nil, e
			}
			if !c.Proven {
				return nil, geom.ErrConvergence
			}
			spend(c.Operations, c.Subdivisions)
			order = c.Order
			p.Min, p.Max = maxRat(p.Min, original(c.Intervals[0][0])), minRat(p.Max, original(c.Intervals[0][1]))
			q.Min, q.Max = maxRat(q.Min, original(c.Intervals[1][0])), minRat(q.Max, original(c.Intervals[1][1]))
		}
		if order < 0 && p.Max.Cmp(q.Min) >= 0 || order > 0 && q.Max.Cmp(p.Min) >= 0 {
			return nil, geom.ErrConvergence
		}
		switch order {
		case -1:
			i++
		case 1:
			j++
		default:
			hits = append(hits, SharedBaseGraphEnvelopeHit{Parameter: geom.ToroidLineRoot{
				Min: maxRat(p.Min, q.Min), Max: minRat(p.Max, q.Max), Multiplicity: max(p.Multiplicity, q.Multiplicity),
			}, TopFaces: []int{0, 1}})
			common[0][i], common[1][j] = true, true
			i++
			j++
		}
	}
	for i, other := range [2]*geom.SurfacePatch{p.b, p.a} {
		for j := range top[i] {
			if common[i][j] {
				continue
			}
			h := &top[i][j]
			a := h.h.Parameter
			sign := 0
			if a.Min.Cmp(a.Max) == 0 {
				v := point(a.Min)
				q, e := other.EvaluateRational(ctx, v[0], v[1], evaluation)
				if e != nil {
					return nil, e
				}
				sign = q.Point[2].Cmp(v[2])
			} else {
				if e := prove(i, h); e != nil {
					return nil, e
				}
				lo, hi := normalize(a.Min), normalize(a.Max)
				proved := false
				for _, piece := range r.pieces[2-i].Pieces {
					if hi.Cmp(piece.Start) < 0 || lo.Cmp(piece.End) > 0 {
						continue
					}
					if refine.MaxOperations < 1 || refine.MaxSubdivisions < 1 {
						return nil, geom.ErrConvergence
					}
					q, e := geom.ClassifyExactGraphRootHeight(ctx, h.proof, piece, r.clipHeight[0], r.clipHeight[1], refine)
					spend(q.Operations, q.Subdivisions)
					if errors.Is(e, geom.ErrDomain) {
						continue
					}
					if e != nil {
						return nil, e
					}
					if !q.Proven {
						return nil, geom.ErrConvergence
					}
					sign, proved = q.Side, true
					a.Min, a.Max = maxRat(a.Min, original(q.Interval.Lower)), minRat(a.Max, original(q.Interval.Upper))
					break
				}
				if !proved {
					return nil, geom.ErrConvergence
				}
			}
			if sign == 0 {
				return nil, geom.ErrConvergence
			}
			if (sign < 0) == (op == parametric.Union) {
				hits = append(hits, SharedBaseGraphEnvelopeHit{Parameter: a, TopFaces: []int{i}})
			}
		}
	}
	for i := range hits {
		if !hits[i].Base {
			continue
		}
		base := &hits[i].Parameter
		for j := range hits {
			if len(hits[j].TopFaces) == 0 {
				continue
			}
			top := &hits[j].Parameter
			if base.Max.Cmp(top.Min) < 0 || top.Max.Cmp(base.Min) < 0 {
				continue
			}
			if width.Sign() == 0 || refine.MaxOperations < 1 || refine.MaxSubdivisions < 1 {
				return nil, geom.ErrConvergence
			}
			makeProof := func(g int, q geom.ToroidLineRoot) (*geom.ExactGraphLineRoot, error) {
				l, h := normalize(q.Min), normalize(q.Max)
				for _, p := range r.pieces[g].Pieces {
					if h.Cmp(p.Start) < 0 || l.Cmp(p.End) > 0 {
						continue
					}
					if refine.MaxOperations < 1 || refine.MaxSubdivisions < 1 {
						return nil, geom.ErrConvergence
					}
					k, e := geom.NewExactGraphLineRoot(ctx, p, r.clipHeight[0], r.clipHeight[1], geom.ExactParameterInterval{Lower: l, Upper: h}, refine)
					if e == nil {
						spend(k.ExactOperations(), k.Subdivisions())
						return k, nil
					}
					// Isolating brackets stay strictly inside the source
					// piece except for exact knot events. Touching the
					// other adjacent piece can yield no root there.
					if !errors.Is(e, geom.ErrConvergence) && !errors.Is(e, geom.ErrDomain) {
						return nil, e
					}
				}
				return nil, geom.ErrConvergence
			}
			a, e := makeProof(0, *base)
			if e != nil {
				return nil, e
			}
			b, e := makeProof(hits[j].TopFaces[0]+1, *top)
			if e != nil {
				return nil, e
			}
			if refine.MaxOperations < 1 || refine.MaxSubdivisions < 1 {
				return nil, geom.ErrConvergence
			}
			c, e := geom.CompareExactGraphLineRoots(ctx, a, b, refine)
			if e != nil {
				return nil, e
			}
			if !c.Proven || c.Order == 0 {
				return nil, geom.ErrConvergence
			}
			spend(c.Operations, c.Subdivisions)
			base.Min, base.Max = maxRat(base.Min, original(c.Intervals[0][0])), minRat(base.Max, original(c.Intervals[0][1]))
			top.Min, top.Max = maxRat(top.Min, original(c.Intervals[1][0])), minRat(top.Max, original(c.Intervals[1][1]))
		}
	}
	for _, t := range [2]*big.Rat{r.Clip.Lower, r.Clip.Upper} {
		v := point(t)
		sides := make([]int, 0, 2)
		for i, z := range [4]*big.Rat{v[1], new(big.Rat).Sub(big.NewRat(1, 1), v[0]),
			new(big.Rat).Sub(big.NewRat(1, 1), v[1]), v[0]} {
			if z.Sign() == 0 {
				sides = append(sides, i)
			}
		}
		if len(sides) == 0 {
			continue
		}
		s, e := p.SectionEnvelopeVertical(ctx, [2]*big.Rat{v[0], v[1]}, op, evaluation)
		if e != nil {
			return nil, e
		}
		if v[2].Cmp(s.Lower) >= 0 && v[2].Cmp(s.Upper) <= 0 {
			hits = append(hits, SharedBaseGraphEnvelopeHit{
				Parameter: geom.ToroidLineRoot{Min: new(big.Rat).Set(t), Max: new(big.Rat).Set(t), Multiplicity: 1},
				Sides:     sides,
			})
		}
	}
	if side >= 0 {
		for i := range hits {
			hits[i].Sides = appendUniqueFace(hits[i].Sides, side)
			sort.Ints(hits[i].Sides)
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Parameter.Min.Cmp(hits[j].Parameter.Min) < 0 })
	result := hits[:0]
	for _, h := range hits {
		if len(result) != 0 {
			last := &result[len(result)-1]
			a, b := last.Parameter, h.Parameter
			if a.Max.Cmp(b.Min) >= 0 {
				if a.Min.Cmp(a.Max) != 0 || b.Min.Cmp(b.Max) != 0 || a.Min.Cmp(b.Min) != 0 {
					return nil, geom.ErrConvergence
				}
				last.Parameter.Multiplicity = max(a.Multiplicity, b.Multiplicity)
				last.Base = last.Base || h.Base
				for _, i := range h.TopFaces {
					last.TopFaces = appendUniqueFace(last.TopFaces, i)
				}
				for _, i := range h.Sides {
					last.Sides = appendUniqueFace(last.Sides, i)
				}
				continue
			}
		}
		result = append(result, h)
	}
	return result, nil
}

func minRat(a, b *big.Rat) *big.Rat {
	if a.Cmp(b) <= 0 {
		return a
	}
	return b
}
func maxRat(a, b *big.Rat) *big.Rat {
	if a.Cmp(b) >= 0 {
		return a
	}
	return b
}

func graphEnvelopeLineSide(origin, direction geom.Vec3) int {
	switch {
	case direction.Y == 0 && origin.Y == 0:
		return 0
	case direction.X == 0 && origin.X == 1:
		return 1
	case direction.Y == 0 && origin.Y == 1:
		return 2
	case direction.X == 0 && origin.X == 0:
		return 3
	}
	return -1
}

// SharedBaseGraphSideLine reports all material intervals and isolated
// boundary events on a finite line in one side-wall plane. Each span
// endpoint is an exact rational or a certified algebraic bracket.
type SharedBaseGraphSideLine struct {
	Side  int
	Hits  []SharedBaseGraphEnvelopeHit
	Spans [][2]geom.ToroidLineRoot
}

// IntersectEnvelopeSideLine proves the material on a nonvertical line
// contained in an original unit-chart side plane. It retains isolated
// tangent contacts and merges adjacent material intervals through them.
// Lines coincident with a top or base span remain unresolved.
func (p SharedBaseSplineGraphMaterial) IntersectEnvelopeSideLine(ctx context.Context, origin, direction geom.Vec3,
	minT, maxT float64, op parametric.BooleanOp, o geom.ToroidLineOptions) (SharedBaseGraphSideLine, error) {
	empty := SharedBaseGraphSideLine{Side: -1}
	side := graphEnvelopeLineSide(origin, direction)
	if side < 0 || direction.X == 0 && direction.Y == 0 {
		return empty, geom.ErrInvalid
	}
	hits, e := p.intersectEnvelopeLine(ctx, origin, direction, minT, maxT, op, o, true)
	if e != nil {
		return empty, e
	}
	r := SharedBaseGraphSideLine{Side: side, Hits: hits}
	if minT == maxT && len(hits) == 0 {
		t := ratInset(minT)
		var point [3]*big.Rat
		for i, x := range [3]float64{origin.X, origin.Y, origin.Z} {
			d := [3]float64{direction.X, direction.Y, direction.Z}[i]
			point[i] = new(big.Rat).Add(ratInset(x), new(big.Rat).Mul(t, ratInset(d)))
		}
		loc, e := p.LocateEnvelopeRational(ctx, point, op, geom.SurfacePatchEvaluationOptions{
			MaxOperations: o.Surface.MaxOperations, MaxCoefficientBits: o.Surface.MaxCoefficientBits,
		})
		if e != nil {
			return empty, e
		}
		if loc == parametric.Boundary {
			r.Hits = append(r.Hits, SharedBaseGraphEnvelopeHit{Parameter: geom.ToroidLineRoot{Min: ratInset(minT), Max: ratInset(maxT), Multiplicity: 1}, Sides: []int{side}})
		}
		return r, nil
	}
	r.Spans, e = p.graphMaterialIntervals(ctx, origin, direction, minT, maxT, op, o, hits)
	if e != nil {
		return empty, e
	}
	return r, nil
}
