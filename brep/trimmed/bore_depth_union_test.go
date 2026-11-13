package trimmed

import (
	"context"
	"errors"
	"math"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func depthUnionBox(t *testing.T) brep.Solid {
	t.Helper()
	s, e := brep.Box(geom.Vec3{X: -10, Y: -10}, geom.Vec3{X: 10, Y: 10, Z: 10})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func depthUnionCut(x, y, r float64) CylinderCut {
	return CylinderCut{Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: x, Y: y}, Direction: geom.Vec3{Z: 1}, Radius: r}
}
func TestDepthUnionThreeLevelChain(t *testing.T) {
	base := depthUnionBox(t)
	alpha := math.Acos(.75)
	lens := 8*alpha - 1.5*math.Sqrt(7)
	areas := []float64{12*math.Pi - 2*lens, 8*math.Pi - lens, 4 * math.Pi}
	perimeters := []float64{12*math.Pi - 16*alpha, 8*math.Pi - 8*alpha, 4 * math.Pi}
	removed, walls := 2*areas[0]+3*areas[1]+3*areas[2], 2*perimeters[0]+3*perimeters[1]+3*perimeters[2]
	for _, order := range [][3]int{{0, 1, 2}, {2, 0, 1}, {1, 2, 0}} {
		var cuts []CylinderCut
		var depths []float64
		for _, i := range order {
			cuts = append(cuts, depthUnionCut(float64(i-1)*3, 0, 2))
			depths = append(depths, []float64{2, 5, 8}[i])
		}
		body, h, e := BoreCylinderDepthUnion(base, cuts, depths, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		unionOracleNear(t, h.RemovedVolume, removed)
		unionOracleNear(t, h.SurfaceAreaDelta, walls)
		unionOracleNear(t, h.EntryRemovedArea, areas[0])
		unionOracleNear(t, h.ExitRemovedArea, 0)
		p, e := body.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil {
			t.Fatal(e)
		}
		unionOracleNear(t, p.Volume, 4000-removed)
		unionOracleNear(t, p.Area, 1600+walls)
		m, e := body.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .05, Angle: .1})
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
	}
}
func TestDepthUnionNestedAndDisconnectedFloors(t *testing.T) {
	base := depthUnionBox(t)
	body, h, e := BoreCylinderDepthUnion(base, []CylinderCut{depthUnionCut(0, 0, 3), depthUnionCut(0, 0, 2), depthUnionCut(0, 0, 1)}, []float64{2, 5, 8}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	unionOracleNear(t, h.RemovedVolume, 33*math.Pi)
	unionOracleNear(t, h.SurfaceAreaDelta, 30*math.Pi)
	if len(h.FloorAreas) != 3 {
		t.Fatal(h)
	}
	for i, want := range []float64{5 * math.Pi, 3 * math.Pi, math.Pi} {
		unionOracleNear(t, h.FloorAreas[i], want)
	}
	if len(body.Faces()[h.FloorFaces[0]].Holes) != 1 || len(body.Faces()[h.FloorFaces[1]].Holes) != 1 {
		t.Fatal("annular floor holes missing")
	}
	cuts := []CylinderCut{depthUnionCut(-4, 0, 2), depthUnionCut(-4, 0, 1), depthUnionCut(4, 0, 2), depthUnionCut(4, 0, 1)}
	body, h, e = BoreCylinderDepthUnion(base, cuts, []float64{2, 5, 2, 5}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if len(h.FloorFaces) != 4 {
		t.Fatal(h)
	}
	unionOracleNear(t, h.RemovedVolume, 22*math.Pi)
	unionOracleNear(t, h.SurfaceAreaDelta, 28*math.Pi)
	m, e := body.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .05, Angle: .1})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	if _, e = body.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .05, Angle: .1, MaxTriangles: 1}); !errors.Is(e, parametric.ErrBudget) {
		t.Fatalf("budget %v", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = body.Tessellate(ctx, parametric.TessellationOptions{Deflection: .05, Angle: .1}); !errors.Is(e, context.Canceled) {
		t.Fatalf("cancel %v", e)
	}
}
func TestDepthUnionDuplicateDepthAndUnsafeContacts(t *testing.T) {
	base := depthUnionBox(t)
	a := depthUnionCut(0, 0, 2)
	for _, depths := range [][]float64{{2, 6}, {6, 2}, {6, 6}} {
		_, h, e := BoreCylinderDepthUnion(base, []CylinderCut{a, a}, depths, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		unionOracleNear(t, h.RemovedVolume, 24*math.Pi)
		if len(h.FloorFaces) != 1 {
			t.Fatal(h)
		}
	}
	cases := []struct {
		cuts   []CylinderCut
		depths []float64
	}{
		{[]CylinderCut{a, depthUnionCut(1, 0, 1)}, []float64{3, 6}}, // internal tangent annular pinch
		{[]CylinderCut{a, depthUnionCut(4, 0, 2)}, []float64{3, 6}},
		{[]CylinderCut{a, depthUnionCut(2, 0, 2)}, []float64{3, 3 + 5e-10}},
		{[]CylinderCut{depthUnionCut(0, 0, 2), depthUnionCut(0, 0, 2-5e-10)}, []float64{3, 6}},
	}
	for _, tc := range cases {
		if _, _, e := BoreCylinderDepthUnion(base, tc.cuts, tc.depths, 1e-9); e == nil {
			t.Fatal("accepted unresolved contacts/depth/floor")
		}
	}
	// A shallow center disk may hide a ring at entry, but a later slab must not
	// leave an unqualified isolated material island.
	var ring []CylinderCut
	var depths []float64
	for _, p := range [][2]float64{{-3, -3}, {-3, 0}, {-3, 3}, {0, 3}, {3, 3}, {3, 0}, {3, -3}, {0, -3}} {
		ring = append(ring, depthUnionCut(p[0], p[1], 2))
		depths = append(depths, 6)
	}
	ring = append(ring, depthUnionCut(0, 0, 3))
	depths = append(depths, 2)
	if _, _, e := BoreCylinderDepthUnion(base, ring, depths, 1e-9); e == nil {
		t.Fatal("accepted later material-island footprint")
	}
}

func TestDepthUnionThreeLevelTripleCommonRegion(t *testing.T) {
	base := depthUnionBox(t)
	cuts := []CylinderCut{depthUnionCut(0, 0, 2), depthUnionCut(2, 0, 2), depthUnionCut(1, math.Sqrt(3), 2)}
	body, h, e := BoreCylinderDepthUnion(base, cuts, []float64{2, 5, 8}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	removed := 2*(6*math.Pi+4*math.Sqrt(3)) + 3*(16*math.Pi/3+2*math.Sqrt(3)) + 3*4*math.Pi
	walls := 2*6*math.Pi + 3*16*math.Pi/3 + 3*4*math.Pi
	unionOracleNear(t, h.RemovedVolume, removed)
	unionOracleNear(t, h.SurfaceAreaDelta, walls)
	p, e := body.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	unionOracleNear(t, p.Volume, 4000-removed)
	unionOracleNear(t, p.Area, 1600+walls)
	m, e := body.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .05, Angle: .1})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
}

func TestDepthUnionDisconnectedAreaUsesLocalAnchors(t *testing.T) {
	input := []depthDisk{{unionDisk: unionDisk{radius: 1}, depth: 2}, {unionDisk: unionDisk{center: geom.Vec2{X: 1e12}, radius: 1}, depth: 3}}
	a, e := newDepthArrangement(input, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	visible, e := a.exposed(1, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var list []directedUnionArc
	for i, on := range visible {
		if on {
			list = append(list, directedUnionArc{id: i})
		}
	}
	boundary, e := a.boundary(list, false, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(boundary.area-2*math.Pi) > 1e-12 {
		t.Fatalf("translated loop area %.17g", boundary.area)
	}
}
