package plaincadbridge

import (
	"math"
	"testing"
)

// The tests of this file hold a part cut by the general Boolean to what the
// same part answers when a legacy path cut it: the bore callouts of its
// drawing, the volume its mesh keeps, and its bounds.

// generalParityParts builds one part on a default bridge and on a general
// bridge: a box and one cylindrical tool cut from it.
func generalParityParts(t *testing.T, min, max, origin, axis vector, radius, height float64) (legacy, general stored) {
	t.Helper()
	var parts [2]stored
	for i, b := range []*Bridge{newLegacy(), newGeneral()} {
		stock := box(t, b, min, max)
		tool := circle(t, b, origin, axis, radius, height)
		cut := shape(t, b, map[string]any{"op": "cut", "a": stock.Handle, "b": tool.Handle})
		parts[i] = b.shapes[cut.Handle]
	}
	if a := parts[0].analytic; a != nil && a.general || parts[1].analytic == nil || !parts[1].analytic.general {
		t.Fatal("the default bridge did not cut with a legacy path, or the general bridge not with the general Boolean")
	}
	return parts[0], parts[1]
}

func TestGeneralDrawingBores(t *testing.T) {
	for _, c := range []struct {
		name         string
		origin, axis vector
		height       float64
	}{
		{"through", vector{5, 5, 0}, vector{0, 0, 1}, 10},
		{"blind", vector{5, 5, 10}, vector{0, 0, -1}, 6},
	} {
		t.Run(c.name, func(t *testing.T) {
			legacy, general := generalParityParts(t, vector{}, vector{20, 10, 10}, c.origin, c.axis, 2, c.height)
			want, err := legacy.drawingGeometry()
			if err != nil {
				t.Fatal(err)
			}
			got, err := general.drawingGeometry()
			if err != nil {
				t.Fatal(err)
			}
			if len(want.Bores) != 1 || len(got.Bores) != 1 || len(got.Warnings) != 0 {
				t.Fatalf("bores %+v and warnings %v, where the legacy part has %+v", got.Bores, got.Warnings, want.Bores)
			}
			g, w := got.Bores[0], want.Bores[0]
			near(t, g.Diameter, w.Diameter)
			near(t, g.Length, w.Length)
			for i := range 3 {
				near(t, float64(g.Start[i]), float64(w.Start[i]))
				near(t, float64(g.Direction[i]), float64(w.Direction[i]))
			}
		})
	}
	// On a default bridge, a part that the fallback cut keeps the callouts
	// of its bores: the one a legacy path cut before, and one cut after.
	b := New()
	type q = map[string]any
	stock := box(t, b, vector{}, vector{20, 10, 10})
	first := circle(t, b, vector{5, 5, 0}, vector{0, 0, 1}, 2, 10)
	bored := shape(t, b, q{"op": "cut", "a": stock.Handle, "b": first.Handle})
	slot := box(t, b, vector{12, -1, 6}, vector{16, 11, 11})
	slotted := declined(t, b, "", "cut", bored.Handle, slot.Handle)
	second := circle(t, b, vector{10, 2, 0}, vector{0, 0, 1}, 1, 10)
	twice := shape(t, b, q{"op": "cut", "a": slotted.Handle, "b": second.Handle})
	drawing, err := b.shapes[twice.Handle].drawingGeometry()
	if err != nil {
		t.Fatal(err)
	}
	if len(drawing.Bores) != 2 || len(drawing.Warnings) != 0 || drawing.Bores[0].Diameter != 4 || drawing.Bores[1].Diameter != 2 {
		t.Fatalf("bores %+v and warnings %v of a slotted part with two bores", drawing.Bores, drawing.Warnings)
	}
}

