package trimmed

import (
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestBoxSideCylinderThroughSignedFrames(t *testing.T) {
	ctx := context.Background()
	for _, n := range []geom.Vec3{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {Z: 1}, {Z: -1}} {
		f, _ := geom.FrameFromNormal(geom.Vec3{X: 28, Y: 7, Z: 16}, n)
		s, e := BoxSideCylinderThrough(ctx, f, 7, 13, 28, 2, 8, 3, 1e-9)
		if e != nil {
			t.Fatal(n, e)
		}
		area := 9*(math.Pi/2+math.Asin(2./3)) + 2*math.Sqrt(5)
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(p.Volume-(4800-8*area)) > 1e-7 {
			t.Fatal(p.Volume)
		}
		m, e := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .5, Angle: .1, MaxTriangles: 100000, MaxSubdivisions: 128})
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
	}
}
func TestBoxSideCylinderThroughRejectsTangencyAndOtherSides(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	for _, a := range []float64{-3, 3, math.Nextafter(3, 0)} {
		if _, e := BoxSideCylinderThrough(ctx, f, 7, 13, 28, a, 8, 3, 1e-9); e == nil {
			t.Fatal(a)
		}
	}
	if _, e := BoxSideCylinderThrough(ctx, f, 3, 13, 28, 2, 8, 3, 1e-9); e == nil {
		t.Fatal("second side contact accepted")
	}
}

func TestBoxSideCylinderBlindFloor(t *testing.T) {
	ctx := context.Background()
	f, err := geom.FrameFromNormal(geom.Vec3{X: 16, Y: 32, Z: 48}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	for _, inset := range []float64{-1, 0, 1} {
		const radius, depth = 3, 6
		s, err := BoxSideCylinderBlind(ctx, f, 10, 10, 19, inset, 20, radius, depth, 1e-9)
		if err != nil {
			t.Fatal(inset, err)
		}
		sweep := math.Pi + 2*math.Asin(inset/radius)
		chord := 2 * math.Sqrt(radius*radius-inset*inset)
		area := (radius*radius*sweep + inset*chord) / 2
		wantV := 400*(19+inset) - depth*area
		wantA := 800 + 80*(19+inset) + depth*(radius*sweep-chord)
		p, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
		if err != nil || math.Abs(p.Volume-wantV) > 1e-7 || math.Abs(p.Area-wantA) > 1e-7 {
			t.Fatalf("%g: integrated measures %+v, want volume %g and area %g: %v", inset, p, wantV, wantA, err)
		}
		floor, err := s.PlanarFaceArea(1)
		if err != nil || math.Abs(floor-area) > 1e-7 || !s.Faces()[1].Reversed {
			t.Fatalf("%g: wrong blind floor %g: %v", inset, floor, err)
		}
	}
}
