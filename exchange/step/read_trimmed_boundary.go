package step

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"plainsolid/brep/meeting"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ReadTrimmedBoundary reads one MANIFOLD_SOLID_BREP whose faces lie on planes
// and circular cylinders and whose edges are lines, circles, ellipses and
// the curves in which two cylinders meet, the class that
// WriteTrimmedBoundary writes, and builds the trimmed solid
// its entities state. Nothing about the arrangement of the faces is
// assumed, and nothing is regenerated and compared: each surface, curve,
// vertex, edge, loop and face is taken from the file, and the solid is
// accepted by trimmed.NewSolid within the tolerance given.
//
// The file gives each edge as a curve in space. The parameters of an edge
// on its faces are worked out here: on a plane the coordinates in the
// plane's frame, on a cylinder the angle about the axis and the height,
// followed around each loop so that a face that winds once around its
// cylinder is cut where its own seam runs. A p-curve that the file
// attaches to an edge is not read.
//
// A file outside that class returns an error that wraps
// ErrTrimmedBoundary: another kind of surface or curve, a vertex loop, an
// assembly or more than one solid, a void, an arc of more than half a turn
// (a rim written as one closed edge is one), a loop that goes around its
// cylinder without a seam to cut the face, or an edge that does not lie on
// a face that uses it.
//
// An edge on an INTERSECTION_CURVE whose curve in space is a B-spline is
// where the two cylinders it names meet. It is given that curve itself
// (meeting.Curve), which the two cylinders fix; the spline says which of
// their curves it is and which way it runs, and must be within the
// tolerance of it.
func ReadTrimmedBoundary(r io.Reader, l Limits, tolerance float64) (TrimmedModel, error) {
	l, err := l.defaults()
	if err != nil {
		return TrimmedModel{}, err
	}
	if !(tolerance > 0) || math.IsInf(tolerance, 0) {
		return TrimmedModel{}, geom.ErrInvalid
	}
	f, err := Parse(r, l)
	if err != nil {
		return TrimmedModel{}, err
	}
	var representation, solid int64
	for _, id := range f.Order {
		for _, rec := range f.Entities[id].Records {
			switch {
			case strings.Contains(rec.Type, "TRANSFORMATION"), strings.Contains(rec.Type, "REPRESENTATION_RELATIONSHIP"),
				rec.Type == "MAPPED_ITEM", rec.Type == "REPRESENTATION_MAP", rec.Type == "NEXT_ASSEMBLY_USAGE_OCCURRENCE":
				return TrimmedModel{}, fmt.Errorf("%w: an assembly placement (%s)", ErrTrimmedBoundary, rec.Type)
			case strings.HasSuffix(rec.Type, "SHAPE_REPRESENTATION") && (rec.Type != "ADVANCED_BREP_SHAPE_REPRESENTATION" || representation != 0):
				return TrimmedModel{}, fmt.Errorf("%w: a representation other than one ADVANCED_BREP_SHAPE_REPRESENTATION", ErrTrimmedBoundary)
			case rec.Type == "ADVANCED_BREP_SHAPE_REPRESENTATION":
				representation = id
			case rec.Type == "MANIFOLD_SOLID_BREP" && solid == 0:
				solid = id
			case strings.HasSuffix(rec.Type, "_BREP"), rec.Type == "BREP_WITH_VOIDS":
				return TrimmedModel{}, fmt.Errorf("%w: more than one solid, or a %s", ErrTrimmedBoundary, rec.Type)
			}
		}
	}
	if representation == 0 || solid == 0 {
		return TrimmedModel{}, fmt.Errorf("%w: no advanced boundary representation of a solid", ErrTrimmedBoundary)
	}
	rep, err := simple(f, representation, "ADVANCED_BREP_SHAPE_REPRESENTATION", 3)
	if err != nil {
		return TrimmedModel{}, err
	}
	items, err := listRefs(rep.Args[1], 1)
	if err != nil {
		return TrimmedModel{}, err
	}
	held := false
	for _, id := range items {
		if id == solid {
			held = true
		} else if _, err := placement(f, id); err != nil {
			return TrimmedModel{}, fmt.Errorf("%w: representation item #%d is neither the solid nor a placement", ErrTrimmedBoundary, id)
		}
	}
	if !held {
		return TrimmedModel{}, fmt.Errorf("%w: the solid is not an item of the representation", ErrTrimmedBoundary)
	}
	context, err := ref(rep.Args[2])
	if err != nil {
		return TrimmedModel{}, err
	}
	unit, err := contextUnits(f, context)
	if err != nil {
		return TrimmedModel{}, err
	}
	body, err := simple(f, solid, "MANIFOLD_SOLID_BREP", 2)
	if err != nil {
		return TrimmedModel{}, err
	}
	name, err := textValue(body.Args[0])
	if err != nil {
		return TrimmedModel{}, err
	}
	shellID, err := ref(body.Args[1])
	if err != nil {
		return TrimmedModel{}, err
	}
	shell, err := simple(f, shellID, "CLOSED_SHELL", 2)
	if err != nil {
		return TrimmedModel{}, err
	}
	faceIDs, err := listRefs(shell.Args[1], 1)
	if err != nil {
		return TrimmedModel{}, err
	}
	if len(faceIDs) > l.MaxFaces {
		return TrimmedModel{}, errors.New("step: trimmed topology limit")
	}
	b := boundaryReader{file: f, limits: l, tolerance: tolerance, vertexOf: map[int64]int{}, edgeOf: map[int64]int{}}
	faces := make([]trimmed.Face, len(faceIDs))
	for i, id := range faceIDs {
		if faces[i], err = b.face(id); err != nil {
			return TrimmedModel{}, fmt.Errorf("face #%d: %w", id, err)
		}
	}
	edges := make([]parametric.Edge, len(b.edges))
	for i, e := range b.edges {
		edges[i] = e.Edge
	}
	out, err := trimmed.NewSolid(b.vertices, edges, faces, tolerance)
	if err != nil {
		return TrimmedModel{}, err
	}
	return TrimmedModel{LengthUnit: unit, Bodies: []TrimmedBody{{Name: name, Solid: out}}}, nil
}