func TestGeneralMeshKeepsVolume(t *testing.T) {
	for _, c := range []struct {
		name         string
		origin, axis vector
		height       float64
	}{
		{"through", vector{5, 5, 0}, vector{0, 0, 1}, 10},
		{"blind", vector{5, 5, 10}, vector{0, 0, -1}, 6},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, general := generalParityParts(t, vector{}, vector{20, 10, 10}, c.origin, c.axis, 2, c.height)
			m, err := general.tessellate(request{})
			if err != nil {
				t.Fatal(err)
			}
			if err := m.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
			volume := general.result.GeometryAssertions.Volume
			if off := math.Abs(float32MeshVolume(m) - volume); off >= volume*meshVolumeBudget {
				t.Fatalf("the mesh is off the volume %.9g by %.3g, over the budget of %.3g", volume, off, volume*meshVolumeBudget)
			}
		})
	}
}

func TestGeneralBoundsEndAtFacePlanes(t *testing.T) {
	// A quarter bore along an edge of the stock: its arcs end on two faces
	// of the stock, at vertices that rounding puts off those faces.
	legacy, general := generalParityParts(t, vector{0, 20, 0}, vector{10, 30, 10}, vector{0, 35, 0}, vector{0, -1, 0}, 2, 40)
	if want := (Bounds{vector{0, 20, 0}, vector{10, 30, 10}}); legacy.result.Bounds != want || general.result.Bounds != want {
		t.Fatalf("bounds %+v on the general bridge and %+v on the default bridge, want %+v", general.result.Bounds, legacy.result.Bounds, want)
	}
}

// TestGeneralMeshRefinesToTheBudget holds the mesh of a result with
// several bores to the volume budget, and to no more facets than that
// takes: its mesh at the angle asked for is over the budget, the mesh the
// bridge gives is within it, and it has fewer triangles than a mesh at
// fineMeshAngle, which the budget does not need here.
func TestGeneralMeshRefinesToTheBudget(t *testing.T) {
	b := New()
	type q = map[string]any
	stock := box(t, b, vector{}, vector{40, 20, 10})
	bored := shape(t, b, q{"op": "cutMany", "a": stock.Handle, "tools": []string{
		circle(t, b, vector{10, 10, 0}, vector{0, 0, 1}, 2, 10).Handle,
		circle(t, b, vector{30, 10, 0}, vector{0, 0, 1}, 2, 10).Handle,
	}})
	s := b.shapes[bored.Handle]
	volume := bored.GeometryAssertions.Volume
	// A facet angle of 0.4 puts 32 facets round a bore. The bores are 3%
	// of the volume, and 32 facets lose more of that than the budget has,
	// 64 less, and fineMeshAngle asks for 256.
	var asked request
	asked.LinearDeflection.set, asked.LinearDeflection.value = true, .5
	asked.AngularDeflection.set, asked.AngularDeflection.value = true, .4
	options, err := tessellationOptions(asked)
	if err != nil {
		t.Fatal(err)
	}
	coarse, err := s.analytic.tessellate(options)
	if err != nil {
		t.Fatal(err)
	}
	if off := math.Abs(float32MeshVolume(coarse) - volume); off < volume*meshVolumeBudget {
		t.Fatalf("the mesh at the angle asked for is within the budget, off by %.3g: the part does not exercise a second mesh", off)
	}
	options.Angle = fineMeshAngle
	finest, err := s.analytic.tessellate(options)
	if err != nil {
		t.Fatal(err)
	}
	m, err := s.tessellate(asked)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	if off := math.Abs(float32MeshVolume(m) - volume); off >= volume*meshVolumeBudget {
		t.Fatalf("the mesh is off the volume %.9g by %.3g, over the budget of %.3g", volume, off, volume*meshVolumeBudget)
	}
	if len(m.Triangles) <= len(coarse.Triangles) || len(m.Triangles) >= len(finest.Triangles) {
		t.Fatalf("the mesh has %d triangles; the mesh at the angle asked for has %d and the finest %d", len(m.Triangles), len(coarse.Triangles), len(finest.Triangles))
	}
}
