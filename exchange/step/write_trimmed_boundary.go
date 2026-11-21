package step

import (
	"errors"
	"fmt"
	"io"
	"math"
	"unicode/utf8"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

// ErrTrimmedBoundary is what WriteTrimmedBoundary returns, wrapped with the
// face or edge at fault, for a solid outside the class it writes.
var ErrTrimmedBoundary = errors.New("step: the boundary is outside the class of planes and circular cylinders with line, circle and ellipse edges and edges in which two cylinders meet")

// WriteTrimmedBoundary writes a solid bounded by planes and circular
// cylinders, with edges that are lines, circles and ellipses, as one
// MANIFOLD_SOLID_BREP: every face, loop, edge and vertex of the solid as it
// is stored, with the geometry of its own surfaces and edge curves. A
// placement of a surface or a curve must keep lengths; it may mirror.
//
// An edge with any other curve is written where its two faces are on two
// cylinders and it lies on both, which fixes it: as an INTERSECTION_CURVE
// on the two surfaces, with a cubic B-spline in space that is within half
// the tolerance of the solid of the stored curve (boundarySpline). The
// stored curve is only evaluated.
//
// Nothing about the arrangement of the faces is assumed: a face may have
// any number of loops, a loop any number of uses, and a face may use an edge
// twice, as a cylinder that winds once around its axis uses its seam. The
// file carries no p-curves; the edge curves in space define the trims.
//
// A solid with a face or an edge of another kind, a collapsed edge, a
// placement that does not keep lengths, or a boundary of more than one
// shell returns an error that wraps ErrTrimmedBoundary, and nothing is
// written.
func WriteTrimmedBoundary(w io.Writer, s trimmed.Solid, unit float64, name string) error {
	if w == nil || !utf8.ValidString(name) || len(name) > 4096 {
		return geom.ErrInvalid
	}
	vertices, edges, faces := s.Vertices(), s.Edges(), s.Faces()
	if len(faces) == 0 {
		return fmt.Errorf("%w: the solid has no face", ErrTrimmedBoundary)
	}
	// A point of the solid agrees with the geometry that is written when it
	// is within the tolerance of the solid, widened by what a placement may
	// differ from an exactly orthonormal one.
	reach := 0.
	for _, p := range vertices {
		reach = math.Max(reach, p.Length())
	}
	tolerance := s.Tolerance() + boundaryRigidSlack*math.Max(1, reach)

	curves := make([]boundaryCurve, len(edges))
	for i, e := range edges {
		if e.Start < 0 || e.End < 0 || e.Start >= len(vertices) || e.End >= len(vertices) {
			return geom.ErrInvalid
		}
		c, err := boundaryCurveOf(e)
		if err != nil {
			return fmt.Errorf("edge %d: %w", i, err)
		}
		if !(c.at(e.Range.Min).Distance(vertices[e.Start]) <= tolerance && c.at(e.Range.Max).Distance(vertices[e.End]) <= tolerance) {
			return fmt.Errorf("edge %d: its curve does not run from its start vertex to its end vertex: %w", i, geom.ErrInvalid)
		}
		curves[i] = c
	}
	surfaces := make([]boundarySurface, len(faces))
	uses := make([]int, len(edges))
	shell := make([]int, len(faces)) // the shell of each face, as a forest
	owner := make([]int, len(edges)) // one face that uses the edge, plus one
	sides := make([][]int, len(edges))
	root := func(i int) int {
		for shell[i] != i {
			shell[i] = shell[shell[i]]
			i = shell[i]
		}
		return i
	}
	for i, f := range faces {
		surface, err := boundarySurfaceOf(f.Surface)
		if err != nil {
			return fmt.Errorf("face %d: %w", i, err)
		}
		surfaces[i], shell[i] = surface, i
		for _, loop := range append([][]trimmed.Use{f.Outer}, f.Holes...) {
			if len(loop) == 0 {
				return fmt.Errorf("face %d: an empty loop: %w", i, geom.ErrInvalid)
			}
			for _, u := range loop {
				if u.Edge < 0 || u.Edge >= len(edges) {
					return geom.ErrInvalid
				}
				e := edges[u.Edge]
				if surface.offset(vertices[e.Start]) > tolerance || surface.offset(vertices[e.End]) > tolerance {
					return fmt.Errorf("face %d: edge %d ends off the surface of the face: %w", i, u.Edge, geom.ErrInvalid)
				}
				uses[u.Edge]++
				sides[u.Edge] = append(sides[u.Edge], i)
				if owner[u.Edge] == 0 {
					owner[u.Edge] = i + 1
				} else {
					shell[root(i)] = root(owner[u.Edge] - 1)
				}
			}
		}
	}
	for i, n := range uses {
		if n != 2 {
			return fmt.Errorf("edge %d has %d uses, and a closed shell gives every edge two: %w", i, n, geom.ErrInvalid)
		}
	}
	for i := range faces {
		if root(i) != root(0) {
			return fmt.Errorf("%w: the boundary has more than one shell", ErrTrimmedBoundary)
		}
	}
	splines := make([][]geom.Vec3, len(edges))
	for i, c := range curves {
		if c.free == nil {
			continue
		}
		a, b := sides[i][0], sides[i][1]
		if a == b || !surfaces[a].cylinder || !surfaces[b].cylinder {
			return fmt.Errorf("edge %d: %w: an edge curve stored as %T that is not where the cylinders of two faces meet", i, ErrTrimmedBoundary, c.free)
		}
		var err error
		if splines[i], err = boundarySpline(edges[i], [2]boundarySurface{surfaces[a], surfaces[b]}, tolerance, s.Tolerance()/2); err != nil {
			return fmt.Errorf("edge %d: %w", i, err)
		}
	}

	return writeStandaloneWithUncertainty(w, unit, 0, "", "ADVANCED_BREP_SHAPE_REPRESENTATION", func(emit func(string) int64) []int64 {
		// A zero is written without a sign: a carrier taken from the other
		// operand, or turned over, has the same axes with negative zeros
		// among their components, and the file should not differ for it.
		point := func(p geom.Vec3) int64 {
			return emit(fmt.Sprintf("CARTESIAN_POINT('',(%s,%s,%s))", real(p.X+0), real(p.Y+0), real(p.Z+0)))
		}
		direction := func(p geom.Vec3) int64 {
			return emit(fmt.Sprintf("DIRECTION('',(%s,%s,%s))", real(p.X+0), real(p.Y+0), real(p.Z+0)))
		}
		placement := func(f geom.Frame) int64 {
			p, z, x := point(f.Origin()), direction(f.ZAxis()), direction(f.XAxis())
			return emit(fmt.Sprintf("AXIS2_PLACEMENT_3D('',#%d,#%d,#%d)", p, z, x))
		}
		flag := func(b bool) string {
			if b {
				return ".T."
			}
			return ".F."
		}
		surfaceIDs := make([]int64, len(faces))
		for i, surface := range surfaces {
			p := placement(surface.frame)
			if surface.cylinder {
				surfaceIDs[i] = emit(fmt.Sprintf("CYLINDRICAL_SURFACE('',#%d,%s)", p, real(surface.radius)))
			} else {
				surfaceIDs[i] = emit(fmt.Sprintf("PLANE('',#%d)", p))
			}
		}
		vertexIDs := make([]int64, len(vertices))
		for i, p := range vertices {
			vertexIDs[i] = emit(fmt.Sprintf("VERTEX_POINT('',#%d)", point(p)))
		}
		// An edge of the solid runs from its start vertex to its end vertex
		// as the parameter of its curve grows, which is the sense of the
		// curve that is written for it.
		edgeIDs := make([]int64, len(edges))
		for i, e := range edges {
			c := curves[i]
			var curve int64
			switch {
			case c.free != nil:
				// Poles 2k+1 and 2k+2 are either side of knot k+1, which
				// is double, so the spline has one tangent there.
				poles := make([]int64, len(splines[i]))
				for k, p := range splines[i] {
					poles[k] = point(p)
				}
				spans := len(poles)/2 - 1
				multiplicities, knots := "4", "0."
				for k := 1; k < spans; k++ {
					multiplicities, knots = multiplicities+",2", knots+fmt.Sprintf(",%d.", k)
				}
				spline := emit(fmt.Sprintf("B_SPLINE_CURVE_WITH_KNOTS('',3,%s,.UNSPECIFIED.,.F.,.F.,(%s,4),(%s,%d.),.UNSPECIFIED.)", references(poles), multiplicities, knots, spans))
				curve = emit(fmt.Sprintf("INTERSECTION_CURVE('',#%d,(#%d,#%d),.CURVE_3D.)", spline, surfaceIDs[sides[i][0]], surfaceIDs[sides[i][1]]))
			case c.line:
				p, d := point(c.frame.Origin()), direction(c.frame.XAxis())
				curve = emit(fmt.Sprintf("LINE('',#%d,#%d)", p, emit(fmt.Sprintf("VECTOR('',#%d,1.)", d))))
			case c.major == c.minor:
				curve = emit(fmt.Sprintf("CIRCLE('',#%d,%s)", placement(c.frame), real(c.major)))
			default:
				curve = emit(fmt.Sprintf("ELLIPSE('',#%d,%s,%s)", placement(c.frame), real(c.major), real(c.minor)))
			}
			edgeIDs[i] = emit(fmt.Sprintf("EDGE_CURVE('',#%d,#%d,#%d,.T.)", vertexIDs[e.Start], vertexIDs[e.End], curve))
		}
		// The loops of a face are stored counterclockwise about the normal
		// of its surface, holes clockwise. A reversed face has the other
		// normal, so its bounds are written with the other sense.
		faceIDs := make([]int64, len(faces))
		for i, f := range faces {
			var bounds []int64
			for j, loop := range append([][]trimmed.Use{f.Outer}, f.Holes...) {
				oriented := make([]int64, len(loop))
				for k, u := range loop {
					oriented[k] = emit(fmt.Sprintf("ORIENTED_EDGE('',*,*,#%d,%s)", edgeIDs[u.Edge], flag(!u.Reversed)))
				}
				kind := "FACE_BOUND"
				if j == 0 {
					kind = "FACE_OUTER_BOUND"
				}
				bounds = append(bounds, emit(fmt.Sprintf("%s('',#%d,%s)", kind, emit("EDGE_LOOP('',"+references(oriented)+")"), flag(!f.Reversed))))
			}
			faceIDs[i] = emit(fmt.Sprintf("ADVANCED_FACE('',%s,#%d,%s)", references(bounds), surfaceIDs[i], flag(f.Reversed == surfaces[i].inward)))
		}
		closed := emit("CLOSED_SHELL(''," + references(faceIDs) + ")")
		return []int64{emit(fmt.Sprintf("MANIFOLD_SOLID_BREP(%s,#%d)", quote(name), closed))}
	})
}

// boundaryRigidSlack bounds how far a placement may be from an orthonormal
// one and still count as keeping lengths. The frame it places is made orthonormal again, so
// what is written differs from the solid by this much of its extent at most.
const boundaryRigidSlack = 1e-9

// boundarySurface is a face surface as the writer knows it: a plane, whose
// normal is the z axis of the frame, or a circular cylinder about that axis,
// whose normal points away from it.
type boundarySurface struct {
	cylinder bool
	frame    geom.Frame
	radius   float64
	// inward is set for a cylinder whose stored surface has its normal
	// towards the axis, which a placement that mirrors gives. A face on it
	// then has the opposite sense against the surface that is written.
	inward bool
}

// offset is the distance from a point to the surface.
func (s boundarySurface) offset(p geom.Vec3) float64 {
	q := s.frame.ToLocal(p)
	if s.cylinder {
		return math.Abs(math.Hypot(q.X, q.Y) - s.radius)
	}
	return math.Abs(q.Z)
}

func boundarySurfaceOf(surface geom.Surface) (boundarySurface, error) {
	base, m := parametric.SurfaceMapping(surface)
	var out boundarySurface
	var own geom.Frame
	switch b := base.(type) {
	case geom.PlaneSurface:
		own = b.Frame()
	case geom.CylinderSurface:
		out.cylinder = true
		own, out.radius = b.Parameters()
	default:
		return out, fmt.Errorf("%w: a surface stored as %T", ErrTrimmedBoundary, base)
	}
	// A mirrored plane keeps its stored normal, the cross product of the
	// images of its axes, which is the z axis of the placed frame.
	frame, mirrored, err := placedBoundaryFrame(own, m)
	out.frame, out.inward = frame, mirrored && out.cylinder
	return out, err
}

// boundaryCurve is an edge curve as the writer knows it. A line passes
// through the origin of the frame along its x axis, with distance as its
// parameter. A conic has the centre and the plane of the frame, its major
// radius along the x axis, and turns counterclockwise about the z axis;
// turn is the parameter of the stored curve at the major axis.
//
// free is set instead for a curve of any other kind, which is written by
// its points where the cylinders of its two faces meet.
type boundaryCurve struct {
	line         bool
	frame        geom.Frame
	major, minor float64
	turn         float64
	free         geom.Curve
}

// at is the point of the curve at a parameter of the stored curve.
func (c boundaryCurve) at(t float64) geom.Vec3 {
	if c.free != nil {
		p, err := c.free.Eval(t)
		if err != nil {
			return geom.Vec3{X: math.NaN()}
		}
		return p
	}
	if c.line {
		return c.frame.ToWorld(geom.Vec3{X: t})
	}
	sin, cos := math.Sincos(t - c.turn)
	return c.frame.ToWorld(geom.Vec3{X: c.major * cos, Y: c.minor * sin})
}

func boundaryCurveOf(e parametric.Edge) (boundaryCurve, error) {
	if e.Curve == nil {
		return boundaryCurve{}, fmt.Errorf("%w: a collapsed edge", ErrTrimmedBoundary)
	}
	span := e.Range.Max - e.Range.Min
	if math.IsNaN(span) || math.IsInf(span, 0) || span <= 0 {
		return boundaryCurve{}, fmt.Errorf("a parameter range that does not grow: %w", geom.ErrInvalid)
	}
	base, m := parametric.CurveMapping(e.Curve)
	switch b := base.(type) {
	case geom.Line:
		along, err := geom.FrameFromNormal(b.Origin(), b.Direction())
		if err != nil {
			return boundaryCurve{}, err
		}
		// The frame of a line has the line as its x axis.
		own, err := geom.NewFrame(b.Origin(), b.Direction(), along.XAxis())
		if err != nil {
			return boundaryCurve{}, err
		}
		frame, _, err := placedBoundaryFrame(own, m)
		return boundaryCurve{line: true, frame: frame}, err
	case geom.Ellipse:
		own, rx, ry := b.Parameters()
		if span > 2*math.Pi*(1+1e-12) {
			return boundaryCurve{}, fmt.Errorf("an arc of more than one turn: %w", geom.ErrInvalid)
		}
		c := boundaryCurve{major: rx, minor: ry}
		if rx < ry {
			// The major axis is the y axis of the stored frame, a quarter
			// turn on from its x axis.
			var err error
			if own, err = geom.NewFrame(own.Origin(), own.YAxis(), own.ZAxis()); err != nil {
				return boundaryCurve{}, err
			}
			c.major, c.minor, c.turn = ry, rx, math.Pi/2
		}
		// The images of the axes carry the parameter, so the placed conic
		// turns counterclockwise about its own z axis under a mirror too.
		var err error
		c.frame, _, err = placedBoundaryFrame(own, m)
		return c, err
	}
	return boundaryCurve{free: e.Curve}, nil
}

// boundarySplineSpans is the greatest number of spans boundarySpline cuts
// an edge into.
const boundarySplineSpans = 4096

// boundarySpline gives the poles of the cubic B-spline that is written for
// an edge where two cylinders meet. The parameter interval of the edge is
// cut into equal spans, and the spline has the point and the derivative of
// the stored curve at both ends of every span, so its knots, the whole
// numbers from zero, are double between the ends and its poles are two to
// a knot. The spans are doubled, from four, until the spline is within
// accuracy of the stored curve at the same parameter at the quarter points
// of every span. Every point taken of the stored curve must be within
// tolerance of both cylinders.
func boundarySpline(e parametric.Edge, on [2]boundarySurface, tolerance, accuracy float64) ([]geom.Vec3, error) {
	type state struct{ p, d geom.Vec3 }
	taken := map[float64]state{} // by the fraction of the interval
	at := func(u float64) (state, error) {
		if s, ok := taken[u]; ok {
			return s, nil
		}
		t := e.Range.Min + (e.Range.Max-e.Range.Min)*u
		p, err := e.Curve.Eval(t)
		if err != nil {
			return state{}, err
		}
		d, err := e.Curve.Derivative(t)
		if err != nil {
			return state{}, err
		}
		if !p.IsFinite() || !d.IsFinite() || !(on[0].offset(p) <= tolerance && on[1].offset(p) <= tolerance) {
			return state{}, fmt.Errorf("%w: an edge curve stored as %T that leaves the cylinders of its two faces", ErrTrimmedBoundary, e.Curve)
		}
		taken[u] = state{p, d}
		return taken[u], nil
	}
	for spans := 4; spans <= boundarySplineSpans; spans *= 2 {
		step := (e.Range.Max - e.Range.Min) / float64(spans)
		poles := make([]geom.Vec3, 0, 2*spans+2)
		worst := 0.
		for k := 0; k < spans; k++ {
			a, err := at(float64(k) / float64(spans))
			if err != nil {
				return nil, err
			}
			b, err := at(float64(k+1) / float64(spans))
			if err != nil {
				return nil, err
			}
			// The span as a Bézier curve on four poles.
			bezier := [4]geom.Vec3{a.p, a.p.Add(a.d.Scale(step / 3)), b.p.Sub(b.d.Scale(step / 3)), b.p}
			if k == 0 {
				poles = append(poles, bezier[0])
			}
			poles = append(poles, bezier[1], bezier[2])
			if k == spans-1 {
				poles = append(poles, bezier[3])
			}
			for _, u := range [3]float64{.25, .5, .75} {
				exact, err := at((float64(k) + u) / float64(spans))
				if err != nil {
					return nil, err
				}
				v := 1 - u
				q := bezier[0].Scale(v * v * v).Add(bezier[1].Scale(3 * v * v * u)).Add(bezier[2].Scale(3 * v * u * u)).Add(bezier[3].Scale(u * u * u))
				worst = math.Max(worst, q.Distance(exact.p))
			}
		}
		if worst <= accuracy {
			return poles, nil
		}
	}
	return nil, fmt.Errorf("%w: an edge curve stored as %T that %d spans of a cubic spline do not follow", ErrTrimmedBoundary, e.Curve, boundarySplineSpans)
}

// placedBoundaryFrame maps a frame by a placement, which must keep lengths:
// a rotation and a translation, with or without a mirror. It gives the frame
// whose x and y axes are the images of the x and y axes, made orthonormal
// again, and tells whether the placement mirrors. The z axis of that frame
// is then the opposite of the image of the z axis.
func placedBoundaryFrame(f geom.Frame, m geom.Affine) (geom.Frame, bool, error) {
	if m == geom.Identity() {
		return f, false, nil
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			dot := m[0][i]*m[0][j] + m[1][i]*m[1][j] + m[2][i]*m[2][j]
			if i == j {
				dot--
			}
			if !(math.Abs(dot) <= boundaryRigidSlack) {
				return geom.Frame{}, false, fmt.Errorf("%w: a placement that does not keep lengths", ErrTrimmedBoundary)
			}
		}
	}
	x, y := m.ApplyVector(f.XAxis()), m.ApplyVector(f.YAxis())
	frame, err := geom.NewFrame(m.Apply(f.Origin()), x, x.Cross(y))
	return frame, m.Determinant() < 0, err
}