// boundaryAngle bounds the sine of an angle that counts as zero where the
// reader asks whether two directions are parallel or perpendicular.
const boundaryAngle = 1e-9

type boundaryReader struct {
	file      *File
	limits    Limits
	tolerance float64
	vertices  []geom.Vec3
	vertexOf  map[int64]int
	edges     []boundaryEdge
	edgeOf    map[int64]int
	uses      int
}

// boundaryEdge is an edge as the solid stores it, with its curve in the
// form the charts need. The edge runs from its start to its end as the
// parameter grows; a curve that the file runs the other way is turned over.
// A conic is centre + a·cos t·x + b·sin t·y in its frame. An edge where
// two cylinders meet has met set, and meet is its curve.
type boundaryEdge struct {
	parametric.Edge
	line  bool
	frame geom.Frame // of a conic
	a, b  float64
	met   bool
	meet  meeting.Curve
}

func (b *boundaryReader) vertex(id int64) (int, error) {
	if i, ok := b.vertexOf[id]; ok {
		return i, nil
	}
	v, err := simple(b.file, id, "VERTEX_POINT", 2)
	if err != nil {
		return 0, err
	}
	pointID, err := ref(v.Args[1])
	if err != nil {
		return 0, err
	}
	p, err := triple(b.file, pointID, "CARTESIAN_POINT")
	if err != nil {
		return 0, err
	}
	if len(b.vertices) >= b.limits.MaxVertices {
		return 0, errors.New("step: trimmed topology limit")
	}
	b.vertexOf[id] = len(b.vertices)
	b.vertices = append(b.vertices, p)
	return b.vertexOf[id], nil
}

