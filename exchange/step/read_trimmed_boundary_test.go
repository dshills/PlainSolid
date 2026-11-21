package step_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"

	"plainsolid/brep/boolean"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

var (
	boundaryEdgeCurve    = regexp.MustCompile(`=EDGE_CURVE\('',(#[0-9]+),(#[0-9]+),(#[0-9]+),\.T\.\)`)
	boundaryOrientedEdge = regexp.MustCompile(`=ORIENTED_EDGE\('',\*,\*,(#[0-9]+),\.([TF])\.\)`)
)

// boundaryAgainst states the same solid with every edge run against its
// curve: each edge goes from its second vertex to its first with same_sense
// false, and each use of it is turned.
func boundaryAgainst(file string) string {
	file = boundaryEdgeCurve.ReplaceAllString(file, "=EDGE_CURVE('',$2,$1,$3,.F.)")
	return boundaryOrientedEdge.ReplaceAllStringFunc(file, func(use string) string {
		if strings.HasSuffix(use, ".T.)") {
			return strings.TrimSuffix(use, ".T.)") + ".F.)"
		}
		return strings.TrimSuffix(use, ".F.)") + ".T.)"
	})
}

// TestReadTrimmedBoundary writes each result, reads the file back and holds
// the solid that was read to the one that was written: the same counts of
// faces, edges and vertices, with the name and the unit of the file; the
// volume and the area of the written solid when the general Boolean takes
// the read solid as an operand, which also holds it to the Boolean's class
// and validation; and, written again, a file that measures as the first.
// It does the same with the file restated so that every edge runs against
// its curve, which the writer never writes.
func TestReadTrimmedBoundary(t *testing.T) {
	for _, c := range boundaryResults(t) {
		var written bytes.Buffer
		if err := step.WriteTrimmedBoundary(&written, c.solid, .001, "a part"); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		for _, v := range []struct{ name, file string }{
			{c.name, written.String()},
			{c.name + ", edges against their curves", boundaryAgainst(written.String())},
		} {
			t.Run(v.name, func(t *testing.T) {
				boundaryMeasured(t, []byte(v.file), c)
				m, err := step.ReadTrimmedBoundary(strings.NewReader(v.file), step.Limits{}, .001, boundaryOptions.Linear)
				if err != nil {
					t.Fatal(err)
				}
				if len(m.Bodies) != 1 || m.Bodies[0].Name != "a part" || m.LengthUnit != .001 {
					t.Fatalf("read %d bodies, name %q, unit %g", len(m.Bodies), m.Bodies[0].Name, m.LengthUnit)
				}
				got := m.Bodies[0].Solid
				if len(got.Faces()) != len(c.solid.Faces()) || len(got.Edges()) != len(c.solid.Edges()) || len(got.Vertices()) != len(c.solid.Vertices()) {
					t.Errorf("read %d faces, %d edges and %d vertices; wrote %d, %d and %d", len(got.Faces()), len(got.Edges()), len(got.Vertices()),
						len(c.solid.Faces()), len(c.solid.Edges()), len(c.solid.Vertices()))
				}
				r, err := boolean.Combine(context.Background(), got, trimmed.Solid{}, boolean.Union, boundaryOptions)
				if err != nil {
					t.Fatal(err)
				}
				if p := r.Properties; r.Solids != 1 || math.Abs(p.Volume-c.want.Volume) > 1e-9*c.want.Volume || math.Abs(p.Area-c.want.Area) > 1e-9*c.want.Area {
					t.Errorf("the read solid is %d solids of volume %.12g and area %.12g, the written one has %.12g and %.12g", r.Solids, p.Volume, p.Area, c.want.Volume, c.want.Area)
				}
				var again bytes.Buffer
				if err := step.WriteTrimmedBoundary(&again, got, .001, "a part"); err != nil {
					t.Fatal(err)
				}
				boundaryMeasured(t, again.Bytes(), c)
			})
		}
	}
}

