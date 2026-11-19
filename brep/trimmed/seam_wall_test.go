package trimmed

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// cappedCylinder is a cylinder of the given radius about the z axis between
// the planes z = low[0] + low[1]*x and z = high[0] + high[1]*x. Its wall is
// one face that winds once around the axis, with a seam at angle zero, and
// each rim is cut into the given number of equal arcs.
func cappedCylinder(t *testing.T, radius float64, low, high [2]float64, arcs int) ([]geom.Vec3, []parametric.Edge, []Face) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	angle := func(k int) float64 { return 2 * math.Pi * float64(k) / float64(arcs) }
	var vertices []geom.Vec3
	for level, rim := range [2][2]float64{low, high} {
		for k := range arcs {
			// The lower rim runs the other way around the axis.
			x, y := radius*math.Cos(angle(k)), float64(2*level-1)*radius*math.Sin(angle(k))
			vertices = append(vertices, geom.Vec3{X: x, Y: y, Z: rim[0] + rim[1]*x})
		}
	}
	seam, err := geom.NewLine(vertices[0], vertices[arcs].Sub(vertices[0]))
	must(err)
	edges := []parametric.Edge{{Start: 0, End: arcs, Curve: seam, Range: parametric.Interval{Max: vertices[arcs].Sub(vertices[0]).Length()}}}
	frame, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	must(err)
	cylinder, err := geom.NewCylinderSurface(frame, radius)
	must(err)
	wall := Face{Surface: cylinder, Outer: []Use{{Edge: 0, UVStart: geom.Vec2{Y: low[0] + low[1]*radius}, UVEnd: geom.Vec2{Y: high[0] + high[1]*radius}}}}
	var caps [2]Face
	var along [2][]Use // the uses of the wall along each rim, in loop order
	for level, rim := range [2][2]float64{low, high} {
		side := float64(2*level - 1) // the lower cap faces down
		stretch := math.Hypot(1, rim[1])
		plane, err := geom.NewFrame(geom.Vec3{Z: rim[0]}, geom.Vec3{X: 1, Z: rim[1]}, geom.Vec3{X: -side * rim[1], Z: side})
		must(err)
		ellipse, err := geom.NewEllipse(plane, radius*stretch, radius)
		must(err)
		surface, err := geom.NewPlaneSurface(plane)
		must(err)
		caps[level].Surface = surface
		for k := range arcs {
			from, to := angle(k), angle(k+1)
			id := len(edges)
			edges = append(edges, parametric.Edge{Start: level*arcs + k, End: level*arcs + (k+1)%arcs, Curve: ellipse, Range: parametric.Interval{Min: from, Max: to}})
			curve := EllipseUV{Cos: geom.Vec2{X: radius * stretch}, Sin: geom.Vec2{Y: radius}, From: from, To: to}
			a, err := curve.EvalUV(0)
			must(err)
			b, err := curve.EvalUV(1)
			must(err)
			caps[level].Outer = append(caps[level].Outer, Use{Edge: id, UVStart: a, UVEnd: b, Curve: curve})
			// On the wall the arc is a cosine over the angle, which runs
			// against the parameter of the lower rim and, a turn back,
			// with that of the upper one. The wall uses it reversed.
			trim := EllipseUV{Center: geom.Vec2{Y: rim[0]}, Cos: geom.Vec2{Y: rim[1] * radius}, Linear: geom.Vec2{X: side}, From: to, To: from}
			if level == 1 {
				trim.From, trim.To = to-2*math.Pi, from-2*math.Pi
			}
			a, err = trim.EvalUV(0)
			must(err)
			b, err = trim.EvalUV(1)
			must(err)
			along[level] = append([]Use{{Edge: id, Reversed: true, UVStart: a, UVEnd: b, Curve: trim}}, along[level]...)
		}
	}
	wall.Outer = append(wall.Outer, along[1]...)
	wall.Outer = append(wall.Outer, Use{Edge: 0, Reversed: true, UVStart: geom.Vec2{X: -2 * math.Pi, Y: high[0] + high[1]*radius}, UVEnd: geom.Vec2{X: -2 * math.Pi, Y: low[0] + low[1]*radius}})
	wall.Outer = append(wall.Outer, along[0]...)
	// Corners take the place the straight seam gives them.
	for i := range wall.Outer {
		next := &wall.Outer[(i+1)%len(wall.Outer)]
		if wall.Outer[i].Curve == nil {
			next.UVStart = wall.Outer[i].UVEnd
		} else if next.Curve == nil {
			wall.Outer[i].UVEnd = next.UVStart
		}
	}
	return vertices, edges, []Face{caps[0], caps[1], wall}
}