// edge reads an EDGE_CURVE. It runs from its first vertex to its second
// along its curve, with the curve when same_sense is true and against it
// otherwise, which on a closed curve says which of the two arcs between
// the vertices is meant.
func (b *boundaryReader) edge(id int64) (int, error) {
	if i, ok := b.edgeOf[id]; ok {
		return i, nil
	}
	r, err := simple(b.file, id, "EDGE_CURVE", 5)
	if err != nil {
		return 0, err
	}
	var ends [2]int
	for k := range ends {
		vid, err := ref(r.Args[1+k])
		if err != nil {
			return 0, err
		}
		if ends[k], err = b.vertex(vid); err != nil {
			return 0, err
		}
	}
	curve, err := ref(r.Args[3])
	if err != nil {
		return 0, err
	}
	same, err := boolean(r.Args[4])
	if err != nil {
		return 0, err
	}
	// A curve may come wrapped with its p-curves. The curve in space is
	// the one that is read.
	kind := b.kind(curve)
	var between []int64 // what an intersection curve is between
	if kind == "SURFACE_CURVE" || kind == "SEAM_CURVE" || kind == "INTERSECTION_CURVE" {
		wrapped, err := simple(b.file, curve, kind, 4)
		if err != nil {
			return 0, err
		}
		if kind == "INTERSECTION_CURVE" {
			if between, err = listRefs(wrapped.Args[2], 1); err != nil {
				return 0, err
			}
		}
		if curve, err = ref(wrapped.Args[1]); err != nil {
			return 0, err
		}
		kind = b.kind(curve)
	}
	out := boundaryEdge{Edge: parametric.Edge{Start: ends[0], End: ends[1]}}
	start, end := b.vertices[ends[0]], b.vertices[ends[1]]
	switch kind {
	case "LINE":
		origin, along, err := lineGeometry(b.file, curve, 3)
		if err != nil {
			return 0, err
		}
		if !same {
			along = along.Scale(-1)
		}
		line, err := geom.NewLine(origin, along)
		if err != nil {
			return 0, err
		}
		out.line, out.Curve = true, line
		out.Range = parametric.Interval{Min: start.Sub(origin).Dot(along), Max: end.Sub(origin).Dot(along)}
		if ends[0] == ends[1] || !(out.Range.Min < out.Range.Max) {
			return 0, fmt.Errorf("edge #%d does not run along its line: %w", id, geom.ErrInvalid)
		}
	case "CIRCLE", "ELLIPSE":
		args := 3
		if kind == "ELLIPSE" {
			args = 4
		}
		c, err := simple(b.file, curve, kind, args)
		if err != nil {
			return 0, err
		}
		place, err := ref(c.Args[1])
		if err != nil {
			return 0, err
		}
		if out.frame, err = placement(b.file, place); err != nil {
			return 0, err
		}
		if out.a, err = numeric(c.Args[2]); err != nil {
			return 0, err
		}
		if out.b = out.a; kind == "ELLIPSE" {
			if out.b, err = numeric(c.Args[3]); err != nil {
				return 0, err
			}
		}
		if !(out.a > 0 && out.b > 0) {
			return 0, fmt.Errorf("edge #%d: radii %g and %g: %w", id, out.a, out.b, geom.ErrInvalid)
		}
		if !same {
			// The same conic, turning the other way: its y axis and its
			// normal are reversed.
			if out.frame, err = geom.NewFrame(out.frame.Origin(), out.frame.XAxis(), out.frame.ZAxis().Scale(-1)); err != nil {
				return 0, err
			}
		}
		ellipse, err := geom.NewEllipse(out.frame, out.a, out.b)
		if err != nil {
			return 0, err
		}
		angle := func(p geom.Vec3) float64 {
			q := out.frame.ToLocal(p)
			return math.Atan2(q.Y/out.b, q.X/out.a)
		}
		out.Curve = ellipse
		out.Range = parametric.Interval{Min: angle(start), Max: angle(end)}
		if ends[0] == ends[1] || out.Range.Max <= out.Range.Min {
			out.Range.Max += 2 * math.Pi
		}
		// The solid is built with the edges the file states. trimmed.NewSolid
		// judges a loop by its vertices, which an arc of more than half a
		// turn, or a rim that is one closed edge, does not give it; such an
		// arc would have to be cut, and the reader adds no vertex.
		if out.Range.Max-out.Range.Min > math.Pi*(1+boundaryAngle) {
			return 0, fmt.Errorf("%w: edge #%d is an arc of more than half a turn, which the reader does not cut", ErrTrimmedBoundary, id)
		}
	default:
		if between == nil {
			return 0, fmt.Errorf("%w: edge #%d has a curve of kind %q", ErrTrimmedBoundary, id, kind)
		}
		if err := b.met(id, curve, between, same, &out); err != nil {
			return 0, err
		}
	}
	for k, t := range [2]float64{out.Range.Min, out.Range.Max} {
		p, err := out.Curve.Eval(t)
		if err != nil {
			return 0, err
		}
		if p.Distance(b.vertices[ends[k]]) > b.tolerance {
			return 0, fmt.Errorf("edge #%d: a vertex is off its curve: %w", id, geom.ErrInvalid)
		}
	}
	b.edgeOf[id] = len(b.edges)
	b.edges = append(b.edges, out)
	return b.edgeOf[id], nil
}

