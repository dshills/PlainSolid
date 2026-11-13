package trimmed

import (
	"context"
	"math"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"strings"
	"testing"
)

func TestBoreUnionTripleCommonRegion(t *testing.T) {
	base, err := brep.Box(geom.Vec3{X: -10, Y: -10}, geom.Vec3{X: 10, Y: 10, Z: 8})
	if err != nil {
		t.Fatal(err)
	}
	points := []geom.Vec3{{}, {X: 2}, {X: 1, Y: math.Sqrt(3)}}
	for _, depth := range []float64{0, 3} {
		var cuts []CylinderCut
		for _, p := range points {
			cuts = append(cuts, CylinderCut{Entry: 0, Exit: 1, AxisPoint: p, Direction: geom.Vec3{Z: 1}, Radius: 2})
		}
		body, h, err := BoreCylinderUnion(base, cuts, depth, 1e-9)
		if err != nil {
			t.Fatal(err)
		}
		// Three equal disks centered at an equilateral triangle of side radius.
		// Inclusion-exclusion includes the nonzero Reuleaux triple intersection.
		area, perimeter := 6*math.Pi+4*math.Sqrt(3), 6*math.Pi
		unionOracleNear(t, h.Area, area)
		unionOracleNear(t, h.Perimeter, perimeter)
		actual, err := body.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil {
			t.Fatal(err)
		}
		unionOracleNear(t, actual.Volume, 3200-area*h.Depth)
		mesh, err := body.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .03, Angle: .1})
		if err != nil {
			t.Fatal(err)
		}
		if err = mesh.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestBoreUnionSimplificationsAndRejections(t *testing.T) {
	base, err := brep.Box(geom.Vec3{X: -10, Y: -10}, geom.Vec3{X: 10, Y: 10, Z: 8})
	if err != nil {
		t.Fatal(err)
	}
	disk := func(x, y, r float64) CylinderCut {
		return CylinderCut{Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: x, Y: y}, Direction: geom.Vec3{Z: 1}, Radius: r}
	}
	for _, cuts := range [][]CylinderCut{{disk(0, 0, 2), disk(0, 0, 2)}, {disk(0, 0, 2), disk(.5, 0, 1)}, {disk(0, 0, 2), disk(1, 0, 1)}} {
		body, h, err := BoreCylinderUnion(base, cuts, 0, 1e-9)
		if err != nil {
			t.Fatal(err)
		}
		unionOracleNear(t, h.Area, 4*math.Pi)
		unionOracleNear(t, h.Perimeter, 4*math.Pi)
		m, err := body.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .05, Angle: .1})
		if err != nil {
			t.Fatal(err)
		}
		if err = m.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		cuts   []CylinderCut
		reason string
	}{
		{[]CylinderCut{disk(0, 0, 2), disk(4, 0, 2)}, "tangent"},
		{[]CylinderCut{disk(0, 0, 2), disk(math.Nextafter(4, 0), 0, 2)}, "near-tangent"},
		{[]CylinderCut{disk(0, 0, 2), disk(math.Nextafter(4, math.Inf(1)), 0, 2)}, ""},
		{[]CylinderCut{disk(0, 0, 2), disk(2, 0, 2), disk(1, math.Sqrt(3)-2, 2)}, "triple"},
	}
	var ring []CylinderCut
	for _, p := range [][2]float64{{-3, -3}, {-3, 0}, {-3, 3}, {0, 3}, {3, 3}, {3, 0}, {3, -3}, {0, -3}} {
		ring = append(ring, disk(p[0], p[1], 2))
	}
	cases = append(cases, struct {
		cuts   []CylinderCut
		reason string
	}{ring, "island"})
	for _, tc := range cases {
		if _, _, err := BoreCylinderUnion(base, tc.cuts, 0, 1e-9); err == nil || !strings.Contains(err.Error(), tc.reason) {
			t.Fatalf("want%s rejection, got%v", tc.reason, err)
		}
	}
	// Independent components are valid, unlike a hole inside one footprint.
	body, h, err := BoreCylinderUnion(base, []CylinderCut{disk(-4, 0, 1), disk(4, 0, 1)}, 3, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.FloorFaces) != 2 {
		t.Fatal(h.FloorFaces)
	}
	m, err := body.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .05, Angle: .1})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
}

