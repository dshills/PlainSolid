package trimmed

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// obliqueCylinder is the cylinder of radius r about the z axis between the
// plane z = 0 and the plane z = c + a*x. Its wall is one face that winds once
// around, with a seam at angle 0. The circle below is cut into bottom arcs
// and the ellipse above into top arcs, so with different counts the two rims
// of the wall are cut at different angles. skew moves the angle of the two
// upper ends of the seam by that many units in the last place, as an
// intersection that computes them does.
func obliqueCylinder(t *testing.T, bottom, top int, r, c, a float64, skew int) Solid {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	angle := func(i, n int) float64 { return 2 * math.Pi * float64(i) / float64(n) }
	height := func(u float64) float64 { return c + a*r*math.Cos(u) }
	var vertices []geom.Vec3
	for i := range bottom {
		u := angle(i, bottom)
		vertices = append(vertices, geom.Vec3{X: r * math.Cos(u), Y: r * math.Sin(u)})
	}
	for i := range top {
		u := angle(i, top)
		vertices = append(vertices, geom.Vec3{X: r * math.Cos(u), Y: r * math.Sin(u), Z: height(u)})
	}
	base, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	must(err)
	circle, err := geom.NewEllipse(base, r, r)
	must(err)
	stretch := math.Hypot(1, a)
	slanted, err := geom.NewFrame(geom.Vec3{Z: c}, geom.Vec3{X: 1, Z: a}, geom.Vec3{X: -a, Z: 1})
	must(err)
	ellipse, err := geom.NewEllipse(slanted, r*stretch, r)
	must(err)
	// Edges 0 to bottom-1 are the arcs below, the next top edges the arcs
	// above, and the last is the seam.
	var edges []parametric.Edge
	for i := range bottom {
		edges = append(edges, parametric.Edge{Start: i, End: (i + 1) % bottom, Curve: circle, Range: parametric.Interval{Min: angle(i, bottom), Max: angle(i+1, bottom)}})
	}
	for i := range top {
		edges = append(edges, parametric.Edge{Start: bottom + i, End: bottom + (i+1)%top, Curve: ellipse, Range: parametric.Interval{Min: angle(i, top), Max: angle(i+1, top)}})
	}
	line, err := geom.NewLine(vertices[0], geom.Vec3{Z: 1})
	must(err)
	seam := len(edges)
	edges = append(edges, parametric.Edge{Start: 0, End: bottom, Curve: line, Range: parametric.Interval{Max: height(0)}})

	cylinder, err := geom.NewCylinderSurface(base, r)
	must(err)
	wall := Face{Surface: cylinder}
	for i := range bottom {
		wall.Outer = append(wall.Outer, Use{Edge: i, UVStart: geom.Vec2{X: angle(i, bottom)}, UVEnd: geom.Vec2{X: angle(i+1, bottom)}})
	}
	right, left := geom.Vec2{X: 2 * math.Pi, Y: height(0)}, geom.Vec2{Y: height(0)}
	for range skew {
		right.X, left.X = math.Nextafter(right.X, 7), math.Nextafter(left.X, 1)
	}
	corner := func(i int) geom.Vec2 {
		switch i {
		case 0:
			return left
		case top:
			return right
		}
		return geom.Vec2{X: angle(i, top), Y: height(angle(i, top))}
	}
	wall.Outer = append(wall.Outer, Use{Edge: seam, UVStart: geom.Vec2{X: 2 * math.Pi}, UVEnd: right})
	for i := top - 1; i >= 0; i-- {
		wall.Outer = append(wall.Outer, Use{Edge: bottom + i, Reversed: true, UVStart: corner(i + 1), UVEnd: corner(i),
			Curve: EllipseUV{Center: geom.Vec2{Y: c}, Cos: geom.Vec2{Y: a * r}, Linear: geom.Vec2{X: 1}, From: angle(i+1, top), To: angle(i, top)}})
	}
	wall.Outer = append(wall.Outer, Use{Edge: seam, Reversed: true, UVStart: left, UVEnd: geom.Vec2{}})

	plane, err := geom.NewPlaneSurface(slanted)
	must(err)
	above := Face{Surface: plane}
	// A cap corner is read at the angle of its vertex, so that the first and
	// the last arc of a loop meet at one chart point.
	on := func(i, n int, x, y float64) geom.Vec2 {
		u := angle(i%n, n)
		return geom.Vec2{X: x * math.Cos(u), Y: y * math.Sin(u)}
	}
	for i := range top {
		above.Outer = append(above.Outer, Use{Edge: bottom + i, UVStart: on(i, top, r*stretch, r), UVEnd: on(i+1, top, r*stretch, r),
			Curve: EllipseUV{Cos: geom.Vec2{X: r * stretch}, Sin: geom.Vec2{Y: r}, From: angle(i, top), To: angle(i+1, top)}})
	}
	// The plane below faces down, so its chart is (x, -y) and its loop runs
	// against the arcs.
	down, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: -1})
	must(err)
	floor, err := geom.NewPlaneSurface(down)
	must(err)
	below := Face{Surface: floor}
	for i := bottom - 1; i >= 0; i-- {
		below.Outer = append(below.Outer, Use{Edge: i, Reversed: true, UVStart: on(i+1, bottom, r, -r), UVEnd: on(i, bottom, r, -r),
			Curve: EllipseUV{Cos: geom.Vec2{X: r}, Sin: geom.Vec2{Y: -r}, From: angle(i+1, bottom), To: angle(i, bottom)}})
	}
	solid, err := NewSolid(vertices, edges, []Face{wall, above, below}, 1e-9)
	must(err)
	return solid
}

