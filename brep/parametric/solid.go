// Package parametric models closed solids made of rectangular surface patches.
// Surface and curve geometry are retained independently of tessellation.
// Opposite sides may share an edge (periodic seam); an entire side may collapse
// to one vertex (pole). Arbitrary UV trims are outside this representation.
package parametric

import (
	"errors"
	"fmt"
	"math"

	"plainsolid/geom"
)

var (
	ErrTopology = errors.New("parametric: invalid topology or boundary geometry")
	ErrBudget   = errors.New("parametric: tessellation budget exceeded")
)

// Interval is a finite, strictly increasing parameter range.
type Interval struct{ Min, Max float64 }

func (r Interval) valid() bool { return r.Max > r.Min && finite(r.Max-r.Min) }
func (r Interval) at(t float64) float64 {
	if t == 1 {
		return r.Max
	}
	return r.Min + (r.Max-r.Min)*t
}
func finite(x float64) bool { return !math.IsNaN(x) && !math.IsInf(x, 0) }

// Edge stores its own model-space curve and stable vertex indices. Its curve is
// evaluated from Range.Min at Start to Range.Max at End. A collapsed edge has
// equal vertex indices and no Curve; a periodic edge may have equal endpoints
// and a nonnil Curve.
type Edge struct {
	Start, End int
	Curve      geom.Curve
	Range      Interval
}

// Use records whether a face traverses the stored edge backwards.
type Use struct {
	Edge     int
	Reversed bool
}

// Face describes a rectangular UV patch. Boundary uses follow the positively
// oriented UV rectangle: bottom, right, top, left. Reversed flips the outward
// orientation without changing that traversal. Along a side, its normalized
// parameter must match the edge's normalized parameter (reversed if indicated).
type Face struct {
	Surface  geom.Surface
	U, V     Interval
	Boundary [4]Use
	Reversed bool
}

// Solid copies all topology on construction. The caller supplies immutable
// Curve and Surface implementations safe for concurrent calls. The zero value
// represents an empty solid.
type Solid struct {
	vertices  []geom.Vec3
	edges     []Edge
	faces     []Face
	tolerance float64
}

func (s Solid) Vertices() []geom.Vec3 { return append([]geom.Vec3(nil), s.vertices...) }
func (s Solid) Edges() []Edge         { return append([]Edge(nil), s.edges...) }
func (s Solid) Faces() []Face         { return append([]Face(nil), s.faces...) }
func (s Solid) Tolerance() float64    { return s.tolerance }

