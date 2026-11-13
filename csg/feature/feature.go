// Package feature constructs bounded planar sketch protrusions and blind
// pockets with exact input classification and planar Boolean provenance.
package feature

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"plainsolid/brep"
	"plainsolid/brep/support"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

var ErrInvalid = errors.New("feature: invalid convex base, selected face or sketch")
var ErrThrough = errors.New("feature: pocket reaches or crosses another support")
var ErrPrecision = errors.New("feature: extrusion coordinates are not exactly representable")
var ErrLimit = errors.New("feature: resource limit exceeded")

type Kind uint8

const (
	Protrusion Kind = iota
	Pocket
)

// Options bounds feature input validation separately from the Boolean engine.
// Zero defaults to 256 sketch vertices, 256 base faces and 100000 validation
// predicates. Boolean options and its additional history work budget are passed
// unchanged to poly.CombineWithTopologyHistory.
type Options struct {
	MaxSketchVertices, MaxBaseFaces, MaxWork int
	Boolean                                  poly.Options
}

// Result maps poly history operand A to Base and B to Tool. Mouth and Terminal
// are tool face indices. SketchWalls[i] is the tool face swept from sketch edge
// i->(i+1)%n, regardless of input winding. The closed tool is returned so every
// tool edge/triangle/face reference can be resolved without inferred naming.
type Result struct {
	Solid, Tool     brep.Solid
	History         poly.TopologyHistory
	Mouth, Terminal int
	SketchWalls     []int
}
type point [3]*big.Rat

func vec(v geom.Vec3) point {
	return point{new(big.Rat).SetFloat64(v.X), new(big.Rat).SetFloat64(v.Y), new(big.Rat).SetFloat64(v.Z)}
}
func add(a, b *big.Rat) *big.Rat { return new(big.Rat).Add(a, b) }
func sub(a, b *big.Rat) *big.Rat { return new(big.Rat).Sub(a, b) }
func mul(a, b *big.Rat) *big.Rat { return new(big.Rat).Mul(a, b) }
func diff(a, b point) point      { return point{sub(a[0], b[0]), sub(a[1], b[1]), sub(a[2], b[2])} }
func cross(a, b point) point {
	return point{sub(mul(a[1], b[2]), mul(a[2], b[1])), sub(mul(a[2], b[0]), mul(a[0], b[2])), sub(mul(a[0], b[1]), mul(a[1], b[0]))}
}
func dot(a, b point) *big.Rat { return add(add(mul(a[0], b[0]), mul(a[1], b[1])), mul(a[2], b[2])) }

type plane struct {
	n point
	d *big.Rat
}

func (p plane) distance(v point) *big.Rat { return sub(dot(p.n, v), p.d) }

// Extrude creates a protrusion outside a selected planar face, or removes a
// blind pocket inside it. Offset is the full translation, including any oblique
// component. Sketch coordinates must lie exactly on the selected support and
// strictly inside its convex polygon. The simple sketch may be concave, but
// holes and boundary contacts are unsupported. Pocket tool vertices must remain
// strictly inside every other noncoplanar base support.
func Extrude(ctx context.Context, base brep.Solid, face int, sketch []geom.Vec3, offset geom.Vec3, kind Kind, o Options) (Result, error) {
	return extrude(ctx, base, face, -1, sketch, offset, kind, o, false)
}