// TestReadTrimmedBoundaryRefusals holds the reader to an error for a file
// that states no valid solid of its class. Each file is the through bore as
// written, with one thing changed in its text.
func TestReadTrimmedBoundaryRefusals(t *testing.T) {
	var written bytes.Buffer
	if err := step.WriteTrimmedBoundary(&written, boundaryResults(t)[1].solid, .001, "PlainSolid"); err != nil {
		t.Fatal(err)
	}
	file := written.String()
	vertex := regexp.MustCompile(`=VERTEX_POINT\('',(#[0-9]+)\)`).FindStringSubmatch(file)
	if vertex == nil {
		t.Fatal("no vertex in the file")
	}
	// change replaces old by new in the first line of the file that holds
	// the entity.
	change := func(entity, old, new string) func(t *testing.T) string {
		return func(t *testing.T) string {
			lines := strings.Split(file, "\n")
			for i, line := range lines {
				if strings.Contains(line, entity) && strings.Contains(line, old) {
					lines[i] = strings.Replace(line, old, new, 1)
					return strings.Join(lines, "\n")
				}
			}
			t.Fatalf("no %s with %s in the file", entity, old)
			return ""
		}
	}
	for _, c := range []struct {
		name      string
		file      func(t *testing.T) string
		tolerance float64
		want      error // nil for any error
	}{
		{"a surface of another kind", change("=CYLINDRICAL_SURFACE(", "=CYLINDRICAL_SURFACE(", "=SPHERICAL_SURFACE("), 1e-7, step.ErrTrimmedBoundary},
		{"a curve of another kind", change("=CIRCLE(", "=CIRCLE(", "=HYPERBOLA("), 1e-7, step.ErrTrimmedBoundary},
		{"a cylinder of another radius", change("=CYLINDRICAL_SURFACE(", ",1.)", ",1.5)"), 1e-7, step.ErrTrimmedBoundary},
		{"a second representation", func(*testing.T) string {
			return strings.Replace(file, "ENDSEC;\nEND-ISO", "#9000=SHAPE_REPRESENTATION('',(),#4);\nENDSEC;\nEND-ISO", 1)
		}, 1e-7, step.ErrTrimmedBoundary},
		{"a face turned", change("=ADVANCED_FACE(", ".T.)", ".F.)"), 1e-7, nil},
		{"an oriented edge turned", change("=ORIENTED_EDGE(", ".T.)", ".F.)"), 1e-7, nil},
		{"an edge run against its curve", change("=EDGE_CURVE(", ".T.)", ".F.)"), 1e-7, nil},
		{"a vertex moved", change(vertex[1]+"=CARTESIAN_POINT(", "('',(", "('',(0.25E1"), 1e-7, nil},
		{"not a STEP file", func(*testing.T) string { return "solid" }, 1e-7, nil},
		{"no tolerance", func(*testing.T) string { return file }, 0, geom.ErrInvalid},
	} {
		t.Run(c.name, func(t *testing.T) {
			m, err := step.ReadTrimmedBoundary(strings.NewReader(c.file(t)), step.Limits{}, .001, c.tolerance)
			if err == nil || (c.want != nil && !errors.Is(err, c.want)) {
				t.Fatalf("got %v and %d bodies, want %v", err, len(m.Bodies), c.want)
			}
		})
	}
	// A cylinder as an older writer of this package writes it has each rim
	// as one closed edge. The reader builds the edges the file states and
	// does not cut an arc, so the file is refused, and by name.
	frame, err := geom.NewFrame(geom.Vec3{X: 1}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	var round bytes.Buffer
	if err := step.WriteCylinders(&round, step.CylinderModel{LengthUnit: .001, Bodies: []step.CylinderBody{{Name: "c", Frame: frame, Radius: 1.5, Height: 4}}}); err != nil {
		t.Fatal(err)
	}
	if m, err := step.ReadTrimmedBoundary(bytes.NewReader(round.Bytes()), step.Limits{}, .001, 1e-7); !errors.Is(err, step.ErrTrimmedBoundary) || !strings.Contains(err.Error(), "half a turn") {
		t.Fatalf("read %d bodies from a cylinder with closed rims: %v", len(m.Bodies), err)
	}
}

// TestReadTrimmedBoundaryCrossingBores reads back boxes with two bores of
// equal radius that cross, cut in either order: an oblique bore across an
// upright one, and a blind bore whose floor the other bore clips. The
// walls of such a result are bounded by arcs of ellipses that run from
// seam to seam.
//
// On a cylinder the reader gives every use the angle its loop has reached
// and the height of its vertex, so a ruling has one angle and the uses
// that meet at a vertex have one point. Where it took the point from an
// arc's own evaluation, a ruling next to the arc was tilted by a unit in
// the last place, and trimmed.NewSolid refused four of these solids for a
// fold in its validation mesh.
func TestReadTrimmedBoundaryCrossingBores(t *testing.T) {
	type bores struct {
		name          string
		min, max      geom.Vec3
		first, second trimmed.Solid
	}
	var cases []bores
	for _, sign := range []float64{1, -1} {
		upright := boundaryCylinder(t, geom.Vec3{X: 10, Y: 8, Z: -2}, geom.Vec3{Z: 1}, 2, 44)
		oblique := boundaryCylinder(t, geom.Vec3{X: -2, Y: 8, Z: 20 - 16*sign}, geom.Vec3{X: 3, Z: 4 * sign}, 2, 44)
		name := "oblique bore that rises"
		if sign < 0 {
			name = "oblique bore that falls"
		}
		cases = append(cases, bores{name + ", cut first", geom.Vec3{}, geom.Vec3{X: 20, Y: 16, Z: 40}, oblique, upright}, bores{name + ", cut second", geom.Vec3{}, geom.Vec3{X: 20, Y: 16, Z: 40}, upright, oblique})
	}
	for _, depth := range []float64{4.8, 6, 7.2} {
		for _, from := range []float64{0, 12} {
			along := geom.Vec3{Z: 1}
			name := "floor clipped from below"
			if from > 0 {
				along, name = geom.Vec3{Z: -1}, "floor clipped from above"
			}
			through := boundaryCylinder(t, geom.Vec3{Y: 8, Z: 6}, geom.Vec3{X: 1}, 2, 20)
			blind := boundaryCylinder(t, geom.Vec3{X: 10, Y: 8, Z: from}, along, 2, depth)
			name = fmt.Sprintf("%s at depth %g", name, depth)
			cases = append(cases, bores{name + ", through bore first", geom.Vec3{}, geom.Vec3{X: 20, Y: 16, Z: 12}, through, blind}, bores{name + ", blind bore first", geom.Vec3{}, geom.Vec3{X: 20, Y: 16, Z: 12}, blind, through})
		}
	}
	// A pair along other coordinate axes. Read back, a midpoint of a seed
	// of one wall lands on a vertex of its trim at the least level of
	// trimmed.NewSolid's validation, and the solid is accepted at the next.
	cases = append(cases, bores{"bores along x, the oblique one cut first", geom.Vec3{X: 4, Y: -8, Z: 16}, geom.Vec3{X: 74, Y: 8, Z: 36},
		boundaryCylinder(t, geom.Vec3{X: 15, Z: 8}, geom.Vec3{X: 4, Z: 3}, 2, 150), boundaryCylinder(t, geom.Vec3{X: 2, Z: 26}, geom.Vec3{X: 1}, 2, 74)})
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			one, err := boolean.Combine(context.Background(), boundaryBox(t, c.min, c.max), c.first, boolean.Cut, boundaryOptions)
			if err != nil {
				t.Fatal(err)
			}
			two, err := boolean.Combine(context.Background(), one.Solid, c.second, boolean.Cut, boundaryOptions)
			if err != nil {
				t.Fatal(err)
			}
			var written bytes.Buffer
			if err := step.WriteTrimmedBoundary(&written, two.Solid, .001, "a part"); err != nil {
				t.Fatal(err)
			}
			m, err := step.ReadTrimmedBoundary(bytes.NewReader(written.Bytes()), step.Limits{}, .001, boundaryOptions.Linear)
			if err != nil {
				t.Fatal(err)
			}
			got := m.Bodies[0].Solid
			for fi, f := range got.Faces() {
				base, _ := parametric.SurfaceMapping(f.Surface)
				if _, ok := base.(geom.CylinderSurface); !ok {
					continue
				}
				for _, loop := range append([][]trimmed.Use{f.Outer}, f.Holes...) {
					for i, u := range loop {
						if _, ok := got.Edges()[u.Edge].Curve.(geom.Line); ok && u.UVStart.X != u.UVEnd.X {
							t.Errorf("face %d: the ruling of edge %d runs from angle %.17g to %.17g", fi, u.Edge, u.UVStart.X, u.UVEnd.X)
						}
						if next := loop[(i+1)%len(loop)]; u.UVEnd != next.UVStart {
							t.Errorf("face %d: edge %d ends at %v and edge %d starts at %v", fi, u.Edge, u.UVEnd, next.Edge, next.UVStart)
						}
					}
				}
			}
			r, err := boolean.Combine(context.Background(), got, trimmed.Solid{}, boolean.Union, boundaryOptions)
			if err != nil {
				t.Fatal(err)
			}
			if p, want := r.Properties, two.Properties; r.Solids != 1 || math.Abs(p.Volume-want.Volume) > 1e-9*want.Volume || math.Abs(p.Area-want.Area) > 1e-9*want.Area {
				t.Errorf("the read solid is %d solids of volume %.12g and area %.12g, the written one has %.12g and %.12g", r.Solids, p.Volume, p.Area, want.Volume, want.Area)
			}
			// The same lengths stated in centimetres are a part ten times
			// the size, and the reader gives it in the unit asked for.
			written.Reset()
			if err := step.WriteTrimmedBoundary(&written, two.Solid, .01, "a part"); err != nil {
				t.Fatal(err)
			}
			m, err = step.ReadTrimmedBoundary(bytes.NewReader(written.Bytes()), step.Limits{}, .001, boundaryOptions.Linear)
			if err != nil {
				t.Fatal(err)
			}
			if m.LengthUnit != .001 {
				t.Errorf("the solid is stated in the unit %g", m.LengthUnit)
			}
			r, err = boolean.Combine(context.Background(), m.Bodies[0].Solid, trimmed.Solid{}, boolean.Union, boundaryOptions)
			if err != nil {
				t.Fatal(err)
			}
			if p, v, a := r.Properties, 1000*two.Properties.Volume, 100*two.Properties.Area; r.Solids != 1 || math.Abs(p.Volume-v) > 1e-9*v || math.Abs(p.Area-a) > 1e-9*a {
				t.Errorf("read from centimetres, the solid is %d solids of volume %.12g and area %.12g, want %.12g and %.12g", r.Solids, p.Volume, p.Area, v, a)
			}
		})
	}
}
