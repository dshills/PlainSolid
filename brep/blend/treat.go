package blend

import (
	"context"
	"errors"
	"fmt"
	"math"

	"plainsolid/brep"
	"plainsolid/brep/boolean"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// Kind selects the treatment of an edge.
type Kind uint8

const (
	// Chamfer replaces the edge by a plane that meets each of its faces
	// at the size, measured along the face from the edge.
	Chamfer Kind = iota
)

// Body is the solid to treat, with what the caller knows of it.
type Body struct {
	Solid trimmed.Solid
	// Known holds the volume and the area of Solid, or is nil.
	Known *parametric.Properties
	// Solids is the number of solids of Solid.
	Solids int
}

// Options carries the options of every boolean.Combine call of a
// treatment and the tolerances of its references.
type Options struct {
	Boolean   boolean.Options
	Tolerance Tolerance
}

// The ways a treatment is refused, besides an error of boolean.Combine,
// which Treat returns as it is.
var (
	// ErrSize: the tool of an edge cannot be built at the size.
	ErrSize = errors.New("blend: the tool of the treatment cannot be built at this size")
	// ErrFit: the size exceeds the stock, or a void, another feature or
	// the far side of the stock lies within the size or touches the new
	// face (K1, K3, K4).
	ErrFit = errors.New("blend: the treatment does not fit the stock")
	// ErrEnds: the tool does not end on faces of the stock, or reaches
	// under a neighbouring feature (K2).
	ErrEnds = errors.New("blend: a support face of the tool is left in the result")
	// ErrDivides: the treatment divides the stock, or leaves an edge in
	// place (K5, K6, K7).
	ErrDivides = errors.New("blend: the treatment divides the stock or leaves the edge in place")
)

// tool is a solid to cut from a body or to unite with it. A working face
// becomes a face of the result; every other face is a support, which
// must leave no trace. owners lists, for a working face, the edges of
// the request it belongs to.
type tool struct {
	solid   trimmed.Solid
	working []bool
	owners  [][]int
}

// Treat applies one treatment of the given size to edges of body that
// Locate resolved in body.Solid, and returns the result of the last
// boolean.Combine call. It makes at most 2n + 1 such calls for n edges.
// Nothing is returned unless every check on the result passes (K1 to
// K7).
func Treat(ctx context.Context, body Body, edges []Edge, kind Kind, size float64, o Options) (boolean.Result, error) {
	if len(edges) == 0 || !(size > o.Tolerance.Point) || math.IsInf(size, 0) {
		return boolean.Result{}, ErrSize
	}
	tools := make([]tool, len(edges))
	call := make([]int, len(edges))
	for i, e := range edges {
		if !e.convex {
			return boolean.Result{}, fmt.Errorf("%w: a concave edge", boolean.ErrUnsupported)
		}
		t, err := e.tool(i, kind, size, o)
		if err != nil {
			return boolean.Result{}, err
		}
		tools[i], call[i] = t, i
	}
	united, err := unite(ctx, tools, o)
	if err != nil {
		return boolean.Result{}, err
	}
	options := o.Boolean
	options.KnownA = body.Known
	r, err := boolean.Combine(ctx, body.Solid, united.solid, boolean.Cut, options)
	if err != nil {
		return boolean.Result{}, err
	}
	if err := check(r, body.Solids, united, edges, call, false, o); err != nil {
		return boolean.Result{}, err
	}
	if err := gone(r.Solid, edges, o.Tolerance); err != nil {
		return boolean.Result{}, err
	}
	return r, nil
}

// tool builds the tool of one edge (T1, T3). index is the place of the
// edge in the request.
func (e Edge) tool(index int, _ Kind, size float64, o Options) (tool, error) {
	// A section thinner than the point tolerance is no section.
	if !(2*size*math.Sin(e.angle/2) > o.Tolerance.Point) {
		return tool{}, ErrSize
	}
	section := []geom.Vec3{e.start, e.start.Add(e.into[0].Scale(size)), e.start.Add(e.into[1].Scale(size))}
	prism, err := brep.Prism(section, e.end.Sub(e.start))
	if err != nil {
		return tool{}, ErrSize
	}
	solid, err := trimmed.FromPlanar(prism, o.Tolerance.Point)
	if err != nil {
		return tool{}, ErrSize
	}
	// The working face of the prism is the one face with no vertex on
	// the line of the edge.
	axis := e.end.Sub(e.start)
	axis = axis.Scale(1 / axis.Length())
	vertices, edges := solid.Vertices(), solid.Edges()
	out := tool{solid: solid, working: make([]bool, len(solid.Faces())), owners: make([][]int, len(solid.Faces()))}
	for i, face := range solid.Faces() {
		out.working[i] = true
		for _, u := range face.Outer {
			for _, v := range [2]int{edges[u.Edge].Start, edges[u.Edge].End} {
				d := vertices[v].Sub(e.start)
				if d.Sub(axis.Scale(d.Dot(axis))).Length() <= o.Tolerance.Point {
					out.working[i] = false
				}
			}
		}
		if out.working[i] {
			out.owners[i] = []int{index}
		}
	}
	return out, nil
}

// unite makes one tool of several (B1). A face of the united tool is a
// working face when every one of its sources is one, and then belongs to
// every edge whose tool contributed a source (B2).
func unite(ctx context.Context, tools []tool, o Options) (tool, error) {
	all := tools[0]
	for _, next := range tools[1:] {
		r, err := boolean.Combine(ctx, all.solid, next.solid, boolean.Union, o.Boolean)
		if err != nil {
			return tool{}, err
		}
		both := tool{solid: r.Solid, working: make([]bool, len(r.Sources)), owners: make([][]int, len(r.Sources))}
		for i, sources := range r.Sources {
			both.working[i] = len(sources) > 0
			for _, source := range sources {
				from := all
				if source[0] == 1 {
					from = next
				}
				both.working[i] = both.working[i] && from.working[source[1]]
				both.owners[i] = append(both.owners[i], from.owners[source[1]]...)
			}
			if !both.working[i] {
				both.owners[i] = nil
			}
		}
		all = both
	}
	return all, nil
}

// flat is a plane a face is held against: a treatment plane of K3.
type flat struct {
	point, normal geom.Vec3
	// signed asks for the normal of the face itself, and not only for a
	// parallel one.
	signed bool
}

func (p flat) holds(point, normal geom.Vec3, t Tolerance) bool {
	return normal.Cross(p.normal).Length() <= t.Angular && (!p.signed || normal.Dot(p.normal) > 0) &&
		math.Abs(point.Sub(p.point).Dot(p.normal)) <= t.Point
}

// check applies K1 to K6 to the result r of combining a body of the
// given number of solids with the tool t, which holds the tools of the
// edges listed in call. union tells a union from a cut.
func check(r boolean.Result, solids int, t tool, edges []Edge, call []int, union bool, o Options) error {
	// K1: something is left.
	if r.Solids == 0 || !(r.Volume > 0) {
		return ErrFit
	}
	// K2: no support face of the tool is a source of a face.
	working := make([]bool, len(r.Sources))
	served := map[int]bool{}
	for i, sources := range r.Sources {
		for _, source := range sources {
			if source[0] != 1 {
				continue
			}
			if !t.working[source[1]] {
				return ErrEnds
			}
			working[i] = true
			for _, e := range t.owners[source[1]] {
				served[e] = true
			}
		}
	}
	// K3: a face that shares a vertex with a new face is a new face or
	// lies in a treatment plane.
	var planes []flat
	for _, i := range call {
		e := edges[i]
		axis := e.end.Sub(e.start)
		axis = axis.Scale(1 / axis.Length())
		planes = append(planes, flat{e.start, e.normal[0], true}, flat{e.start, e.normal[1], true},
			flat{e.start, axis, false}, flat{e.end, axis, false})
	}
	if !clearance(r.Solid, working, planes, o.Tolerance) {
		return ErrFit
	}
	// K4: the volume that changed is the volume of the tool.
	changed := r.VolumeA - r.Volume
	if union {
		changed = r.Volume - r.VolumeA
	}
	if math.Abs(changed-r.VolumeB) > o.Boolean.Volume*r.VolumeB {
		return ErrFit
	}
	// K5: the solids are as many as before.
	if r.Solids != solids {
		return ErrDivides
	}
	// K6: every edge of the call left a face.
	for _, i := range call {
		if !served[i] {
			return ErrDivides
		}
	}
	return nil
}

// clearance is K3 on a solid whose new faces are marked.
func clearance(s trimmed.Solid, working []bool, planes []flat, t Tolerance) bool {
	edges, faces := s.Edges(), s.Faces()
	corners := func(face trimmed.Face, visit func(v int)) {
		for _, loop := range append([][]trimmed.Use{face.Outer}, face.Holes...) {
			for _, u := range loop {
				visit(edges[u.Edge].Start)
				visit(edges[u.Edge].End)
			}
		}
	}
	touched := map[int]bool{}
	for i, face := range faces {
		if working[i] {
			corners(face, func(v int) { touched[v] = true })
		}
	}
	for i, face := range faces {
		if working[i] {
			continue
		}
		beside := false
		corners(face, func(v int) { beside = beside || touched[v] })
		if !beside {
			continue
		}
		if base, _ := parametric.SurfaceMapping(face.Surface); !isPlane(base) {
			return false
		}
		at := face.Outer[0].UVStart
		point, err := face.Surface.Eval(at.X, at.Y)
		if err != nil {
			return false
		}
		du, dv, err := face.Surface.Partials(at.X, at.Y)
		if err != nil {
			return false
		}
		normal, err := du.Cross(dv).Unit()
		if err != nil {
			return false
		}
		if face.Reversed {
			normal = normal.Scale(-1)
		}
		allowed := false
		for _, p := range planes {
			allowed = allowed || p.holds(point, normal, t)
		}
		if !allowed {
			return false
		}
	}
	return true
}

// gone is K7: no edge of the request resolves in the result.
func gone(s trimmed.Solid, edges []Edge, t Tolerance) error {
	for _, e := range edges {
		if _, err := Locate(s, e.start, e.end, t); err == nil {
			return ErrDivides
		}
	}
	return nil
}
