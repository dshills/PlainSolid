package trimmed

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// boredBlock is the block [-half,half]² x [0,height] with a bore of the
// given radius along the line through center parallel to the z axis. The
// wall of the bore is one face that winds once around its axis, with a seam
// at angle zero, and each rim is cut into the given number of equal arcs.
func boredBlock(t *testing.T, half, height float64, center geom.Vec2, radius float64, arcs int) ([]geom.Vec3, []parametric.Edge, []Face) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	vertices := []geom.Vec3{
		{X: -half, Y: -half}, {X: half, Y: -half}, {X: half, Y: half}, {X: -half, Y: half},
		{X: -half, Y: -half, Z: height}, {X: half, Y: -half, Z: height}, {X: half, Y: half, Z: height}, {X: -half, Y: half, Z: height},
	}
	edges, faces := planarShell(t, vertices, [][]int{{0, 3, 2, 1}, {4, 5, 6, 7}, {0, 1, 5, 4}, {1, 2, 6, 5}, {2, 3, 7, 6}, {3, 0, 4, 7}}, nil)
	angle := func(k int) float64 { return 2 * math.Pi * float64(k) / float64(arcs) }
	rim := func(level, k int) int { return 8 + level*arcs + k%arcs }
	for _, z := range []float64{0, height} {
		for k := range arcs {
			vertices = append(vertices, geom.Vec3{X: center.X + radius*math.Cos(angle(k)), Y: center.Y + radius*math.Sin(angle(k)), Z: z})
		}
	}
	seam := len(edges)
	line, err := geom.NewLine(vertices[rim(0, 0)], geom.Vec3{Z: 1})
	must(err)
	edges = append(edges, parametric.Edge{Start: rim(0, 0), End: rim(1, 0), Curve: line, Range: parametric.Interval{Max: height}})
	arc := func(level, k int) int { return seam + 1 + level*arcs + k }
	for level, z := range []float64{0, height} {
		frame, err := geom.NewFrame(geom.Vec3{X: center.X, Y: center.Y, Z: z}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
		must(err)
		circle, err := geom.NewEllipse(frame, radius, radius)
		must(err)
		for k := range arcs {
			edges = append(edges, parametric.Edge{Start: rim(level, k), End: rim(level, k+1), Curve: circle, Range: parametric.Interval{Min: angle(k), Max: angle(k + 1)}})
		}
	}
	// The holes of the two plates. The chart of the lower plate is
	// (x+half, -y-half) and that of the upper one (x+half, y+half); a hole
	// runs clockwise in its chart.
	for level, side := range []float64{-1, 1} {
		var hole []Use
		for i := range arcs {
			k, from, to := i, angle(i), angle(i+1)
			if level == 1 {
				k, from, to = arcs-1-i, angle(arcs-i), angle(arcs-1-i)
			}
			curve := EllipseUV{Center: geom.Vec2{X: half + center.X, Y: side * (half + center.Y)}, Cos: geom.Vec2{X: radius}, Sin: geom.Vec2{Y: side * radius}, From: from, To: to}
			a, err := curve.EvalUV(0)
			must(err)
			b, err := curve.EvalUV(1)
			must(err)
			hole = append(hole, Use{Edge: arc(level, k), Reversed: level == 1, UVStart: a, UVEnd: b, Curve: curve})
		}
		faces[level].Holes = [][]Use{hole}
	}
	// The wall, seen from the material: up the seam, back along the upper
	// rim, down the seam a turn earlier, and forward along the lower rim.
	frame, err := geom.NewFrame(geom.Vec3{X: center.X, Y: center.Y}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	must(err)
	cylinder, err := geom.NewCylinderSurface(frame, radius)
	must(err)
	wall := Face{Surface: cylinder, Reversed: true, Outer: []Use{{Edge: seam, UVStart: geom.Vec2{}}}}
	for i := range arcs {
		wall.Outer = append(wall.Outer, Use{Edge: arc(1, arcs-1-i), Reversed: true, UVStart: geom.Vec2{X: angle(arcs-i) - angle(arcs), Y: height}})
	}
	wall.Outer = append(wall.Outer, Use{Edge: seam, Reversed: true, UVStart: geom.Vec2{X: -angle(arcs), Y: height}})
	for i := range arcs {
		wall.Outer = append(wall.Outer, Use{Edge: arc(0, i), UVStart: geom.Vec2{X: angle(i) - angle(arcs)}})
	}
	for i := range wall.Outer {
		wall.Outer[i].UVEnd = wall.Outer[(i+1)%len(wall.Outer)].UVStart
	}
	return vertices, edges, append(faces, wall)
}

// On the wall of a bore that winds once around its axis, a chart triangle
// can have two corners on one copy of the seam and the third on the other.
// All three are then on the seam in space. The triangles of the display mesh
// that are made from sampled trims have such a triangle when the hole of a
// plate is off its middle, because the plate then needs sampled trims.
func TestNewSolidSeamTriangle(t *testing.T) {
	for _, c := range []struct {
		name         string
		half, height float64
		center       geom.Vec2
		radius       float64
		arcs         int
	}{
		{"bore in the middle", 10, 18, geom.Vec2{}, 2.5, 4},
		{"bore off the middle", 10, 18, geom.Vec2{X: 6, Y: 3}, 2.5, 4},
		{"bore off the middle along y", 10, 18, geom.Vec2{Y: 5}, 2.5, 4},
		{"eight arcs", 2, 1, geom.Vec2{X: 0.5, Y: 0.25}, 1, 8},
	} {
		t.Run(c.name, func(t *testing.T) {
			vertices, edges, faces := boredBlock(t, c.half, c.height, c.center, c.radius, c.arcs)
			solid, err := NewSolid(vertices, edges, faces, 1e-7)
			if err != nil {
				t.Fatal(err)
			}
			checkClosedDisplay(t, solid, (4*c.half*c.half-math.Pi*c.radius*c.radius)*c.height, 0.02*c.radius)
		})
	}
}

// checkClosedDisplay requires the volume from MassProperties to be want, or
// only to be positive when want is zero, and a closed mesh from Tessellate
// at the given deflection whose volume differs from it by less than the
// solid's area times the deflection.
func checkClosedDisplay(t *testing.T, solid Solid, want, deflection float64) {
	t.Helper()
	properties, err := solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-7})
	if err != nil {
		t.Fatal(err)
	}
	if !(properties.Volume > 0) || want != 0 && math.Abs(properties.Volume-want) > 1e-6*want {
		t.Fatalf("volume %.12g, want %.12g", properties.Volume, want)
	}
	m, err := solid.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: deflection})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	if got := m.SignedVolume(); math.Abs(got-properties.Volume) > properties.Area*deflection {
		t.Fatalf("mesh volume %.12g, want %.12g within %.3g", got, properties.Volume, properties.Area*deflection)
	}
}

