package unify

import (
	"context"
	"math/big"
	"sort"

	"plainsolid/brep"
	"plainsolid/geom"
)

// HistoryOptions separately limits unification and ancestry work. MaxWork
// defaults to 2000000 edge/edge or vertex/edge comparisons. Arithmetic is
// exact on the represented input coordinates; no proximity matching occurs.
type HistoryOptions struct {
	Unification Options
	MaxWork     int
}

// ParameterInterval is a closed normalized interval on a directed source
// edge (Start to End). A deleted interval denotes lost topological edge
// identity, not removal of points from the material boundary.
type ParameterInterval struct{ Start, End *big.Rat }

// EdgeSpan maps a surviving interval to an output edge. Source parameters
// increase; target parameters may decrease when edge directions disagree.
type EdgeSpan struct {
	Edge           int
	Source, Target ParameterInterval
}

type EdgeHistory struct {
	Spans   []EdgeSpan
	Deleted []ParameterInterval
}

// VertexOnEdge locates a removed subdivision vertex on a retained output edge.
type VertexOnEdge struct {
	Edge      int
	Parameter *big.Rat
}
type VertexHistory struct {
	Vertex int            // output vertex, or -1 when removed
	Edges  []VertexOnEdge // empty for retained vertices and deleted internal seams
}

// History contains caller-owned exact data. Rows follow original edge/vertex
// order; spans follow increasing source parameter, with output index as tie
// breaker. MaxSquaredDisplacement is identically zero: unification changes
// topology without moving geometric points, including removed subdivisions.
type History struct {
	Edges                  []EdgeHistory
	Vertices               []VertexHistory
	MaxSquaredDisplacement *big.Rat
}

// UnifyWithHistory adds exact edge interval and vertex provenance to Unify.
// Partial coverage is represented explicitly through Deleted intervals. Every
// output edge must be fully accounted for by source intervals; otherwise the
// operation fails rather than guessing across a deleted seam. Limits or
// cancellation return no partial solid or history.
func UnifyWithHistory(ctx context.Context, s brep.Solid, o HistoryOptions) (Result, History, error) {
	fail := func(e error) (Result, History, error) { return Result{}, History{}, e }
	if ctx == nil || o.MaxWork < 0 {
		return fail(ErrInvalid)
	}
	if o.MaxWork == 0 {
		o.MaxWork = 2000000
	}
	r, e := Unify(ctx, s, o.Unification)
	if e != nil {
		return fail(e)
	}
	b := &budget{ctx: ctx, left: o.MaxWork}
	source, target := historyPoints(s.Vertices()), historyPoints(r.Solid.Vertices())
	oldEdges, newEdges := s.Edges(), r.Solid.Edges()
	h := History{Edges: make([]EdgeHistory, len(oldEdges)), Vertices: make([]VertexHistory, len(source)), MaxSquaredDisplacement: new(big.Rat)}
	coverage := make([][]ParameterInterval, len(newEdges))
	for i, old := range oldEdges {
		for j, next := range newEdges {
			if e = b.tick(1); e != nil {
				return fail(e)
			}
			a, c := source[old.Start], source[old.End]
			u, ok := historyParameter(a, c, target[next.Start])
			if !ok {
				continue
			}
			v, ok := historyParameter(a, c, target[next.End])
			if !ok {
				continue
			}
			lo, hi := u, v
			if lo.Cmp(hi) > 0 {
				lo, hi = hi, lo
			}
			if lo.Sign() < 0 {
				lo = new(big.Rat)
			}
			if hi.Cmp(big.NewRat(1, 1)) > 0 {
				hi = big.NewRat(1, 1)
			}
			if lo.Cmp(hi) >= 0 {
				continue
			}
			delta := sub(v, u)
			t0, t1 := new(big.Rat).Quo(sub(lo, u), delta), new(big.Rat).Quo(sub(hi, u), delta)
			h.Edges[i].Spans = append(h.Edges[i].Spans, EdgeSpan{j, ParameterInterval{new(big.Rat).Set(lo), new(big.Rat).Set(hi)}, ParameterInterval{t0, t1}})
			x, y := t0, t1
			if x.Cmp(y) > 0 {
				x, y = y, x
			}
			coverage[j] = append(coverage[j], ParameterInterval{new(big.Rat).Set(x), new(big.Rat).Set(y)})
		}
		sort.Slice(h.Edges[i].Spans, func(a, b int) bool {
			x, y := h.Edges[i].Spans[a], h.Edges[i].Spans[b]
			if c := x.Source.Start.Cmp(y.Source.Start); c != 0 {
				return c < 0
			}
			return x.Edge < y.Edge
		})
		intervals := make([]ParameterInterval, len(h.Edges[i].Spans))
		for j, x := range h.Edges[i].Spans {
			intervals[j] = x.Source
		}
		h.Edges[i].Deleted = historyGaps(intervals)
	}
	for _, intervals := range coverage {
		if e = b.tick(1); e != nil {
			return fail(e)
		}
		if len(historyGaps(intervals)) != 0 {
			return fail(ErrUnsupported)
		}
	}
	for i, p := range source {
		h.Vertices[i].Vertex = r.VertexMap[i]
		if r.VertexMap[i] >= 0 {
			continue
		}
		for j, edge := range newEdges {
			if e = b.tick(1); e != nil {
				return fail(e)
			}
			t, ok := historyParameter(target[edge.Start], target[edge.End], p)
			if ok && t.Sign() >= 0 && t.Cmp(big.NewRat(1, 1)) <= 0 {
				h.Vertices[i].Edges = append(h.Vertices[i].Edges, VertexOnEdge{j, t})
			}
		}
	}
	if e = ctx.Err(); e != nil {
		return fail(e)
	}
	return r, h, nil
}

func historyPoints(v []geom.Vec3) []point {
	p := make([]point, len(v))
	for i, x := range v {
		p[i] = point{new(big.Rat).SetFloat64(x.X), new(big.Rat).SetFloat64(x.Y), new(big.Rat).SetFloat64(x.Z)}
	}
	return p
}
func historyParameter(a, b, p point) (*big.Rat, bool) {
	d := diff(b, a)
	axis := -1
	for i, x := range d {
		if x.Sign() != 0 {
			axis = i
			break
		}
	}
	if axis < 0 {
		return nil, false
	}
	t := new(big.Rat).Quo(sub(p[axis], a[axis]), d[axis])
	for i := range p {
		if add(a[i], mul(t, d[i])).Cmp(p[i]) != 0 {
			return nil, false
		}
	}
	return t, true
}
func historyGaps(intervals []ParameterInterval) []ParameterInterval {
	sort.Slice(intervals, func(i, j int) bool {
		if c := intervals[i].Start.Cmp(intervals[j].Start); c != 0 {
			return c < 0
		}
		return intervals[i].End.Cmp(intervals[j].End) < 0
	})
	at := new(big.Rat)
	var gaps []ParameterInterval
	for _, x := range intervals {
		if x.Start.Cmp(at) > 0 {
			gaps = append(gaps, ParameterInterval{new(big.Rat).Set(at), new(big.Rat).Set(x.Start)})
		}
		if x.End.Cmp(at) > 0 {
			at.Set(x.End)
		}
	}
	if at.Cmp(big.NewRat(1, 1)) < 0 {
		gaps = append(gaps, ParameterInterval{new(big.Rat).Set(at), big.NewRat(1, 1)})
	}
	return gaps
}