// met reads an edge on an INTERSECTION_CURVE whose curve in space is not a
// line or a conic: the curve in which two cylinders meet. The cylinders
// are the two items the entity names, each a surface or a p-curve on one.
// Of the curves they meet in, the edge has the one that 17 points of the
// spline are within the tolerance of, in the direction those points run
// along the edge, from the parameter of its start vertex to that of its
// end vertex.
func (b *boundaryReader) met(id, curve int64, between []int64, same bool, out *boundaryEdge) error {
	if len(between) != 2 {
		return fmt.Errorf("%w: edge #%d is on an intersection curve that does not name two surfaces", ErrTrimmedBoundary, id)
	}
	var frames [2]geom.Frame
	var radii [2]float64
	for k, sid := range between {
		if b.kind(sid) == "PCURVE" {
			on, err := simple(b.file, sid, "PCURVE", 3)
			if err != nil {
				return err
			}
			if sid, err = ref(on.Args[1]); err != nil {
				return err
			}
		}
		if kind := b.kind(sid); kind != "CYLINDRICAL_SURFACE" {
			return fmt.Errorf("%w: edge #%d is where a surface of kind %q meets another, and only two cylinders are read", ErrTrimmedBoundary, id, kind)
		}
		var err error
		if frames[k], radii[k], err = b.cylinder(sid); err != nil {
			return err
		}
	}
	spline, err := DecodeBSplineCurve(b.file, curve, b.limits.MaxVertices)
	if err != nil {
		return fmt.Errorf("%w: edge #%d has a curve of kind %q", ErrTrimmedBoundary, id, b.kind(curve))
	}
	curves, reason := meeting.Between(frames[0], radii[0], frames[1], radii[1], boundaryAngle, b.tolerance)
	if len(curves) == 0 {
		if reason == "" {
			reason = "that are apart"
		}
		return fmt.Errorf("%w: edge #%d is where two cylinders %s meet", ErrTrimmedBoundary, id, reason)
	}
	const samples = 17
	lo, hi := spline.Geometry.Domain()
	points := make([]geom.Vec3, samples)
	for k := range points {
		u := float64(k) / (samples - 1)
		if !same {
			u = 1 - u
		}
		if points[k], err = spline.Geometry.Eval(lo + (hi-lo)*u); err != nil {
			return err
		}
	}
	found, least := 0, math.Inf(1)
	for _, c := range curves {
		worst := 0.
		for _, p := range points {
			q, _ := c.Eval(c.Param(p))
			worst = math.Max(worst, q.Distance(p))
		}
		if least = math.Min(least, worst); worst <= b.tolerance {
			out.meet = c
			found++
		}
	}
	if found != 1 {
		return fmt.Errorf("edge #%d: its spline is %g from where its two cylinders meet: %w", id, least, geom.ErrInvalid)
	}
	advance := 0.
	for k := 0; k+1 < samples; k++ {
		advance += math.Remainder(out.meet.Param(points[k+1])-out.meet.Param(points[k]), 2*math.Pi)
	}
	if advance < 0 {
		out.meet = out.meet.Reversed()
	}
	out.met, out.Curve = true, out.meet
	out.Range = parametric.Interval{Min: out.meet.Param(b.vertices[out.Start]), Max: out.meet.Param(b.vertices[out.End])}
	if out.Start == out.End || out.Range.Max <= out.Range.Min {
		out.Range.Max += 2 * math.Pi
	}
	// As for an arc of a conic: the reader adds no vertex.
	if out.Range.Max-out.Range.Min > math.Pi*(1+boundaryAngle) {
		return fmt.Errorf("%w: edge #%d is an arc of more than half a turn, which the reader does not cut", ErrTrimmedBoundary, id)
	}
	return nil
}

