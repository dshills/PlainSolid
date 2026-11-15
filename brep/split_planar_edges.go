package brep

import (
	"context"
	"fmt"
	"math/big"
	"sort"

	"plainsolid/geom"
)

// PlanarEdgeSplit declares exact interior points on an original planar edge.
// Points need not arrive in the original edge direction.
type PlanarEdgeSplit struct {
	Edge   int
	Points []geom.Vec3
}

// PlanarEdgeChild describes an oriented result edge in the parent direction.
// Range is expressed as two exact parameters in the parent interval [0,1].
type PlanarEdgeChild struct {
	Edge  int
	Range [2]*big.Rat
}

// PlanarEdgeSplitHistory gives the ordered children of each original edge.
// Original vertex and face indices remain unchanged.
type PlanarEdgeSplitHistory struct {
	Children [][]PlanarEdgeChild
}

type planarSplitStation struct {
	p geom.Vec3
	t *big.Rat
}

// PlanarEdgeParameter locates a represented point on a finite stored edge
// without snapping and returns its exact original fraction. A point off the
// segment returns geom.ErrDomain.
func (s Solid) PlanarEdgeParameter(id int, p geom.Vec3) (*big.Rat, error) {
	if id < 0 || id >= len(s.edges) || !p.IsFinite() {
		return nil, geom.ErrInvalid
	}
	e := s.edges[id]
	a, b := s.vertices[e.Start], s.vertices[e.End]
	if p == a {
		return new(big.Rat), nil
	}
	if p == b {
		return big.NewRat(1, 1), nil
	}
	if !strictSegmentStation(a, p, b) {
		return nil, geom.ErrDomain
	}
	collinear, err := collinearPlanarStation(a, p, b)
	if err != nil {
		return nil, err
	}
	if !collinear {
		return nil, geom.ErrDomain
	}
	origin, point, end := a.X, p.X, b.X
	if origin == end {
		origin, point, end = a.Y, p.Y, b.Y
	}
	if origin == end {
		origin, point, end = a.Z, p.Z, b.Z
	}
	f := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	return new(big.Rat).Quo(new(big.Rat).Sub(f(point), f(origin)), new(big.Rat).Sub(f(end), f(origin))), nil
}

// SplitPlanarEdges preserves every original face and vertex and inserts the
// specified stations into both original coedges and the conforming tessellation.
// Zero maxCuts allows up to 100,000 stations. No partial result is published.
func SplitPlanarEdges(ctx context.Context, s Solid, splits []PlanarEdgeSplit, maxCuts int) (Solid, PlanarEdgeSplitHistory, error) {
	var zero PlanarEdgeSplitHistory
	if ctx == nil || maxCuts < 0 {
		return Solid{}, zero, geom.ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return Solid{}, zero, err
	}
	if maxCuts == 0 {
		maxCuts = 100000
	}
	if len(splits) > maxCuts {
		return Solid{}, zero, ErrTriangulationLimit
	}
	stations := make(map[int][]planarSplitStation, len(splits))
	total := 0
	for _, u := range splits {
		if u.Edge < 0 || u.Edge >= len(s.edges) || len(u.Points) == 0 || stations[u.Edge] != nil {
			return Solid{}, zero, geom.ErrInvalid
		}
		if len(u.Points) > maxCuts-total {
			return Solid{}, zero, ErrTriangulationLimit
		}
		total += len(u.Points)
		list := make([]planarSplitStation, len(u.Points))
		for i, p := range u.Points {
			if err := ctx.Err(); err != nil {
				return Solid{}, zero, err
			}
			t, err := s.PlanarEdgeParameter(u.Edge, p)
			if err != nil {
				return Solid{}, zero, fmt.Errorf("brep: edge %d split station %d: %w", u.Edge, i, err)
			}
			if t.Sign() == 0 || t.Cmp(big.NewRat(1, 1)) >= 0 {
				return Solid{}, zero, fmt.Errorf("%w: original edge split point is not interior", geom.ErrDomain)
			}
			list[i] = planarSplitStation{p: p, t: t}
		}
		sort.Slice(list, func(i, j int) bool { return list[i].t.Cmp(list[j].t) < 0 })
		for i := 1; i < len(list); i++ {
			if list[i-1].t.Cmp(list[i].t) == 0 {
				return Solid{}, zero, fmt.Errorf("%w: duplicate original edge split station", geom.ErrInvalid)
			}
		}
		stations[u.Edge] = list
	}
	v, e, f := s.Vertices(), s.Edges(), s.Faces()
	h := PlanarEdgeSplitHistory{Children: make([][]PlanarEdgeChild, len(e))}
	for i, old := range s.edges {
		if err := ctx.Err(); err != nil {
			return Solid{}, zero, err
		}
		points := stations[i]
		if len(points) == 0 {
			h.Children[i] = []PlanarEdgeChild{{Edge: i, Range: [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}}}
			continue
		}
		currentVertex, currentParameter := old.Start, new(big.Rat)
		for j, u := range points {
			id := len(v)
			v = append(v, u.p)
			child := Edge{Start: currentVertex, End: id}
			childID := i
			if j == 0 {
				e[i] = child
			} else {
				childID = len(e)
				e = append(e, child)
			}
			h.Children[i] = append(h.Children[i], PlanarEdgeChild{Edge: childID, Range: [2]*big.Rat{new(big.Rat).Set(currentParameter), new(big.Rat).Set(u.t)}})
			currentVertex, currentParameter = id, u.t
		}
		h.Children[i] = append(h.Children[i], PlanarEdgeChild{Edge: len(e), Range: [2]*big.Rat{new(big.Rat).Set(currentParameter), big.NewRat(1, 1)}})
		e = append(e, Edge{Start: currentVertex, End: old.End})
	}
	expand := func(w []EdgeUse) []EdgeUse {
		r := make([]EdgeUse, 0, len(w))
		for _, u := range w {
			k := h.Children[u.Edge]
			for j := range k {
				if u.Reversed {
					j = len(k) - 1 - j
				}
				r = append(r, EdgeUse{Edge: k[j].Edge, Reversed: u.Reversed})
			}
		}
		return r
	}
	for i := range f {
		if err := ctx.Err(); err != nil {
			return Solid{}, zero, err
		}
		f[i].Boundary = expand(f[i].Boundary)
		for j, wire := range f[i].Holes {
			f[i].Holes[j] = expand(wire)
		}
	}
	out, err := NewSolid(v, e, f)
	if err != nil {
		return Solid{}, zero, err
	}
	if err := ctx.Err(); err != nil {
		return Solid{}, zero, err
	}
	return out, h, nil
}
