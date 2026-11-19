package stepmeasure_test

import (
	"bytes"
	"math"
	"regexp"
	"strings"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
	"plainsolid/internal/stepmeasure"
)

// TestBoundary measures files of two writers that were in the repository
// before this package, a block with a through bore and a blind bore and a
// cylinder with a seam and closed rims, against their closed forms, and
// requires an error for a file whose topology or geometry was then broken.
func TestBoundary(t *testing.T) {
	var planar, round bytes.Buffer
	stock, err := geom.NewFrame(geom.Vec3{X: 1, Y: 2, Z: 3}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := step.WriteCircularHoleExtrusions(&planar, step.CircularHoleExtrusionModel{LengthUnit: .001, Bodies: []step.CircularHoleExtrusionBody{{
		Name: "block", Frame: stock, Height: 4,
		Outer: []geom.Vec2{{}, {X: 6}, {X: 6, Y: 3}, {Y: 3}},
		Holes: []trimmed.ExtrusionCircle{{Center: geom.Vec2{X: 1.5, Y: 1.5}, Radius: 1}},
		Blind: []trimmed.ExtrusionBlindHole{{ExtrusionCircle: trimmed.ExtrusionCircle{Center: geom.Vec2{X: 4.5, Y: 1.5}, Radius: .5}, Depth: 1}},
	}}}); err != nil {
		t.Fatal(err)
	}
	frame, err := geom.NewFrame(geom.Vec3{X: 1, Y: -1, Z: 2}, geom.Vec3{X: 1, Y: 1}, geom.Vec3{X: 1, Y: -1, Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := step.WriteCylinders(&round, step.CylinderModel{LengthUnit: .001, Bodies: []step.CylinderBody{{Name: "cylinder", Frame: frame, Radius: 1.5, Height: 4}}}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name         string
		file         string
		volume, area float64
	}{
		// The through bore takes its disc from both caps and adds its
		// wall; the blind bore moves its disc to the floor and adds a wall.
		{"block with two bores", planar.String(), 72 - 4*math.Pi - .25*math.Pi, 108 - 2*math.Pi + 8*math.Pi + math.Pi},
		{"cylinder", round.String(), math.Pi * 1.5 * 1.5 * 4, 2*math.Pi*1.5*4 + 2*math.Pi*1.5*1.5},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := stepmeasure.Boundary([]byte(c.file))
			if err != nil {
				t.Fatal(err)
			}
			if math.Abs(got.Volume-c.volume) > 1e-12*c.volume || math.Abs(got.Area-c.area) > 1e-12*c.area {
				t.Fatalf("volume %.15g and area %.15g, want %.15g and %.15g", got.Volume, got.Area, c.volume, c.area)
			}
		})
	}
	if got, err := stepmeasure.Boundary(planar.Bytes()); err != nil || got.Box.Min != (geom.Vec3{X: 1, Y: 2, Z: 3}) || got.Box.Max != (geom.Vec3{X: 7, Y: 5, Z: 7}) {
		t.Fatalf("box %+v (%v)", got.Box, err)
	}
	// Each change is made to the first entity of its kind in the block
	// file; the vertex that moves is the point of the first VERTEX_POINT.
	point := regexp.MustCompile(`=VERTEX_POINT\('',(#[0-9]+)\)`).FindStringSubmatch(planar.String())
	if point == nil {
		t.Fatal("no vertex in the file")
	}
	for _, c := range []struct{ name, entity, old, new string }{
		{"an oriented edge turned", "=ORIENTED_EDGE(", ".T.)", ".F.)"},
		{"a face turned", "=ADVANCED_FACE(", ".T.)", ".F.)"},
		{"a vertex moved", point[1] + "=CARTESIAN_POINT(", "('',(", "('',(0.25E1"},
	} {
		t.Run(c.name, func(t *testing.T) {
			lines, changed := strings.Split(planar.String(), "\n"), false
			for i, line := range lines {
				if strings.Contains(line, c.entity) && strings.Contains(line, c.old) {
					lines[i], changed = strings.Replace(line, c.old, c.new, 1), true
					break
				}
			}
			if !changed {
				t.Fatalf("no %s with %s in the file", c.entity, c.old)
			}
			if got, err := stepmeasure.Boundary([]byte(strings.Join(lines, "\n"))); err == nil {
				t.Fatalf("measured %+v", got)
			}
		})
	}
}