// A wall that winds once around a cylinder, between a circle and the ellipse
// of an oblique plane, is meshed within the angle bound in strips along the
// axis, whether or not the two rims are cut at the same angles. Uniform
// refinement of its chart triangles did not meet the bound within the budget.
func TestTessellateCylinderBand(t *testing.T) {
	o := parametric.TessellationOptions{Deflection: .05, Angle: .1, MaxSubdivisions: 256, MaxTriangles: 1000000}
	for _, c := range []struct {
		name        string
		bottom, top int
		r, c, a     float64
		skew        int
		most        int
	}{
		{name: "rims cut at different angles", bottom: 4, top: 5, r: 1, c: 1.6, a: 1.4, most: 600},
		{name: "rims cut at the same angles", bottom: 4, top: 4, r: 1, c: 1.6, a: 1.4, most: 600},
		{name: "seam ends that differ in the last bits", bottom: 4, top: 5, r: 1, c: 1.6, a: 1.4, skew: 2, most: 600},
		{name: "long thin wall", bottom: 4, top: 4, r: .35, c: 6.3, a: 1. / 3, most: 300},
		{name: "rims of five arcs and three", bottom: 5, top: 3, r: 2, c: 3, a: .5, most: 600},
	} {
		t.Run(c.name, func(t *testing.T) {
			solid := obliqueCylinder(t, c.bottom, c.top, c.r, c.c, c.a, c.skew)
			if _, _, band := compactBand(solid.faces[0]); !band {
				t.Fatal("the wall is not a band")
			}
			m, err := solid.Tessellate(context.Background(), o)
			if err != nil {
				t.Fatal(err)
			}
			if len(m.Triangles) > c.most {
				t.Fatalf("%d triangles, want at most %d", len(m.Triangles), c.most)
			}
			p, err := m.MassProperties()
			if err != nil {
				t.Fatal(err)
			}
			if want := math.Pi * c.r * c.r * c.c; math.Abs(p.Volume-want) > .02*want {
				t.Fatalf("volume %g, want %g within 2%%", p.Volume, want)
			}
			// The strips also carry the surface normals of their corners.
			shaded, normals, ok, err := solid.TessellateCompactNormals(context.Background(), o)
			if err != nil || !ok || len(normals) != len(shaded.Triangles) || len(shaded.Triangles) != len(m.Triangles) {
				t.Fatalf("compact normals: %d triangles, %d normals, ok %v: %v", len(shaded.Triangles), len(normals), ok, err)
			}
		})
	}
}

// Where the strips cannot be finished within the caller's limits, the compact
// mesher reports that a solid with a band is not its to mesh, so that
// Tessellate goes on to uniform refinement as it did before bands were charts.
func TestCompactCylinderMeshLeavesAnUnfinishedBand(t *testing.T) {
	solid := obliqueCylinder(t, 4, 5, 1, 1.6, 1.4, 0)
	o := parametric.TessellationOptions{Deflection: .05, Angle: .1, MaxSubdivisions: 1, MaxTriangles: 1000000}
	if _, banded, ok, err := solid.compactCylinderCharts(context.Background(), o, nil); !banded || !ok || err != parametric.ErrBudget {
		t.Fatalf("one subdivision: banded %v, ok %v, %v; want the strips to exceed the budget", banded, ok, err)
	}
	if m, ok, err := solid.compactCylinderMesh(context.Background(), o); ok || err != nil || len(m.Triangles) != 0 {
		t.Fatalf("one subdivision: %d triangles, ok %v, %v; want the solid left to the caller", len(m.Triangles), ok, err)
	}
}