// storedSolid is a solid of planes and cylinders with line and ellipse
// edges, as testdata holds it: what a Boolean handed to NewSolidContext.
type storedSolid struct {
	Tolerance float64
	Vertices  [][3]float64
	Edges     []struct {
		Start, End int
		Ellipse    *storedFrame
		Rx, Ry     float64
		Min, Max   float64
	}
	Faces []struct {
		Frame    storedFrame
		Radius   float64 // of a cylinder; zero for a plane
		Reversed bool
		Loops    [][]struct {
			Edge     int
			Reversed bool
			UV       [2]float64
			Arc      *struct {
				Center, Cos, Sin, Linear [2]float64
				From, To                 float64
			}
		}
	}
}
type storedFrame struct{ O, X, Z [3]float64 }

func (f storedFrame) frame(t *testing.T) geom.Frame {
	t.Helper()
	vec := func(p [3]float64) geom.Vec3 { return geom.Vec3{X: p[0], Y: p[1], Z: p[2]} }
	frame, err := geom.NewFrame(vec(f.O), vec(f.X), vec(f.Z))
	if err != nil {
		t.Fatal(err)
	}
	return frame
}

func loadStoredSolid(t *testing.T, name string) ([]geom.Vec3, []parametric.Edge, []Face, float64) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile("testdata/" + name)
	must(err)
	var stored storedSolid
	must(json.Unmarshal(data, &stored))
	var vertices []geom.Vec3
	for _, p := range stored.Vertices {
		vertices = append(vertices, geom.Vec3{X: p[0], Y: p[1], Z: p[2]})
	}
	var edges []parametric.Edge
	for _, e := range stored.Edges {
		edge := parametric.Edge{Start: e.Start, End: e.End, Range: parametric.Interval{Min: e.Min, Max: e.Max}}
		if e.Ellipse != nil {
			edge.Curve, err = geom.NewEllipse(e.Ellipse.frame(t), e.Rx, e.Ry)
		} else {
			edge.Curve, err = geom.NewLine(vertices[e.Start], vertices[e.End].Sub(vertices[e.Start]))
		}
		must(err)
		edges = append(edges, edge)
	}
	var faces []Face
	for _, f := range stored.Faces {
		face := Face{Reversed: f.Reversed}
		if f.Radius > 0 {
			face.Surface, err = geom.NewCylinderSurface(f.Frame.frame(t), f.Radius)
		} else {
			face.Surface, err = geom.NewPlaneSurface(f.Frame.frame(t))
		}
		must(err)
		for i, loop := range f.Loops {
			uses := make([]Use, len(loop))
			for j, u := range loop {
				next := loop[(j+1)%len(loop)]
				uses[j] = Use{Edge: u.Edge, Reversed: u.Reversed, UVStart: geom.Vec2{X: u.UV[0], Y: u.UV[1]}, UVEnd: geom.Vec2{X: next.UV[0], Y: next.UV[1]}}
				if a := u.Arc; a != nil {
					vec := func(p [2]float64) geom.Vec2 { return geom.Vec2{X: p[0], Y: p[1]} }
					uses[j].Curve = EllipseUV{Center: vec(a.Center), Cos: vec(a.Cos), Sin: vec(a.Sin), Linear: vec(a.Linear), From: a.From, To: a.To}
				}
			}
			if i == 0 {
				face.Outer = uses
			} else {
				face.Holes = append(face.Holes, uses)
			}
		}
		faces = append(faces, face)
	}
	return vertices, edges, faces, stored.Tolerance
}

// The block [-5,5]³ with two overlapping blind bores of radius 2 along x,
// as a Boolean cut it. Both walls wind once around their axes.
// Its display mesh had a vertex in the middle of a seam whose triangles did
// not form one fan.
func TestNewSolidSeamFan(t *testing.T) {
	vertices, edges, faces, tolerance := loadStoredSolid(t, "overlapping_bores.json")
	solid, err := NewSolid(vertices, edges, faces, tolerance)
	if err != nil {
		t.Fatal(err)
	}
	// Two bores of radius 2 and depths 6 and 7 with axes 2 apart, which
	// overlap over a length of 3.
	lens := 8*math.Acos(0.5) - math.Sqrt(12)
	checkClosedDisplay(t, solid, 1000-math.Pi*4*(6+7)+3*lens, 1)
}