func TestBoreUnionExactRelationsAndLimits(t *testing.T) {
	a := unionDisk{radius: 2}
	for _, tc := range []struct {
		x    float64
		sum  bool
		want int
	}{{math.Nextafter(4, 0), true, -1}, {4, true, 0}, {math.Nextafter(4, math.Inf(1)), true, 1}} {
		b := unionDisk{center: geom.Vec2{X: tc.x}, radius: 2}
		if got := circleRelation(a, b, tc.sum); got != tc.want {
			t.Fatalf("exactrelation %g: %d", tc.x, got)
		}
	}
	base, err := brep.Box(geom.Vec3{X: -10, Y: -10}, geom.Vec3{X: 10, Y: 10, Z: 8})
	if err != nil {
		t.Fatal(err)
	}
	cut := CylinderCut{Entry: 0, Exit: 1, Direction: geom.Vec3{Z: 1}, Radius: 2}
	cuts := make([]CylinderCut, 32)
	for i := range cuts {
		cuts[i] = cut
	}
	_, h, err := BoreCylinderUnion(base, cuts, 0, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	unionOracleNear(t, h.Area, 4*math.Pi)
	if _, _, err = BoreCylinderUnion(base, append(cuts, cut), 0, 1e-9); err == nil {
		t.Fatal("accepted more than32 inputs")
	}
	// Just outside internal tangency has a real tiny crescent: reject unresolved
	// topology rather than silently classifying it as a contained tool.
	smaller := cut
	smaller.Radius = 1
	smaller.AxisPoint.X = math.Nextafter(1, math.Inf(1))
	if _, _, err = BoreCylinderUnion(base, []CylinderCut{cut, smaller}, 0, 1e-9); err == nil {
		t.Fatal("dropped unresolved tiny crescent")
	}
	// Diagonal disks have overlapping AABBs but provably separate circles.
	second := cut
	second.AxisPoint = geom.Vec3{X: 3, Y: 3}
	body, h, err := BoreCylinderUnion(base, []CylinderCut{cut, second}, 0, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	unionOracleNear(t, h.Area, 8*math.Pi)
	m, err := body.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .05, Angle: .1})
	if err != nil {
		t.Fatal(err)
	}
	if err = m.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	if _, err = body.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .05, Angle: .1, MaxTriangles: 1}); err == nil {
		t.Fatal("ignored mesh budget")
	}
}

func TestBoreUnionScaleAndTranslation(t *testing.T) {
	area, perimeter := 16*math.Pi/3+2*math.Sqrt(3), 16*math.Pi/3
	for _, scale := range []float64{.001, 1, 1000} {
		shift := geom.Vec3{X: 1e5 * scale, Y: -1e5 * scale, Z: 10 * scale}
		base, err := brep.Box(shift.Add(geom.Vec3{X: -10 * scale, Y: -10 * scale}), shift.Add(geom.Vec3{X: 10 * scale, Y: 10 * scale, Z: 8 * scale}))
		if err != nil {
			t.Fatal(err)
		}
		var cuts []CylinderCut
		for _, x := range []float64{-1, 1} {
			cuts = append(cuts, CylinderCut{Entry: 0, Exit: 1, AxisPoint: shift.Add(geom.Vec3{X: x * scale}), Direction: geom.Vec3{Z: 1}, Radius: 2 * scale})
		}
		body, h, err := BoreCylinderUnion(base, cuts, 0, 1e-9*scale)
		if err != nil {
			t.Fatalf("scale %g: %v", scale, err)
		}
		unionOracleNear(t, h.Area/(scale*scale), area)
		unionOracleNear(t, h.Perimeter/scale, perimeter)
		m, err := body.Tessellate(context.Background(), parametric.TessellationOptions{Deflection: .05 * scale, Angle: .1})
		if err != nil {
			t.Fatal(err)
		}
		if err = m.ValidateSolid(); err != nil {
			t.Fatal(err)
		}
	}
}