func TestCompactBand(t *testing.T) {
	frame, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	cylinder, err := geom.NewCylinderSurface(frame, 1)
	if err != nil {
		t.Fatal(err)
	}
	plane, err := geom.NewPlaneSurface(frame)
	if err != nil {
		t.Fatal(err)
	}
	// rim is the trim of a plane from angle a to angle b: the height is
	// 2 + cos(angle) + lift.
	rim := func(a, b, lift float64) Use {
		at := func(u float64) geom.Vec2 { return geom.Vec2{X: u, Y: 2 + lift + math.Cos(u)} }
		return Use{UVStart: at(a), UVEnd: at(b), Curve: EllipseUV{Center: geom.Vec2{Y: 2 + lift}, Cos: geom.Vec2{Y: 1}, Linear: geom.Vec2{X: 1}, From: a, To: b}}
	}
	straight := func(ax, ay, bx, by float64) Use {
		return Use{UVStart: geom.Vec2{X: ax, Y: ay}, UVEnd: geom.Vec2{X: bx, Y: by}}
	}
	turn, quarter := 2*math.Pi, math.Pi/2
	seamTop := math.Nextafter(turn, 7)
	circleBelow := []Use{straight(0, 0, quarter, 0), straight(quarter, 0, 2*quarter, 0), straight(2*quarter, 0, 3*quarter, 0), straight(3*quarter, 0, turn, 0)}
	wound := append(append([]Use{}, circleBelow...),
		straight(turn, 0, seamTop, 3),
		rim(turn, 5, 0), rim(5, 4, 0), rim(4, 2.5, 0), rim(2.5, 1.3, 0), rim(1.3, 0, 0),
		straight(0, 3, 0, 0))
	wound[5].UVStart = wound[4].UVEnd
	memo := &tessellationTrimMemo{limit: 16}
	read := append([]Use{}, wound...)
	for i := range read {
		if read[i].Curve != nil {
			read[i].Curve = tessellationMemoCurve{read[i].Curve, memo, i}
		}
	}
	bent := rim(0, 1, 0)
	bent.Curve = EllipseUV{Center: geom.Vec2{Y: 2}, Cos: geom.Vec2{X: .1, Y: 1}, Linear: geom.Vec2{X: 1}, To: 1}
	arched := straight(1, 0, 1, 1)
	arched.Curve = EllipseUV{Center: geom.Vec2{X: 1}, Sin: geom.Vec2{Y: 1}, To: quarter}
	for _, c := range []struct {
		name         string
		face         Face
		start, lower int
		ok           bool
	}{
		{name: "wall around the seam, a circle below and five arcs above", face: Face{Surface: cylinder, Outer: wound}, start: 10, lower: 6, ok: true},
		{name: "trims read through the memo of Tessellate", face: Face{Surface: cylinder, Outer: read}, start: 10, lower: 6, ok: true},
		{name: "loop that starts on the seam, arcs on both rims", face: Face{Surface: cylinder, Outer: []Use{
			straight(0, 3, 0, 9), rim(0, -quarter, 6), rim(-quarter, -2*quarter, 6), rim(-2*quarter, -3*quarter, 6), rim(-3*quarter, -turn, 6),
			straight(-turn, 9, -turn, 3), rim(-turn, -3*quarter, 0), rim(-3*quarter, -2*quarter, 0), rim(-2*quarter, -quarter, 0), rim(-quarter, 0, 0),
		}}, start: 5, lower: 6, ok: true},
		{name: "chains that meet at both ends", face: Face{Surface: cylinder, Outer: []Use{
			straight(0, 0, 1, 0), straight(1, 0, 2, 0), rim(2, 1, 0), straight(1, 2+math.Cos(1), 0, 0),
		}}, start: 0, lower: 2, ok: true},
		{name: "generator that steps a chain", face: Face{Surface: cylinder, Outer: []Use{
			straight(0, 0, 1, 0), straight(1, 0, 1, .5), straight(1, .5, 2, .5), straight(2, .5, 2, 2), straight(2, 2, 0, 2), straight(0, 2, 0, 0),
		}}, start: 5, lower: 5, ok: true},
		{name: "rectangle, which keeps its own strips", face: Face{Surface: cylinder, Outer: []Use{
			straight(0, 0, 1, 0), straight(1, 0, 1, 2), straight(1, 2, 0, 2), straight(0, 2, 0, 0),
		}}},
		{name: "hole", face: Face{Surface: cylinder, Outer: wound, Holes: [][]Use{{straight(1, 1, 1, 1.2), straight(1, 1.2, 1.2, 1), straight(1.2, 1, 1, 1)}}}},
		{name: "angle that turns back twice", face: Face{Surface: cylinder, Outer: []Use{
			straight(0, 0, 2, 0), straight(2, 0, 1, 1), straight(1, 1, 2, 2), straight(2, 2, 0, 2), straight(0, 2, 0, 0),
		}}},
		{name: "trim that is not a graph over the angle", face: Face{Surface: cylinder, Outer: []Use{
			bent, straight(1, 2+math.Cos(1), 1, 4), straight(1, 4, 0, 4), straight(0, 4, 0, 3),
		}}},
		{name: "curved trim along a generator", face: Face{Surface: cylinder, Outer: []Use{
			straight(0, 0, 1, 0), arched, straight(1, 1, 0, 1), straight(0, 1, 0, 0),
		}}},
		{name: "plane", face: Face{Surface: plane, Outer: wound}},
	} {
		t.Run(c.name, func(t *testing.T) {
			start, lower, ok := compactBand(c.face)
			if ok != c.ok || start != c.start || lower != c.lower {
				t.Fatalf("got start %d, lower %d, %v; want %d, %d, %v", start, lower, ok, c.start, c.lower, c.ok)
			}
			if at, axial := compactAxialStart(c.face); ok && (!axial || at != start) {
				t.Fatalf("the band is read from use %d, axial %v", at, axial)
			}
		})
	}
}