// cylinder reads the frame and the radius of a CYLINDRICAL_SURFACE.
func (b *boundaryReader) cylinder(id int64) (geom.Frame, float64, error) {
	s, err := simple(b.file, id, "CYLINDRICAL_SURFACE", 3)
	if err != nil {
		return geom.Frame{}, 0, err
	}
	place, err := ref(s.Args[1])
	if err != nil {
		return geom.Frame{}, 0, err
	}
	frame, err := placement(b.file, place)
	if err != nil {
		return geom.Frame{}, 0, err
	}
	radius, err := numeric(s.Args[2])
	return frame, radius, err
}

func (b *boundaryReader) kind(id int64) string {
	if e, ok := b.file.Entities[id]; ok && len(e.Records) == 1 {
		return e.Records[0].Type
	}
	return ""
}

// face reads an ADVANCED_FACE. The normal of the face is the normal of
// its surface when same_sense is true and the opposite otherwise, and each
// bound, taken in the sense its own flag gives, runs counterclockwise about
// that normal. The solid stores every loop counterclockwise about the
// surface normal, so a loop is kept as written when the two flags agree
// and is turned around when they differ.
func (b *boundaryReader) face(id int64) (trimmed.Face, error) {
	r, err := simple(b.file, id, "ADVANCED_FACE", 4)
	if err != nil {
		return trimmed.Face{}, err
	}
	bounds, err := listRefs(r.Args[1], 1)
	if err != nil {
		return trimmed.Face{}, err
	}
	surfaceID, err := ref(r.Args[2])
	if err != nil {
		return trimmed.Face{}, err
	}
	same, err := boolean(r.Args[3])
	if err != nil {
		return trimmed.Face{}, err
	}
	var chart boundaryChart
	out := trimmed.Face{Reversed: !same}
	switch kind := b.kind(surfaceID); kind {
	case "PLANE":
		frame, err := planeFrame(b.file, surfaceID)
		if err != nil {
			return trimmed.Face{}, err
		}
		if out.Surface, err = geom.NewPlaneSurface(frame); err != nil {
			return trimmed.Face{}, err
		}
		chart = boundaryChart{frame: frame}
	case "CYLINDRICAL_SURFACE":
		frame, radius, err := b.cylinder(surfaceID)
		if err != nil {
			return trimmed.Face{}, err
		}
		if out.Surface, err = geom.NewCylinderSurface(frame, radius); err != nil {
			return trimmed.Face{}, err
		}
		chart = boundaryChart{frame: frame, radius: radius}
	default:
		return trimmed.Face{}, fmt.Errorf("%w: a surface of kind %q", ErrTrimmedBoundary, kind)
	}
	chart.tolerance = b.tolerance
	var loops [][]boundaryUse
	for _, boundID := range bounds {
		kind := b.kind(boundID)
		if kind != "FACE_BOUND" && kind != "FACE_OUTER_BOUND" {
			return trimmed.Face{}, fmt.Errorf("%w: a bound of kind %q", ErrTrimmedBoundary, kind)
		}
		bound, err := simple(b.file, boundID, kind, 3)
		if err != nil {
			return trimmed.Face{}, err
		}
		loopID, err := ref(bound.Args[1])
		if err != nil {
			return trimmed.Face{}, err
		}
		forward, err := boolean(bound.Args[2])
		if err != nil {
			return trimmed.Face{}, err
		}
		if kind := b.kind(loopID); kind != "EDGE_LOOP" {
			return trimmed.Face{}, fmt.Errorf("%w: a loop of kind %q", ErrTrimmedBoundary, kind)
		}
		loop, err := simple(b.file, loopID, "EDGE_LOOP", 2)
		if err != nil {
			return trimmed.Face{}, err
		}
		oriented, err := listRefs(loop.Args[1], 1)
		if err != nil {
			return trimmed.Face{}, err
		}
		if b.uses += len(oriented); b.uses > b.limits.MaxEdgeUses {
			return trimmed.Face{}, errors.New("step: trimmed topology limit")
		}
		uses := make([]boundaryUse, len(oriented))
		for i, oid := range oriented {
			o, err := simple(b.file, oid, "ORIENTED_EDGE", 5)
			if err != nil {
				return trimmed.Face{}, err
			}
			edgeID, err := ref(o.Args[3])
			if err != nil {
				return trimmed.Face{}, err
			}
			with, err := boolean(o.Args[4])
			if err != nil {
				return trimmed.Face{}, err
			}
			index, err := b.edge(edgeID)
			if err != nil {
				return trimmed.Face{}, err
			}
			at := i
			if forward != same {
				at, with = len(oriented)-1-i, !with
			}
			uses[at] = boundaryUse{edge: b.edges[index], index: index, reversed: !with}
		}
		for i, u := range uses {
			if next := uses[(i+1)%len(uses)]; u.to() != next.from() {
				return trimmed.Face{}, fmt.Errorf("a loop does not close vertex to vertex: %w", geom.ErrInvalid)
			}
		}
		loops = append(loops, uses)
	}
	// Each loop is laid out in the chart of the surface. The loop that
	// runs counterclockwise there is the outer one, whatever kind of bound
	// the file calls it; on a cylinder the others are then moved by whole
	// turns into the turn of the outer one.
	laid := make([]boundaryLoop, len(loops))
	outer := -1
	for i, loop := range loops {
		if laid[i], err = chart.lay(b.vertices, loop, 0); err != nil {
			return trimmed.Face{}, err
		}
		if laid[i].area > 0 {
			if outer >= 0 {
				return trimmed.Face{}, fmt.Errorf("two loops of one face run counterclockwise: %w", geom.ErrInvalid)
			}
			outer = i
		}
	}
	if outer < 0 {
		return trimmed.Face{}, fmt.Errorf("no loop of the face runs counterclockwise: %w", geom.ErrInvalid)
	}
	out.Outer = laid[outer].uses
	for i, loop := range loops {
		if i == outer {
			continue
		}
		if chart.radius > 0 {
			turns := math.Round(((laid[outer].lo+laid[outer].hi)/2 - laid[i].uses[0].UVStart.X) / (2 * math.Pi))
			if laid[i], err = chart.lay(b.vertices, loop, turns); err != nil {
				return trimmed.Face{}, err
			}
		}
		out.Holes = append(out.Holes, laid[i].uses)
	}
	return out, nil
}

