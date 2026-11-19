package blend

import (
	"errors"
	"math"
	"slices"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// Tolerance carries the two tolerances of a reference.
type Tolerance struct {
	// Point is the distance within which two points are one.
	Point float64
	// Angular bounds the sine of the angle between two directions that
	// count as parallel.
	Angular float64
}

// The three ways a reference fails to resolve (R8).
var (
	// ErrLost: no chain of straight edges between two planes joins the
	// two ends, or more than one does.
	ErrLost = errors.New("blend: no complete edge between two planar faces joins the reference")
	// ErrCurved: a chain joins the ends, and one of its faces is not a
	// plane.
	ErrCurved = errors.New("blend: the edge has a face that is not a plane")
	// ErrParallel: a chain joins the ends, between two parallel planes.
	ErrParallel = errors.New("blend: the two faces of the edge are parallel")
)

// Edge is a resolved reference: a chain of straight edges of a solid, end
// to end on one line, all between the same two planes.
type Edge struct {
	// chain lists the edges of the solid, and vertices the vertices they
	// pass through, from start to end.
	chain, vertices []int
	start, end      geom.Vec3
	// normal holds the outward normals of the two faces; into holds, for
	// each, the unit vector in its plane, perpendicular to the edge, that
	// points from the edge into the face.
	normal, into [2]geom.Vec3
	// angle is the angle between the two into vectors, in (0, π).
	angle float64
	// convex reports that the material lies between the two faces.
	convex bool
}

// side is one use of an edge by a face.
type side struct {
	face int
	use  trimmed.Use
}

// plane is what a side tells of its face at the edge.
type plane struct {
	point, normal, into geom.Vec3
}

// Locate resolves the reference from start to end in s (R1 to R8). The
// chain it returns runs from the end of the reference nearest start.
func Locate(s trimmed.Solid, start, end geom.Vec3, t Tolerance) (Edge, error) {
	if !(t.Point > 0) || !(t.Angular > 0) || !start.IsFinite() || !end.IsFinite() {
		return Edge{}, ErrLost
	}
	span := end.Sub(start)
	length := span.Length()
	if !(length > t.Point) {
		return Edge{}, ErrLost
	}
	axis := span.Scale(1 / length)
	vertices, edges := s.Vertices(), s.Edges()
	// along gives the position of a vertex along the reference, and
	// whether the vertex lies on the segment of the reference.
	along := func(v int) (float64, bool) {
		d := vertices[v].Sub(start)
		x := d.Dot(axis)
		return x, d.Sub(axis.Scale(x)).Length() <= t.Point && x >= -t.Point && x <= length+t.Point
	}
	// Every straight edge of the solid that lies on the segment, each
	// turned to run from start to end.
	type piece struct {
		edge, from, to int
		low            float64
	}
	var pieces []piece
	for i, e := range edges {
		if e.Curve == nil || e.Start == e.End {
			continue
		}
		if base, _ := parametric.CurveMapping(e.Curve); !isLine(base) {
			continue
		}
		a, onA := along(e.Start)
		b, onB := along(e.End)
		if !onA || !onB {
			continue
		}
		if a <= b {
			pieces = append(pieces, piece{i, e.Start, e.End, a})
		} else {
			pieces = append(pieces, piece{i, e.End, e.Start, b})
		}
	}
	if len(pieces) == 0 {
		return Edge{}, ErrLost
	}
	slices.SortFunc(pieces, func(p, q piece) int {
		switch {
		case p.low < q.low:
			return -1
		case p.low > q.low:
			return 1
		}
		return p.edge - q.edge
	})
	// The pieces must be one chain from start to end: each begins at the
	// vertex where the one before ends. Two edges over one stretch, or a
	// gap, leave no chain or more than one (R6).
	last := pieces[len(pieces)-1]
	if vertices[pieces[0].from].Sub(start).Length() > t.Point || vertices[last.to].Sub(end).Length() > t.Point {
		return Edge{}, ErrLost
	}
	out := Edge{start: vertices[pieces[0].from], end: vertices[last.to], vertices: []int{pieces[0].from}}
	for i, p := range pieces {
		if i > 0 && p.from != pieces[i-1].to {
			return Edge{}, ErrLost
		}
		out.chain = append(out.chain, p.edge)
		out.vertices = append(out.vertices, p.to)
	}
	// The uses of the chain's edges.
	faces := s.Faces()
	uses := make(map[int][]side, len(out.chain))
	for _, e := range out.chain {
		uses[e] = nil
	}
	for f, face := range faces {
		for _, loop := range append([][]trimmed.Use{face.Outer}, face.Holes...) {
			for _, u := range loop {
				if _, ok := uses[u.Edge]; ok {
					uses[u.Edge] = append(uses[u.Edge], side{f, u})
				}
			}
		}
	}
	for _, e := range out.chain {
		if len(uses[e]) != 2 || uses[e][0].face == uses[e][1].face {
			return Edge{}, ErrLost // a seam, or an edge that is not manifold (R3)
		}
	}
	var first [2]plane
	for i, e := range out.chain {
		var here [2]plane
		for k, sd := range uses[e] {
			face := faces[sd.face]
			if base, _ := parametric.SurfaceMapping(face.Surface); !isPlane(base) {
				return Edge{}, ErrCurved
			}
			p, err := sd.plane(face, axis)
			if err != nil {
				return Edge{}, ErrLost
			}
			here[k] = p
		}
		if i == 0 {
			first = here
			continue
		}
		// The same two planes as the first edge, in either order (R5).
		direct := samePlane(here[0], first[0], t) && samePlane(here[1], first[1], t)
		crossed := samePlane(here[0], first[1], t) && samePlane(here[1], first[0], t)
		if !direct && !crossed {
			return Edge{}, ErrLost
		}
	}
	if first[0].normal.Cross(first[1].normal).Length() <= t.Angular {
		return Edge{}, ErrParallel
	}
	for k, p := range first {
		out.normal[k], out.into[k] = p.normal, p.into
	}
	out.angle = math.Atan2(out.into[0].Cross(out.into[1]).Length(), out.into[0].Dot(out.into[1]))
	out.convex = out.into[1].Dot(out.normal[0]) < 0
	return out, nil
}

func isLine(c geom.Curve) bool    { _, ok := c.(geom.Line); return ok }
func isPlane(s geom.Surface) bool { _, ok := s.(geom.PlaneSurface); return ok }

// plane reads the face of a side at the middle of its use. The outer loop
// of a face is wound positively in the chart and a hole negatively, so
// the face lies to the left of every use in the chart, whatever the
// orientation of the face.
func (sd side) plane(face trimmed.Face, axis geom.Vec3) (plane, error) {
	from, to := sd.use.UVStart, sd.use.UVEnd
	u, v := (from.X+to.X)/2, (from.Y+to.Y)/2
	point, err := face.Surface.Eval(u, v)
	if err != nil {
		return plane{}, err
	}
	du, dv, err := face.Surface.Partials(u, v)
	if err != nil {
		return plane{}, err
	}
	normal, err := du.Cross(dv).Unit()
	if err != nil {
		return plane{}, err
	}
	if face.Reversed {
		normal = normal.Scale(-1)
	}
	left := du.Scale(from.Y - to.Y).Add(dv.Scale(to.X - from.X))
	into, err := left.Sub(axis.Scale(left.Dot(axis))).Unit()
	if err != nil {
		return plane{}, err
	}
	return plane{point, normal, into}, nil
}

// samePlane is the plane test of R5: equal normals and equal offsets.
func samePlane(a, b plane, t Tolerance) bool {
	return a.normal.Dot(b.normal) > 0 && a.normal.Cross(b.normal).Length() <= t.Angular &&
		math.Abs(b.point.Sub(a.point).Dot(a.normal)) <= t.Point
}

// Ends returns the two ends of the chain.
func (e Edge) Ends() (start, end geom.Vec3) { return e.start, e.end }

// Normals returns the outward normals of the two faces of the edge.
func (e Edge) Normals() [2]geom.Vec3 { return e.normal }

// Same reports whether two references resolved to one chain.
func (e Edge) Same(other Edge) bool {
	if len(e.chain) != len(other.chain) {
		return false
	}
	for _, x := range e.chain {
		if !slices.Contains(other.chain, x) {
			return false
		}
	}
	return true
}

// Meets reports whether two chains have a vertex of the solid in common.
func (e Edge) Meets(other Edge) bool {
	for _, v := range e.vertices {
		if slices.Contains(other.vertices, v) {
			return true
		}
	}
	return false
}