// NewSolid checks closure, orientation, vertex links, edge endpoints and
// sampled agreement between face sides and shared edge curves. Samples are a
// consistency check; custom evaluators must agree over their entire domain and
// describe an embedded solid. Geometric self-intersections are not detected.
func NewSolid(vertices []geom.Vec3, edges []Edge, faces []Face, tolerance float64) (Solid, error) {
	if len(vertices) == 0 && len(edges) == 0 && len(faces) == 0 {
		return Solid{}, nil
	}
	if tolerance <= 0 || !finite(tolerance) || len(vertices) == 0 || len(edges) == 0 || len(faces) == 0 {
		return Solid{}, ErrTopology
	}
	s := Solid{append([]geom.Vec3(nil), vertices...), append([]Edge(nil), edges...), append([]Face(nil), faces...), tolerance}
	for i, v := range s.vertices {
		if !v.IsFinite() {
			return Solid{}, fmt.Errorf("%w: vertex %d", ErrTopology, i)
		}
	}
	used := make([]bool, len(vertices))
	for i, e := range s.edges {
		if e.Start < 0 || e.Start >= len(vertices) || e.End < 0 || e.End >= len(vertices) || e.Curve == nil && e.Start != e.End || e.Curve != nil && !e.Range.valid() {
			return Solid{}, fmt.Errorf("%w: edge %d", ErrTopology, i)
		}
		used[e.Start], used[e.End] = true, true
		for k, id := range [2]int{e.Start, e.End} {
			p, err := s.edgePoint(i, float64(k))
			if err != nil || !near(p, vertices[id], tolerance) {
				return Solid{}, fmt.Errorf("%w: edge %d endpoint %d", ErrTopology, i, k)
			}
		}
	}
	for i, b := range used {
		if !b {
			return Solid{}, fmt.Errorf("%w: unused vertex %d", ErrTopology, i)
		}
	}
	counts, orientation := make([]int, len(edges)), make([]int, len(edges))
	for i, f := range s.faces {
		if f.Surface == nil || !f.U.valid() || !f.V.valid() {
			return Solid{}, fmt.Errorf("%w: face %d domain", ErrTopology, i)
		}
		var start, end [4]int
		for side, use := range f.Boundary {
			if use.Edge < 0 || use.Edge >= len(edges) {
				return Solid{}, fmt.Errorf("%w: face %d edge index", ErrTopology, i)
			}
			e := s.edges[use.Edge]
			start[side], end[side] = e.Start, e.End
			if use.Reversed {
				start[side], end[side] = e.End, e.Start
			}
			counts[use.Edge]++
			if use.Reversed != f.Reversed {
				orientation[use.Edge]--
			} else {
				orientation[use.Edge]++
			}
			for k := 0; k <= 8; k++ {
				t := float64(k) / 8
				p, err := f.sidePoint(side, t)
				if err != nil {
					return Solid{}, fmt.Errorf("%w: face %d side %d evaluation: %v", ErrTopology, i, side, err)
				}
				if use.Reversed {
					t = 1 - t
				}
				q, err := s.edgePoint(use.Edge, t)
				if err != nil || !near(p, q, tolerance) {
					return Solid{}, fmt.Errorf("%w: face %d side %d differs from edge %d", ErrTopology, i, side, use.Edge)
				}
			}
		}
		for j, v := range end {
			if v != start[(j+1)%4] {
				return Solid{}, fmt.Errorf("%w: face %d open wire", ErrTopology, i)
			}
		}
	}
	for i, n := range counts {
		if s.edges[i].Curve == nil {
			if n != 1 {
				return Solid{}, fmt.Errorf("%w: collapsed edge %d needs one use", ErrTopology, i)
			}
		} else if n != 2 || orientation[i] != 0 {
			return Solid{}, fmt.Errorf("%w: edge %d needs two opposite uses", ErrTopology, i)
		}
	}
	// A conforming small grid also detects disconnected vertex links. Use the
	// same IDs as the full tessellation so seams and poles are tested here.
	if _, err := s.grid(4, false, 0, 0); err != nil {
		return Solid{}, fmt.Errorf("%w: %v", ErrTopology, err)
	}
	return s, nil
}

func near(a, b geom.Vec3, tolerance float64) bool {
	return a.IsFinite() && b.IsFinite() && a.Sub(b).Length() <= tolerance
}
func (s Solid) edgePoint(i int, t float64) (geom.Vec3, error) {
	e := s.edges[i]
	if e.Curve == nil {
		return s.vertices[e.Start], nil
	}
	return e.Curve.Eval(e.Range.at(t))
}
func (f Face) point(u, v float64) (geom.Vec3, error) { return f.Surface.Eval(f.U.at(u), f.V.at(v)) }
func sideUV(side int, t float64) (u, v float64) {
	switch side {
	case 0:
		return t, 0
	case 1:
		return 1, t
	case 2:
		return 1 - t, 1
	default:
		return 0, 1 - t
	}
}
func (f Face) sidePoint(side int, t float64) (geom.Vec3, error) {
	u, v := sideUV(side, t)
	return f.point(u, v)
}

// iso represents a face side as an independent edge evaluator. Its parameter
// is normalized to [0,1]; orientation follows the face boundary.
type iso struct {
	face Face
	side int
}

func (c iso) Eval(t float64) (geom.Vec3, error) { return c.face.sidePoint(c.side, t) }
func (c iso) Derivative(t float64) (geom.Vec3, error) {
	u, v := sideUV(c.side, t)
	du, dv, err := c.face.Surface.Partials(c.face.U.at(u), c.face.V.at(v))
	if err != nil {
		return geom.Vec3{}, err
	}
	switch c.side {
	case 0:
		return du.Scale(c.face.U.Max - c.face.U.Min), nil
	case 1:
		return dv.Scale(c.face.V.Max - c.face.V.Min), nil
	case 2:
		return du.Scale(c.face.U.Min - c.face.U.Max), nil
	default:
		return dv.Scale(c.face.V.Min - c.face.V.Max), nil
	}
}