// boundaryUse is one traversal of an edge by a loop, in the direction the
// solid stores the loop.
type boundaryUse struct {
	edge     boundaryEdge
	index    int
	reversed bool
}

func (u boundaryUse) from() int {
	if u.reversed {
		return u.edge.End
	}
	return u.edge.Start
}

func (u boundaryUse) to() int {
	if u.reversed {
		return u.edge.Start
	}
	return u.edge.End
}

// boundaryChart is the parameter plane of a face surface as geom evaluates
// it: on a plane the x and y of the frame, on a cylinder (radius above
// zero) the angle about the z axis from the x axis, and the height.
type boundaryChart struct {
	frame     geom.Frame
	radius    float64
	tolerance float64
}

// boundaryLoop is a loop laid out in a chart: its uses, twice the area it
// encloses, positive when it runs counterclockwise, and the extent of its
// first parameter.
type boundaryLoop struct {
	uses   []trimmed.Use
	area   float64
	lo, hi float64
}

// lay gives the uses of a loop their parameters. On a cylinder the angle is
// followed from use to use, starting from the angle of the first vertex
// plus the given number of turns, so each use starts where the one before
// it ended; a loop that does not come back to its first angle goes around
// the cylinder, and a face bounded by such loops has no seam to be cut at.
//
// On a cylinder every use starts and ends at the angle the loop has
// reached and at the height of its vertex, so the uses that meet at a
// vertex have one point there and a ruling has one angle. An arc's own
// evaluation at its ends differs from that point by rounding, which
// trimmed.NewSolid allows; taken as the point, it tilted the ruling next
// to it by a unit in the last place, and the validation mesh folded on
// the sliver.
func (c boundaryChart) lay(vertices []geom.Vec3, loop []boundaryUse, turns float64) (boundaryLoop, error) {
	out := boundaryLoop{uses: make([]trimmed.Use, len(loop))}
	first := c.frame.ToLocal(vertices[loop[0].from()])
	angle := math.Atan2(first.Y, first.X) + 2*math.Pi*turns
	begin := angle
	out.lo, out.hi = angle, angle
	var polygon []geom.Vec2
	pinned := make([][2]geom.Vec2, len(loop))
	level := make([]*float64, len(loop)) // of a use that keeps one height
	for i, u := range loop {
		use := trimmed.Use{Edge: u.index, Reversed: u.reversed}
		from, to := c.frame.ToLocal(vertices[u.from()]), c.frame.ToLocal(vertices[u.to()])
		for _, p := range [2]geom.Vec3{from, to} {
			off := math.Abs(p.Z)
			if c.radius > 0 {
				off = math.Abs(math.Hypot(p.X, p.Y) - c.radius)
			}
			if off > c.tolerance {
				return out, fmt.Errorf("%w: a vertex of edge %d is off the surface of a face that uses it", ErrTrimmedBoundary, u.index)
			}
		}
		t0, t1 := u.edge.Range.Min, u.edge.Range.Max
		if u.reversed {
			t0, t1 = t1, t0
		}
		pinned[i][0].X = angle
		switch {
		case u.edge.met:
			// Where two cylinders meet: on a face of either, at the angle
			// the curve itself gives about that axis, taken in the turn
			// the loop has reached.
			for _, t := range [3]float64{t0, t0 + (t1-t0)/2, t1} {
				p, _ := u.edge.meet.Eval(t)
				if q := c.frame.ToLocal(p); c.radius == 0 || math.Abs(math.Hypot(q.X, q.Y)-c.radius) > c.tolerance {
					return out, fmt.Errorf("%w: edge %d, where two cylinders meet, does not lie on a face that uses it", ErrTrimmedBoundary, u.index)
				}
			}
			arc := u.edge.meet.Trim(c.frame, t0, t1, angle)
			end, _ := arc.EvalUV(1)
			use.Curve = arc
			pinned[i][1].X = end.X
			angle = end.X
		case c.radius == 0 && u.edge.line:
			use.UVStart, use.UVEnd = geom.Vec2{X: from.X, Y: from.Y}, geom.Vec2{X: to.X, Y: to.Y}
		case c.radius == 0:
			// A conic in the plane: its own frame, seen in the frame of
			// the plane.
			f := u.edge.frame
			centre := c.frame.ToLocal(f.Origin())
			if math.Abs(centre.Z) > c.tolerance || f.ZAxis().Cross(c.frame.ZAxis()).Length() > boundaryAngle {
				return out, fmt.Errorf("%w: the conic of edge %d does not lie in the plane of a face that uses it", ErrTrimmedBoundary, u.index)
			}
			flat := func(v geom.Vec3) geom.Vec2 { return geom.Vec2{X: v.Dot(c.frame.XAxis()), Y: v.Dot(c.frame.YAxis())} }
			use.Curve = trimmed.EllipseUV{
				Center: geom.Vec2{X: centre.X, Y: centre.Y},
				Cos:    flat(f.XAxis().Scale(u.edge.a)), Sin: flat(f.YAxis().Scale(u.edge.b)),
				From: t0, To: t1,
			}
		case u.edge.line:
			// A line on a cylinder is a ruling: it keeps its angle.
			if line, ok := u.edge.Curve.(geom.Line); !ok || line.Direction().Cross(c.frame.ZAxis()).Length() > boundaryAngle {
				return out, fmt.Errorf("%w: the line of edge %d is not a ruling of a cylinder that uses it", ErrTrimmedBoundary, u.index)
			}
			use.UVStart, use.UVEnd = geom.Vec2{X: angle, Y: from.Z}, geom.Vec2{X: angle, Y: to.Z}
		default:
			// A conic on a cylinder is a section by a plane across the
			// axis: its centre is on the axis, and seen along the axis it
			// is the circle of the cylinder, which it goes around at the
			// rate of its own parameter, one way or the other.
			f := u.edge.frame
			centre := c.frame.ToLocal(f.Origin())
			x, y := c.frame.ToLocal(f.Origin().Add(f.XAxis().Scale(u.edge.a))).Sub(centre), c.frame.ToLocal(f.Origin().Add(f.YAxis().Scale(u.edge.b))).Sub(centre)
			if math.Hypot(centre.X, centre.Y) > c.tolerance || math.Abs(math.Hypot(x.X, x.Y)-c.radius) > c.tolerance || math.Abs(math.Hypot(y.X, y.Y)-c.radius) > c.tolerance ||
				math.Abs(x.X*y.X+x.Y*y.Y) > c.tolerance*c.radius {
				return out, fmt.Errorf("%w: the conic of edge %d does not lie on a cylinder that uses it", ErrTrimmedBoundary, u.index)
			}
			rate := 1.
			if x.X*y.Y-x.Y*y.X < 0 {
				rate = -1
			}
			base := math.Atan2(x.Y, x.X)
			// The arc is written at angles a whole number of turns on, so
			// that it starts at the angle the loop has reached.
			shift := 2 * math.Pi * math.Round((angle-(base+rate*t0))/(2*math.Pi)) * rate
			t0, t1 = t0+shift, t1+shift
			end := base + rate*t1
			if flat := c.tolerance * 1e-3; math.Abs(x.Z) <= flat && math.Abs(y.Z) <= flat {
				// A section across the axis at one height is straight in
				// the chart.
				use.UVStart, use.UVEnd = geom.Vec2{X: angle, Y: from.Z}, geom.Vec2{X: end, Y: to.Z}
				level[i] = &centre.Z
			} else {
				use.Curve = trimmed.EllipseUV{
					Center: geom.Vec2{X: base, Y: centre.Z}, Cos: geom.Vec2{Y: x.Z}, Sin: geom.Vec2{Y: y.Z}, Linear: geom.Vec2{X: rate},
					From: t0, To: t1,
				}
			}
			pinned[i][1].X = end
			angle = end
		}
		if c.radius > 0 {
			if u.edge.line {
				pinned[i][1].X = pinned[i][0].X
			}
			pinned[i][0].Y, pinned[i][1].Y = from.Z, to.Z
		}
		if arc := use.Curve; arc != nil {
			var err error
			if use.UVStart, err = arc.EvalUV(0); err != nil {
				return out, err
			}
			if use.UVEnd, err = arc.EvalUV(1); err != nil {
				return out, err
			}
			for k := 0; k < 8; k++ {
				p, err := arc.EvalUV(float64(k) / 8)
				if err != nil {
					return out, err
				}
				polygon = append(polygon, p)
			}
		} else {
			polygon = append(polygon, use.UVStart)
		}
		out.uses[i] = use
	}
	if c.radius > 0 && math.Abs(angle-begin) > math.Pi {
		return out, fmt.Errorf("%w: a loop goes around its cylinder, and the face has no seam", ErrTrimmedBoundary)
	}
	if c.radius > 0 {
		// The angle comes back to where it began only to within rounding.
		// The loop closes at the first angle: the rulings that end the
		// loop are at it, and the use before them ends at it.
		for i := len(loop) - 1; i >= 0; i-- {
			pinned[i][1].X = begin
			if !loop[i].edge.line {
				break
			}
			pinned[i][0].X = begin
		}
		// A section across the axis has one height, that of its centre,
		// and so have the vertices along it. Each taken from its own
		// vertex, they differ in the last bits under a frame off the
		// coordinate axes, and the corners along one rim are then not in
		// line in the chart.
		for i, h := range level {
			if h != nil {
				before, after := (i+len(loop)-1)%len(loop), (i+1)%len(loop)
				pinned[i][0].Y, pinned[i][1].Y, pinned[before][1].Y, pinned[after][0].Y = *h, *h, *h, *h
			}
		}
		for i := range out.uses {
			out.uses[i].UVStart, out.uses[i].UVEnd = pinned[i][0], pinned[i][1]
		}
	}
	// The use after an arc starts where the arc ends, and every use ends
	// where the next one starts; trimmed.NewSolid takes an arc whose own
	// ends are within its tolerance of those.
	for i := range out.uses {
		if out.uses[i].Curve != nil {
			out.uses[(i+1)%len(out.uses)].UVStart = out.uses[i].UVEnd
		}
	}
	for i := range out.uses {
		out.uses[i].UVEnd = out.uses[(i+1)%len(out.uses)].UVStart
		if arc, ok := out.uses[i].Curve.(meeting.Trim); ok {
			out.uses[i].Curve = arc.Pinned(out.uses[i].UVStart, out.uses[i].UVEnd)
		}
	}
	for i, p := range polygon {
		q := polygon[(i+1)%len(polygon)]
		out.area += p.X*q.Y - p.Y*q.X
		out.lo, out.hi = math.Min(out.lo, p.X), math.Max(out.hi, p.X)
	}
	return out, nil
}
