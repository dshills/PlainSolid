package trimmed

import (
	"context"
	"math"
	"strings"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// lensPrism is what two cylinders of radius 1 and height 2 have in common
// when their axes are the lines x = 0 and x = 1 in the plane y = 0. Each cap
// is bounded by an arc of either circle, cut at its middle, so arcs of two
// different circles meet at the tips (0.5, ±√3/2). The tips are moved along
// x by shift, and their chart coordinates in the caps by a further slip that
// their positions do not follow.
func lensPrism(t *testing.T, shift, slip float64) ([]geom.Vec3, []parametric.Edge, []Face) {
	t.Helper()
	const height = 2
	h := math.Sqrt(3) / 2
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	// Vertices 0 to 3 are the lower tip, the middle of the arc of the first
	// circle, the upper tip and the middle of the arc of the second circle,
	// at z = 0; vertices 4 to 7 are the same at the top.
	flat := []geom.Vec2{{X: 0.5 + shift, Y: -h}, {X: 1}, {X: 0.5 + shift, Y: h}, {}}
	var vertices []geom.Vec3
	for _, z := range []float64{0, height} {
		for _, p := range flat {
			vertices = append(vertices, geom.Vec3{X: p.X, Y: p.Y, Z: z})
		}
	}
	// The angles of the four arcs about their centers, in loop order of the
	// top cap, and the vertices they join.
	third := math.Pi / 3
	arcs := []struct {
		center   float64
		from, to float64
		a, b     int
	}{{0, -third, 0, 0, 1}, {0, 0, third, 1, 2}, {1, 2 * third, math.Pi, 2, 3}, {1, math.Pi, 4 * third, 3, 0}}
	var edges []parametric.Edge
	for level, z := range []float64{0, height} {
		for _, arc := range arcs {
			frame, err := geom.NewFrame(geom.Vec3{X: arc.center, Z: z}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
			must(err)
			circle, err := geom.NewEllipse(frame, 1, 1)
			must(err)
			edges = append(edges, parametric.Edge{Start: 4*level + arc.a, End: 4*level + arc.b, Curve: circle, Range: parametric.Interval{Min: arc.from, Max: arc.to}})
		}
	}
	for _, tip := range []int{0, 2} { // edges 8 and 9
		line, err := geom.NewLine(vertices[tip], geom.Vec3{Z: 1})
		must(err)
		edges = append(edges, parametric.Edge{Start: tip, End: tip + 4, Curve: line, Range: parametric.Interval{Max: height}})
	}
	// A cap in the chart (x, side*y) of a plane whose normal is side*z.
	cap := func(level int, side float64) Face {
		frame, err := geom.NewFrame(geom.Vec3{Z: float64(level) * height}, geom.Vec3{X: 1}, geom.Vec3{Z: side})
		must(err)
		plane, err := geom.NewPlaneSurface(frame)
		must(err)
		at := func(vertex int) geom.Vec2 {
			p := flat[vertex]
			if vertex%2 == 0 {
				p.X += slip
			}
			return geom.Vec2{X: p.X, Y: side * p.Y}
		}
		face := Face{Surface: plane}
		for i := range arcs {
			arc, use := arcs[i], Use{Edge: 4*level + i}
			if side < 0 { // the loop of the lower cap runs the other way
				arc = arcs[len(arcs)-1-i]
				arc.from, arc.to, arc.a, arc.b = arc.to, arc.from, arc.b, arc.a
				use = Use{Edge: 4*level + len(arcs) - 1 - i, Reversed: true}
			}
			use.UVStart, use.UVEnd = at(arc.a), at(arc.b)
			use.Curve = EllipseUV{Center: geom.Vec2{X: arc.center}, Cos: geom.Vec2{X: 1}, Sin: geom.Vec2{Y: side}, From: arc.from, To: arc.to}
			face.Outer = append(face.Outer, use)
		}
		return face
	}
	// The wall on the circle whose arcs are edges first and first+1 at the
	// bottom, between the tip lines up and down.
	wall := func(first int, up, down Use) Face {
		frame, err := geom.NewFrame(geom.Vec3{X: arcs[first].center}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		must(err)
		cylinder, err := geom.NewCylinderSurface(frame, 1)
		must(err)
		a, m, b := arcs[first].from, arcs[first].to, arcs[first+1].to
		corners := []geom.Vec2{{X: a}, {X: m}, {X: b}, {X: b, Y: height}, {X: m, Y: height}, {X: a, Y: height}}
		uses := []Use{{Edge: first}, {Edge: first + 1}, up, {Edge: 4 + first + 1, Reversed: true}, {Edge: 4 + first, Reversed: true}, down}
		for i := range uses {
			uses[i].UVStart, uses[i].UVEnd = corners[i], corners[(i+1)%len(corners)]
		}
		return Face{Surface: cylinder, Outer: uses}
	}
	return vertices, edges, []Face{cap(0, -1), cap(1, 1),
		wall(0, Use{Edge: 9}, Use{Edge: 8, Reversed: true}), wall(2, Use{Edge: 8}, Use{Edge: 9, Reversed: true})}
}

// Arcs of two different curves meet at a vertex of every lens-shaped face,
// and each rounds the shared point in its own way. The solid stores such
// arcs with their ends at the declared point, and every query reads them.
func TestNewSolidArcsOfTwoCurvesAtAVertex(t *testing.T) {
	const tolerance = 1e-9
	h := math.Sqrt(3) / 2
	lens := 2*math.Acos(0.5) - h
	for _, c := range []struct {
		name        string
		shift, slip float64
		want        string
	}{
		{name: "tips where the circles meet"},
		{name: "tips moved within the tolerance", shift: 1e-12},
		{name: "chart tips off the arcs", slip: 1e-6, want: "p-curve endpoints differ"},
	} {
		t.Run(c.name, func(t *testing.T) {
			vertices, edges, faces := lensPrism(t, c.shift, c.slip)
			apart := 0
			for _, u := range faces[1].Outer {
				end, err := u.Curve.EvalUV(1)
				if err != nil {
					t.Fatal(err)
				}
				if end != u.UVEnd {
					apart++
				}
			}
			if apart == 0 {
				t.Fatal("every arc ends at its declared end to the last bit, so the case tests nothing")
			}
			solid, err := NewSolid(vertices, edges, faces, tolerance)
			if c.want != "" {
				if err == nil || !strings.Contains(err.Error(), c.want) {
					t.Fatalf("got %v, want an error with %q", err, c.want)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got := solid.Topology(); got.Vertices != 8 || got.Edges != 10 || got.Faces != 4 || got.EulerCharacteristic != 2 {
				t.Fatalf("topology %+v", got)
			}
			// The caller's faces are untouched and the stored arcs keep
			// their coefficients.
			for i, u := range solid.Faces()[1].Outer {
				stored, ok := u.Curve.(interface{ EllipseParameters() EllipseUV })
				if _, plain := faces[1].Outer[i].Curve.(EllipseUV); !plain || !ok || stored.EllipseParameters() != faces[1].Outer[i].Curve {
					t.Fatalf("use %d stores %#v", i, u.Curve)
				}
				for k, want := range [2]geom.Vec2{u.UVStart, u.UVEnd} {
					if got, err := u.Curve.EvalUV(float64(k)); err != nil || got != want {
						t.Fatalf("use %d ends at %v, want %v: %v", i, got, want, err)
					}
				}
			}
			box, err := solid.Bounds()
			if err != nil {
				t.Fatal(err)
			}
			// Bounds are those of the whole circles: at least the lens
			// and at most the two cylinders.
			inner := geom.Box{Min: geom.Vec3{Y: -h}, Max: geom.Vec3{X: 1, Y: h, Z: 2}}
			outer := geom.Box{Min: geom.Vec3{X: -1 - 1e-6, Y: -1 - 1e-6, Z: -1e-6}, Max: geom.Vec3{X: 2 + 1e-6, Y: 1 + 1e-6, Z: 2 + 1e-6}}
			if !(box.Min.X <= inner.Min.X && box.Min.Y <= inner.Min.Y && box.Min.Z <= inner.Min.Z &&
				box.Max.X >= inner.Max.X && box.Max.Y >= inner.Max.Y && box.Max.Z >= inner.Max.Z &&
				box.Min.X >= outer.Min.X && box.Min.Y >= outer.Min.Y && box.Min.Z >= outer.Min.Z &&
				box.Max.X <= outer.Max.X && box.Max.Y <= outer.Max.Y && box.Max.Z <= outer.Max.Z) {
				t.Fatalf("bounds %+v", box)
			}
			properties, err := solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(properties.Volume-2*lens) > 1e-7 || math.Abs(properties.Area-(2*lens+2*4*math.Pi/3)) > 1e-7 {
				t.Fatalf("volume %.12g and area %.12g, want %.12g and %.12g", properties.Volume, properties.Area, 2*lens, 2*lens+2*4*math.Pi/3)
			}
			m, err := solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: 1e-4})
			if err != nil {
				t.Fatal(err)
			}
			if err := m.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
			if got := m.SignedVolume(); math.Abs(got-2*lens) > 1e-3 {
				t.Fatalf("mesh volume %.12g, want %.12g", got, 2*lens)
			}
			for _, q := range []struct {
				point geom.Vec3
				want  parametric.Location
			}{
				{geom.Vec3{X: 0.5, Z: 1}, parametric.Inside},
				{geom.Vec3{X: 0.05, Z: 1}, parametric.Inside},
				{geom.Vec3{X: 0.5, Y: h - 1e-3, Z: 1.9}, parametric.Inside},
				{geom.Vec3{X: 0.5, Y: -h + 1e-3, Z: 0.1}, parametric.Inside},
				{geom.Vec3{X: -0.05, Z: 1}, parametric.Outside},
				{geom.Vec3{X: 0.5, Z: 3}, parametric.Outside},
				{geom.Vec3{X: 0.5, Y: h + 1e-3, Z: 1}, parametric.Outside},
				{geom.Vec3{X: 0.4, Y: h - 1e-3, Z: 1}, parametric.Outside},
				{geom.Vec3{X: 1, Z: 1}, parametric.Boundary},
				{geom.Vec3{X: 0.5, Z: 2}, parametric.Boundary},
				{geom.Vec3{X: 0.5, Y: h - 1e-3}, parametric.Boundary},
				{geom.Vec3{X: 0.5 + c.shift, Y: h, Z: 2}, parametric.Boundary},
			} {
				if got, err := solid.Classify(q.point, tolerance); err != nil || got != q.want {
					t.Fatalf("Classify(%v) = %v, %v; want %v", q.point, got, err, q.want)
				}
			}
			for _, q := range []struct {
				origin, direction geom.Vec3
				want              []float64
			}{
				{geom.Vec3{X: -5, Z: 1}, geom.Vec3{X: 1}, []float64{5, 6}},
				{geom.Vec3{X: 0.5, Y: 0.2, Z: -5}, geom.Vec3{Z: 1}, []float64{5, 7}},
				{geom.Vec3{X: 0.5, Y: h - 1e-3, Z: -5}, geom.Vec3{Z: 1}, []float64{5, 7}},
				{geom.Vec3{X: 0.4, Y: h - 1e-3, Z: -5}, geom.Vec3{Z: 1}, nil},
			} {
				hits, err := solid.Ray(q.origin, q.direction, 100, tolerance)
				if err != nil || len(hits) != len(q.want) {
					t.Fatalf("Ray(%v, %v) = %+v, %v; want hits at %v", q.origin, q.direction, hits, err, q.want)
				}
				for i, hit := range hits {
					if math.Abs(hit.Distance-q.want[i]) > 1e-9 {
						t.Fatalf("Ray(%v, %v) hit %d at %.12g, want %g", q.origin, q.direction, i, hit.Distance, q.want[i])
					}
				}
			}
		})
	}
}

// An arc is pinned to its declared ends only when they are where it ends.
func TestPinArcEnds(t *testing.T) {
	frame, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	plane, err := geom.NewPlaneSurface(frame)
	if err != nil {
		t.Fatal(err)
	}
	cylinder, err := geom.NewCylinderSurface(frame, 3)
	if err != nil {
		t.Fatal(err)
	}
	circle := EllipseUV{Cos: geom.Vec2{X: 2}, Sin: geom.Vec2{Y: 2}, To: 1}
	// An oblique section of the cylinder in its chart of angle and height.
	section := EllipseUV{Cos: geom.Vec2{Y: 1}, Linear: geom.Vec2{X: 1}, From: 0.5, To: 1.5}
	for _, c := range []struct {
		name       string
		surface    geom.Surface
		arc        EllipseUV
		start, end geom.Vec2 // added to where the arc ends
		pinned, ok bool
	}{
		{"its own ends", plane, circle, geom.Vec2{}, geom.Vec2{}, false, true},
		{"ends within the tolerance", plane, circle, geom.Vec2{X: 3e-10}, geom.Vec2{Y: -3e-10}, true, true},
		{"one end at its own place", plane, circle, geom.Vec2{}, geom.Vec2{Y: 3e-10}, true, true},
		{"start beyond the tolerance", plane, circle, geom.Vec2{X: 2e-9}, geom.Vec2{}, false, false},
		{"end beyond the tolerance", plane, circle, geom.Vec2{}, geom.Vec2{Y: 2e-9}, false, false},
		{"angle within the tolerance", cylinder, section, geom.Vec2{X: 1e-10}, geom.Vec2{}, true, true},
		{"angle beyond the tolerance on that radius", cylinder, section, geom.Vec2{X: 5e-10}, geom.Vec2{}, false, false},
		{"start one turn away", cylinder, section, geom.Vec2{X: 2 * math.Pi}, geom.Vec2{}, false, false},
		{"end one turn away", cylinder, section, geom.Vec2{}, geom.Vec2{X: -2 * math.Pi}, false, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			start, err := c.arc.EvalUV(0)
			if err != nil {
				t.Fatal(err)
			}
			end, err := c.arc.EvalUV(1)
			if err != nil {
				t.Fatal(err)
			}
			start, end = add2(start, c.start), add2(end, c.end)
			got, err := pinArcEnds(c.surface, c.arc, start, end, 1e-9)
			if !c.ok {
				if err == nil {
					t.Fatalf("accepted as %#v", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, pinned := got.(unionArcUV); pinned != c.pinned {
				t.Fatalf("stored as %#v", got)
			}
			for k, want := range [2]geom.Vec2{start, end} {
				if p, err := got.EvalUV(float64(k)); err != nil || p != want {
					t.Fatalf("end %d is %v, want %v: %v", k, p, want, err)
				}
			}
			middle, err := got.EvalUV(0.5)
			if want, _ := c.arc.EvalUV(0.5); err != nil || middle != want {
				t.Fatalf("middle %v, want %v: %v", middle, want, err)
			}
		})
	}
}