// obliqueSlug is the part of a cylinder of radius 0.5, 30° from the
// vertical, between the planes z = 0 and z = 2, with the vertices and the
// chart coordinates that a Boolean of a slab and a cylinder gave it.
func obliqueSlug(t *testing.T) ([]geom.Vec3, []parametric.Edge, []Face) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	vertices := []geom.Vec3{
		{X: 1.9999999999999998, Y: 1.9999999999999998, Z: -9.0515721843007224e-17},
		{X: 3.1547005383792515, Y: 1.9999999999999998, Z: 2.0000000000000004},
		{X: 1.4226497308103745, Y: 1.5, Z: 7.355370892572099e-17},
		{X: 0.84529946162074865, Y: 2, Z: 7.355370892572099e-17},
		{X: 1.4226497308103745, Y: 2.5, Z: 7.355370892572099e-17},
		{X: 2.5773502691896262, Y: 2.5, Z: 2},
		{X: 2.0000000000000004, Y: 2, Z: 2},
		{X: 2.5773502691896262, Y: 1.5, Z: 2},
	}
	line, err := geom.NewLine(vertices[0], vertices[1].Sub(vertices[0]))
	must(err)
	edges := []parametric.Edge{{Start: 0, End: 1, Curve: line, Range: parametric.Interval{Max: 2.3094010767585034}}}
	const quarter, half, three, turn = 1.5707963267948966, 3.141592653589793, 4.71238898038469, 6.283185307179586
	angles := []float64{0, quarter, half, three, turn}
	var caps [2]Face
	for level, c := range []struct {
		center float64
		side   float64
		ring   []int
	}{{1.4226497308103745, -1, []int{0, 2, 3, 4, 0}}, {2.577350269189626, 1, []int{1, 5, 6, 7, 1}}} {
		origin := geom.Vec3{X: c.center, Y: 2, Z: float64(2 * level)}
		frame, err := geom.NewFrame(origin, geom.Vec3{X: 1}, geom.Vec3{Z: c.side})
		must(err)
		ellipse, err := geom.NewEllipse(frame, 0.5773502691896258, 0.5)
		must(err)
		plane, err := geom.NewFrame(geom.Vec3{Z: origin.Z}, geom.Vec3{X: 1}, geom.Vec3{Z: c.side})
		must(err)
		surface, err := geom.NewPlaneSurface(plane)
		must(err)
		caps[level].Surface = surface
		for k := range 4 {
			edges = append(edges, parametric.Edge{Start: c.ring[k], End: c.ring[k+1], Curve: ellipse, Range: parametric.Interval{Min: angles[k], Max: angles[k+1]}})
			curve := EllipseUV{Center: geom.Vec2{X: c.center, Y: c.side * 2}, Cos: geom.Vec2{X: 0.5773502691896258}, Sin: geom.Vec2{Y: 0.5}, From: angles[k], To: angles[k+1]}
			a, err := curve.EvalUV(0)
			must(err)
			b, err := curve.EvalUV(1)
			must(err)
			caps[level].Outer = append(caps[level].Outer, Use{Edge: 1 + 4*level + k, UVStart: a, UVEnd: b, Curve: curve})
		}
	}
	frame, err := geom.NewFrame(geom.Vec3{X: 0.5, Y: 2, Z: -1.598076211353316}, geom.Vec3{X: 0.8660254037844385, Z: -0.5}, geom.Vec3{X: 0.5, Z: 0.8660254037844385})
	must(err)
	cylinder, err := geom.NewCylinderSurface(frame, 0.5)
	must(err)
	arc := func(height, direction, from, to float64) EllipseUV {
		return EllipseUV{Center: geom.Vec2{Y: height}, Cos: geom.Vec2{Y: 0.2886751345948129}, Linear: geom.Vec2{X: direction}, From: from, To: to}
	}
	const top, bottom = 4.1547005383792515, 1.8452994616207483
	wall := Face{Surface: cylinder, Outer: []Use{
		{Edge: 0, UVStart: geom.Vec2{X: 0, Y: 2.1339745962155612}},
		{Edge: 8, Reversed: true, UVStart: geom.Vec2{X: 0, Y: 4.4433756729740645}, Curve: arc(top, 1, 0, -quarter)},
		{Edge: 7, Reversed: true, UVStart: geom.Vec2{X: -1.5707963267948966, Y: 4.1547005383792515}, Curve: arc(top, 1, -quarter, -half)},
		{Edge: 6, Reversed: true, UVStart: geom.Vec2{X: -3.1415926535897931, Y: 3.8660254037844384}, Curve: arc(top, 1, -half, -three)},
		{Edge: 5, Reversed: true, UVStart: geom.Vec2{X: -4.7123889803846897, Y: 4.1547005383792515}, Curve: arc(top, 1, -three, -turn)},
		{Edge: 0, Reversed: true, UVStart: geom.Vec2{X: -6.2831853071795862, Y: 4.4433756729740645}},
		{Edge: 4, Reversed: true, UVStart: geom.Vec2{X: -6.2831853071795862, Y: 2.1339745962155612}, Curve: arc(bottom, -1, turn, three)},
		{Edge: 3, Reversed: true, UVStart: geom.Vec2{X: -4.7123889803846897, Y: 1.8452994616207483}, Curve: arc(bottom, -1, three, half)},
		{Edge: 2, Reversed: true, UVStart: geom.Vec2{X: -3.1415926535897931, Y: 1.5566243270259355}, Curve: arc(bottom, -1, half, quarter)},
		{Edge: 1, Reversed: true, UVStart: geom.Vec2{X: -1.5707963267948966, Y: 1.8452994616207483}, Curve: arc(bottom, -1, quarter, 0)},
	}}
	for i := range wall.Outer {
		wall.Outer[i].UVEnd = wall.Outer[(i+1)%len(wall.Outer)].UVStart
	}
	return vertices, edges, []Face{caps[0], caps[1], wall}
}

