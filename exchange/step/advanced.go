package step

import (
	"errors"
	"math"

	"plainsolid/brep"
	"plainsolid/geom"
)

type linearEdge struct{ startVertex, endVertex, startPoint, endPoint int64 }
type linearLoop struct{ points, vertices, edges []int64 }

func boolean(v Value) (bool, error) {
	if v.Kind != Enumeration || (v.Text != "T" && v.Text != "F") {
		return false, errors.New("step: expected boolean")
	}
	return v.Text == "T", nil
}
func placement(f *File, id int64) (geom.Frame, error) {
	r, e := simple(f, id, "AXIS2_PLACEMENT_3D", 4)
	if e != nil {
		return geom.Frame{}, e
	}
	if _, e = textValue(r.Args[0]); e != nil {
		return geom.Frame{}, e
	}
	pointID, e := ref(r.Args[1])
	if e != nil {
		return geom.Frame{}, e
	}
	origin, e := triple(f, pointID, "CARTESIAN_POINT")
	if e != nil {
		return geom.Frame{}, e
	}
	normal, e := direction(f, r.Args[2], geom.Vec3{Z: 1})
	if e != nil {
		return geom.Frame{}, e
	}
	fallback := geom.Vec3{X: 1}
	if math.Abs(normal.X) > 1-1e-12 {
		fallback = geom.Vec3{Y: 1}
	}
	x, e := direction(f, r.Args[3], fallback)
	if e != nil {
		return geom.Frame{}, e
	}
	return geom.NewFrame(origin, x, normal)
}
func planeFrame(f *File, id int64) (geom.Frame, error) {
	r, e := simple(f, id, "PLANE", 2)
	if e != nil {
		return geom.Frame{}, e
	}
	if _, e = textValue(r.Args[0]); e != nil {
		return geom.Frame{}, e
	}
	id, e = ref(r.Args[1])
	if e != nil {
		return geom.Frame{}, e
	}
	return placement(f, id)
}
func coordinate(f *File, id int64, kind string, dimension int) (geom.Vec3, error) {
	r, e := simple(f, id, kind, 2)
	if e != nil {
		return geom.Vec3{}, e
	}
	if _, e = textValue(r.Args[0]); e != nil {
		return geom.Vec3{}, e
	}
	v := r.Args[1]
	if v.Kind != Aggregate || len(v.Items) != dimension {
		return geom.Vec3{}, errors.New("step: wrong coordinate dimension")
	}
	p := geom.Vec3{}
	a := []*float64{&p.X, &p.Y, &p.Z}
	for i := 0; i < dimension; i++ {
		*a[i], e = numeric(v.Items[i])
		if e != nil {
			return geom.Vec3{}, e
		}
	}
	return p, nil
}
func lineGeometry(f *File, id int64, dimension int) (geom.Vec3, geom.Vec3, error) {
	r, e := simple(f, id, "LINE", 3)
	if e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	if _, e = textValue(r.Args[0]); e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	pointID, e := ref(r.Args[1])
	if e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	origin, e := coordinate(f, pointID, "CARTESIAN_POINT", dimension)
	if e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	vectorID, e := ref(r.Args[2])
	if e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	v, e := simple(f, vectorID, "VECTOR", 3)
	if e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	if _, e = textValue(v.Args[0]); e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	directionID, e := ref(v.Args[1])
	if e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	d, e := coordinate(f, directionID, "DIRECTION", dimension)
	if e != nil {
		return geom.Vec3{}, geom.Vec3{}, e
	}
	magnitude, e := numeric(v.Args[2])
	if e != nil || magnitude <= 0 {
		return geom.Vec3{}, geom.Vec3{}, errors.New("step: invalid line vector magnitude")
	}
	d, e = d.Unit()
	return origin, d, e
}
func pointsOnLine(a, b, origin, direction geom.Vec3) error {
	delta := b.Sub(a)
	length := delta.Length()
	if length == 0 || math.IsInf(length, 0) || !delta.IsFinite() {
		return errors.New("step: invalid linear edge extent")
	}
	for _, p := range []geom.Vec3{a, b} {
		v := p.Sub(origin)
		distance := v.Cross(direction).Length()
		if !v.IsFinite() || !finiteNumber(distance) || distance/length > 1e-9 {
			return errors.New("step: edge endpoints disagree with line geometry")
		}
	}
	return nil
}
func finiteNumber(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func pcurveLine(f *File, id int64, a, b geom.Vec3, exactSupports bool) error {
	r, e := simple(f, id, "PCURVE", 3)
	if e != nil {
		return e
	}
	if _, e = textValue(r.Args[0]); e != nil {
		return e
	}
	surfaceID, e := ref(r.Args[1])
	if e != nil {
		return e
	}
	frame, e := planeFrame(f, surfaceID)
	if e != nil {
		return e
	}
	repID, e := ref(r.Args[2])
	if e != nil {
		return e
	}
	rep, e := simple(f, repID, "DEFINITIONAL_REPRESENTATION", 3)
	if e != nil {
		return e
	}
	if _, e = textValue(rep.Args[0]); e != nil {
		return e
	}
	ids, e := listRefs(rep.Args[1], 1)
	if e != nil || len(ids) != 1 {
		return errors.New("step: pcurve requires one planar line")
	}
	ctxID, e := ref(rep.Args[2])
	if e != nil {
		return e
	}
	ctx := f.Entities[ctxID]
	dimension, ok := component(ctx, "GEOMETRIC_REPRESENTATION_CONTEXT")
	if !ok || len(dimension.Args) != 1 {
		return errors.New("step: invalid pcurve context")
	}
	dim, e := numeric(dimension.Args[0])
	if e != nil || dim != 2 {
		return errors.New("step: pcurve requires dimension 2")
	}
	for _, c := range ctx.Records {
		switch c.Type {
		case "GEOMETRIC_REPRESENTATION_CONTEXT":
		case "PARAMETRIC_REPRESENTATION_CONTEXT":
			if len(c.Args) != 0 {
				return errors.New("step: invalid parametric context")
			}
		case "REPRESENTATION_CONTEXT":
			if len(c.Args) != 2 || c.Args[0].Kind != String || c.Args[1].Kind != String {
				return errors.New("step: invalid representation context")
			}
		default:
			return errors.New("step: unsupported pcurve context component")
		}
	}
	// Finite linear splines are accepted only by the exact support path.
	// It authenticates their knots, direction and full model edge coverage.
	if exactSupports && len(f.Entities[ids[0]].Records) == 1 && f.Entities[ids[0]].Records[0].Type == "B_SPLINE_CURVE_WITH_KNOTS" {
		return nil
	}
	p, d, e := lineGeometry(f, ids[0], 2)
	if e != nil {
		return e
	}
	origin := frame.ToWorld(p)
	direction, e := frame.XAxis().Scale(d.X).Add(frame.YAxis().Scale(d.Y)).Unit()
	if e != nil {
		return e
	}
	return pointsOnLine(a, b, origin, direction)
}
func validateEdgeGeometry(f *File, id int64, a, b geom.Vec3, sense, exactSupports bool) error {
	ent := f.Entities[id]
	if len(ent.Records) != 1 {
		return errors.New("step: complex edge geometry unsupported")
	}
	r := ent.Records[0]
	var pcurves []int64
	if r.Type == "SURFACE_CURVE" {
		if len(r.Args) != 4 {
			return errors.New("step: invalid surface curve")
		}
		if _, e := textValue(r.Args[0]); e != nil {
			return e
		}
		var e error
		id, e = ref(r.Args[1])
		if e != nil {
			return e
		}
		pcurves, e = listRefs(r.Args[2], 1)
		if e != nil {
			return e
		}
		if len(pcurves) > 2 {
			return errors.New("step: unsupported surface-curve associations")
		}
		v := r.Args[3]
		if v.Kind != Enumeration || (v.Text != "CURVE_3D" && v.Text != "PCURVE_S1" && v.Text != "PCURVE_S2") {
			return errors.New("step: invalid surface-curve master")
		}
		if v.Text == "PCURVE_S2" && len(pcurves) < 2 {
			return errors.New("step: missing second pcurve")
		}
	}
	origin, direction, e := lineGeometry(f, id, 3)
	if e != nil {
		return e
	}
	if e = pointsOnLine(a, b, origin, direction); e != nil {
		return e
	}
	unit, e := b.Sub(a).Unit()
	if e != nil {
		return e
	}
	dot := unit.Dot(direction)
	if !sense {
		dot = -dot
	}
	if dot < 1-1e-8 {
		return errors.New("step: edge sense disagrees with line")
	}
	for _, id := range pcurves {
		if e = pcurveLine(f, id, a, b, exactSupports); e != nil {
			return e
		}
	}
	return nil
}
func readLinearEdge(f *File, id int64, exactSupports bool) (linearEdge, error) {
	r, e := simple(f, id, "EDGE_CURVE", 5)
	if e != nil {
		return linearEdge{}, e
	}
	if _, e = textValue(r.Args[0]); e != nil {
		return linearEdge{}, e
	}
	var edge linearEdge
	edge.startVertex, e = ref(r.Args[1])
	if e != nil {
		return edge, e
	}
	edge.endVertex, e = ref(r.Args[2])
	if e != nil {
		return edge, e
	}
	if edge.startVertex == edge.endVertex {
		return edge, errors.New("step: closed/degenerate linear edge unsupported")
	}
	coords := [2]geom.Vec3{}
	for i, id := range []int64{edge.startVertex, edge.endVertex} {
		v, e := simple(f, id, "VERTEX_POINT", 2)
		if e != nil {
			return edge, e
		}
		if _, e = textValue(v.Args[0]); e != nil {
			return edge, e
		}
		pointID, e := ref(v.Args[1])
		if e != nil {
			return edge, e
		}
		coords[i], e = triple(f, pointID, "CARTESIAN_POINT")
		if e != nil {
			return edge, e
		}
		if i == 0 {
			edge.startPoint = pointID
		} else {
			edge.endPoint = pointID
		}
	}
	geometry, e := ref(r.Args[3])
	if e != nil {
		return edge, e
	}
	sense, e := boolean(r.Args[4])
	if e != nil {
		return edge, e
	}
	if e = validateEdgeGeometry(f, geometry, coords[0], coords[1], sense, exactSupports); e != nil {
		return edge, e
	}
	return edge, nil
}
func edgeLoop(f *File, id int64, cache map[int64]linearEdge) (linearLoop, error) {
	return edgeLoopWithExactSupport(f, id, cache, false)
}

func edgeLoopWithExactSupport(f *File, id int64, cache map[int64]linearEdge, exactSupports bool) (linearLoop, error) {
	r, e := simple(f, id, "EDGE_LOOP", 2)
	if e != nil {
		return linearLoop{}, e
	}
	if _, e = textValue(r.Args[0]); e != nil {
		return linearLoop{}, e
	}
	ids, e := listRefs(r.Args[1], 3)
	if e != nil {
		return linearLoop{}, e
	}
	result := linearLoop{}
	ends := []int64{}
	for _, id := range ids {
		r, e := simple(f, id, "ORIENTED_EDGE", 5)
		if e != nil {
			return result, e
		}
		if _, e = textValue(r.Args[0]); e != nil {
			return result, e
		}
		if r.Args[1].Kind != Derived || r.Args[2].Kind != Derived {
			return result, errors.New("step: oriented-edge endpoints must be derived")
		}
		edgeID, e := ref(r.Args[3])
		if e != nil {
			return result, e
		}
		edge, ok := cache[edgeID]
		if !ok {
			edge, e = readLinearEdge(f, edgeID, exactSupports)
			if e != nil {
				return result, e
			}
			cache[edgeID] = edge
		}
		sense, e := boolean(r.Args[4])
		if e != nil {
			return result, e
		}
		if !sense {
			edge.startVertex, edge.endVertex = edge.endVertex, edge.startVertex
			edge.startPoint, edge.endPoint = edge.endPoint, edge.startPoint
		}
		result.points = append(result.points, edge.startPoint)
		result.vertices = append(result.vertices, edge.startVertex)
		result.edges = append(result.edges, edgeID)
		ends = append(ends, edge.endVertex)
	}
	for i, end := range ends {
		if end != result.vertices[(i+1)%len(ends)] {
			return result, errors.New("step: edge loop does not close by vertex identity")
		}
	}
	return result, nil
}

func largestLoop(loops [][]brep.EdgeUse, vertices []geom.Vec3, edges []brep.Edge) (int, error) {
	if len(loops) == 0 {
		return 0, errors.New("step: missing face bounds")
	}
	point := func(u brep.EdgeUse) geom.Vec3 {
		e := edges[u.Edge]
		if u.Reversed {
			return vertices[e.End]
		}
		return vertices[e.Start]
	}
	origin := point(loops[0][0])
	scale := 0.
	for _, loop := range loops {
		for _, u := range loop {
			scale = math.Max(scale, point(u).Sub(origin).Length())
		}
	}
	if !finiteNumber(scale) || scale == 0 {
		return 0, errors.New("step: invalid face extent")
	}
	local := func(u brep.EdgeUse) geom.Vec3 {
		p := point(u).Sub(origin)
		return geom.Vec3{X: p.X / scale, Y: p.Y / scale, Z: p.Z / scale}
	}
	best, index := 0., -1
	for i, loop := range loops {
		sum := geom.Vec3{}
		for j, u := range loop {
			sum = sum.Add(local(u).Cross(local(loop[(j+1)%len(loop)])))
		}
		area := sum.Length()
		if area > best {
			best, index = area, i
		}
	}
	if index < 0 {
		return 0, errors.New("step: zero-area face loops")
	}
	return index, nil
}