func TestCompactBandTriangles(t *testing.T) {
	p := func(x, y float64) geom.Vec2 { return geom.Vec2{X: x, Y: y} }
	for _, c := range []struct {
		name         string
		lower, upper [][]geom.Vec2
		fails        bool
	}{
		{name: "samples at the same angles",
			lower: [][]geom.Vec2{{p(0, 1), p(0, 0)}, {p(0, 0), p(1, 0), p(2, 0)}, {p(2, 0), p(2, 1)}},
			upper: [][]geom.Vec2{{p(2, 1), p(1, 1.5), p(0, 1)}}},
		{name: "samples at different angles",
			lower: [][]geom.Vec2{{p(0, 1), p(0, 0)}, {p(0, 0), p(.5, .1), p(1, 0), p(1.5, .1), p(2, 0)}, {p(2, 0), p(2, 1)}},
			upper: [][]geom.Vec2{{p(2, 1), p(1.3, 1.2), p(.7, 1.2), p(0, 1)}}},
		{name: "chains that meet at both ends",
			lower: [][]geom.Vec2{{p(0, 0), p(1, -.5), p(2, 0)}},
			upper: [][]geom.Vec2{{p(2, 0), p(1.5, .4), p(.5, .4), p(0, 0)}}},
		{name: "one sample closes a chain",
			lower: [][]geom.Vec2{{p(0, 0), p(1, -.5), p(2, 0)}},
			upper: [][]geom.Vec2{{p(2, 0), p(0, 0)}}},
		{name: "step down that the other chain must pass first",
			lower: [][]geom.Vec2{{p(0, 1), p(0, .5)}, {p(0, .5), p(1, .5)}, {p(1, .5), p(1, 0)}, {p(1, 0), p(2, 0)}, {p(2, 0), p(2, 1)}},
			upper: [][]geom.Vec2{{p(2, 1), p(1.1, 1), p(.9, 1), p(0, 1)}}},
		{name: "rise that hides the next sample",
			lower: [][]geom.Vec2{{p(0, 1), p(0, 0)}, {p(0, 0), p(1, .9), p(2, 0)}, {p(2, 0), p(2, 1)}},
			upper: [][]geom.Vec2{{p(2, 1), p(0, 1)}}, fails: true},
		{name: "two samples", lower: [][]geom.Vec2{{p(0, 0), p(1, 0)}}, upper: [][]geom.Vec2{{p(1, 0), p(0, 0)}}, fails: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			triangles, err := compactBandTriangles(c.lower, c.upper)
			if c.fails {
				if err != errRefine {
					t.Fatalf("got %d triangles and %v, want a request for finer samples", len(triangles), err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var loop []geom.Vec2
			for _, use := range append(append([][]geom.Vec2{}, c.lower...), c.upper...) {
				loop = append(loop, use[:len(use)-1]...)
			}
			area := func(points ...geom.Vec2) float64 {
				sum := 0.
				for i, a := range points {
					b := points[(i+1)%len(points)]
					sum += a.X*b.Y - b.X*a.Y
				}
				return sum / 2
			}
			if len(triangles) != len(loop)-2 {
				t.Fatalf("%d triangles for %d samples", len(triangles), len(loop))
			}
			// Triangles that all keep the orientation of the loop and
			// together have its area cover it once.
			covered := 0.
			for _, tri := range triangles {
				if turn, err := geom.Orient2D(tri[0], tri[1], tri[2]); err != nil || turn != 1 {
					t.Fatalf("triangle %v is not positively oriented", tri)
				}
				covered += area(tri[:]...)
			}
			if want := area(loop...); math.Abs(covered-want) > 1e-12 {
				t.Fatalf("the triangles cover %g of an area of %g", covered, want)
			}
		})
	}
}