// On a wall that winds once around a cylinder, a seed side can join the two
// chart copies of one seam vertex. It is then a closed curve in space, and
// one level of subdivision makes one mesh edge of its two halves.
func TestNewSolidSeedSideAroundASeam(t *testing.T) {
	slope := math.Tan(math.Pi / 6)
	for _, c := range []struct {
		name   string
		build  func(t *testing.T) ([]geom.Vec3, []parametric.Edge, []Face)
		volume float64
	}{
		{"lower cap oblique, eight arcs", func(t *testing.T) ([]geom.Vec3, []parametric.Edge, []Face) {
			return cappedCylinder(t, 0.5, [2]float64{0, -slope}, [2]float64{2, 0}, 8)
		}, 0.5 * math.Pi},
		{"both caps oblique, from a Boolean", obliqueSlug, 0.25 * math.Pi * 2 / math.Cos(math.Pi/6)},
		{"flat caps, four arcs", func(t *testing.T) ([]geom.Vec3, []parametric.Edge, []Face) {
			return cappedCylinder(t, 0.5, [2]float64{0, 0}, [2]float64{2, 0}, 4)
		}, 0.5 * math.Pi},
	} {
		t.Run(c.name, func(t *testing.T) {
			vertices, edges, faces := c.build(t)
			solid, err := NewSolid(vertices, edges, faces, 1e-7)
			if err != nil {
				t.Fatal(err)
			}
			properties, err := solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-6})
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(properties.Volume-c.volume) > 1e-5 {
				t.Fatalf("volume %.12g, want %.12g", properties.Volume, c.volume)
			}
			m, err := solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: 0.1})
			if err != nil {
				t.Fatal(err)
			}
			if err := m.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
			// Chords 0.1 inside a circle of radius 0.5 enclose 74% of it.
			if got := m.SignedVolume(); got > c.volume+1e-9 || got < 0.74*c.volume {
				t.Fatalf("mesh volume %.12g, want a little under %.12g", got, c.volume)
			}
		})
	}
}