// ThroughPocket removes a translated sketch between explicit opposite parallel
// entry and exit faces. Both openings must be strictly inside their polygons;
// the translation must land exactly on the exit plane. Side breakthrough and
// nonparallel exits are rejected. Both cap faces disappear from output ancestry.
func ThroughPocket(ctx context.Context, base brep.Solid, entry, exit int, sketch []geom.Vec3, offset geom.Vec3, o Options) (Result, error) {
	if exit < 0 || exit == entry || exit >= len(base.Faces()) {
		return Result{}, ErrInvalid
	}
	return extrude(ctx, base, entry, exit, sketch, offset, Pocket, o, false)
}
func extrude(ctx context.Context, base brep.Solid, face, exit int, sketch []geom.Vec3, offset geom.Vec3, kind Kind, o Options, toolOnly bool) (Result, error) {
	empty := Result{}
	if ctx == nil || kind > Pocket || !offset.IsFinite() {
		return empty, ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return empty, e
	}
	for _, v := range []*int{&o.MaxSketchVertices, &o.MaxBaseFaces, &o.MaxWork} {
		if *v < 0 {
			return empty, ErrInvalid
		}
	}
	if o.MaxSketchVertices == 0 {
		o.MaxSketchVertices = 256
	}
	if o.MaxBaseFaces == 0 {
		o.MaxBaseFaces = 256
	}
	if o.MaxWork == 0 {
		o.MaxWork = 100000
	}
	faces, edges, vertices := base.Faces(), base.Edges(), base.Vertices()
	if len(sketch) > o.MaxSketchVertices || len(faces) > o.MaxBaseFaces {
		return empty, ErrLimit
	}
	if len(sketch) < 3 || face < 0 || face >= len(faces) || base.Topology().Shells != 1 || len(faces[face].Holes) != 0 {
		return empty, ErrInvalid
	}
	work := 0
	tick := func() error {
		if e := ctx.Err(); e != nil {
			return e
		}
		work++
		if work > o.MaxWork {
			return ErrLimit
		}
		return nil
	}
	ps := make([]point, len(vertices))
	for i, v := range vertices {
		if e := tick(); e != nil {
			return empty, e
		}
		ps[i] = vec(v)
	}
	remaining := o.MaxWork - work
	if remaining <= 0 {
		return empty, ErrLimit
	}
	certificate, e := support.Certify(ctx, base, support.Options{Convex: true, MaxFaces: o.MaxBaseFaces, MaxVertices: len(vertices), MaxWork: remaining})
	if e != nil {
		if errors.Is(e, support.ErrLimit) {
			return empty, fmt.Errorf("%w: %w", ErrLimit, e)
		}
		if errors.Is(e, support.ErrInvalid) || errors.Is(e, support.ErrNonplanar) || errors.Is(e, support.ErrNonconvex) {
			return empty, fmt.Errorf("%w: %w", ErrInvalid, e)
		}
		return empty, e
	}
	work += certificate.Work
	planes := make([]plane, len(faces))
	loops := make([][]int, len(faces))
	for i, f := range faces {
		if len(f.Holes) > 0 {
			return empty, ErrInvalid
		}
		cert := certificate.Planes[i]
		planes[i] = plane{point(cert.Normal), cert.Offset}
		for _, u := range f.Boundary {
			if e := tick(); e != nil {
				return empty, e
			}
			x := edges[u.Edge].Start
			if u.Reversed {
				x = edges[u.Edge].End
			}
			loops[i] = append(loops[i], x)
		}
	}
	selected := planes[face]
	if exit >= 0 {
		other := planes[exit]
		c := cross(selected.n, other.n)
		if dot(c, c).Sign() != 0 || dot(selected.n, other.n).Sign() >= 0 {
			return empty, ErrInvalid
		}
	}
	direction := dot(selected.n, vec(offset)).Sign()
	if (kind == Protrusion && direction <= 0) || (kind == Pocket && direction >= 0) {
		return empty, ErrInvalid
	}
	for _, v := range sketch {
		if !v.IsFinite() {
			return empty, ErrInvalid
		}
		p := vec(v)
		if selected.distance(p).Sign() != 0 {
			return empty, ErrInvalid
		}
		// Every boundary edge defines a strict interior halfplane on this face.
		ids := loops[face]
		for i, a := range ids {
			if e := tick(); e != nil {
				return empty, e
			}
			b := ids[(i+1)%len(ids)]
			if dot(cross(diff(ps[b], ps[a]), diff(p, ps[a])), selected.n).Sign() <= 0 {
				return empty, ErrInvalid
			}
		}
		end := v.Add(offset)
		if !end.IsFinite() {
			return empty, ErrPrecision
		}
		q := vec(end)
		delta := vec(offset)
		for k := range q {
			if q[k].Cmp(add(p[k], delta[k])) != 0 {
				return empty, ErrPrecision
			}
		}
		if exit >= 0 {
			target := planes[exit]
			if target.distance(q).Sign() != 0 {
				return empty, ErrThrough
			}
			ids := loops[exit]
			for i, a := range ids {
				if e := tick(); e != nil {
					return empty, e
				}
				b := ids[(i+1)%len(ids)]
				if dot(cross(diff(ps[b], ps[a]), diff(q, ps[a])), target.n).Sign() <= 0 {
					return empty, ErrThrough
				}
			}
		}
		if kind == Pocket {
			for i, support := range planes {
				if e := tick(); e != nil {
					return empty, e
				}
				if i == face {
					continue
				}
				if dot(cross(support.n, selected.n), cross(support.n, selected.n)).Sign() == 0 && support.distance(p).Sign() == 0 {
					continue
				}
				if support.distance(p).Sign() >= 0 {
					return empty, ErrInvalid
				}
				if exit >= 0 {
					target := planes[exit]
					c := cross(support.n, target.n)
					if dot(c, c).Sign() == 0 && support.distance(q).Sign() == 0 {
						continue
					}
				}
				if support.distance(q).Sign() >= 0 {
					return empty, ErrThrough
				}
			}
		}
	}
	tool, e := brep.Prism(sketch, offset)
	if e != nil {
		return empty, ErrInvalid
	}
	walls := make([]int, len(sketch))
	tv, te := tool.Vertices(), tool.Edges()
	for i, a := range sketch {
		walls[i] = -1
		b := sketch[(i+1)%len(sketch)]
		for j, f := range tool.Faces() {
			if j < 2 {
				continue
			}
			for _, use := range f.Boundary {
				if e := tick(); e != nil {
					return empty, e
				}
				edge := te[use.Edge]
				x, y := tv[edge.Start], tv[edge.End]
				if (x == a && y == b) || (x == b && y == a) {
					walls[i] = j
				}
			}
		}
		if walls[i] < 0 {
			return empty, ErrInvalid
		}
	}
	if toolOnly {
		return Result{Tool: tool, Mouth: 0, Terminal: 1, SketchWalls: walls}, nil
	}
	op := poly.Union
	if kind == Pocket {
		op = poly.Difference
	}
	result, history, e := poly.CombineWithTopologyHistory(ctx, base, tool, op, o.Boolean)
	if e != nil {
		return empty, e
	}
	return Result{result, tool, history, 0, 1, walls}, nil
}
