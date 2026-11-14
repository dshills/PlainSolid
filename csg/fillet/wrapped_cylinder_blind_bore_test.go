package fillet

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestBlindBorePreservesRoundedCylinderSupports(t *testing.T) {
	ctx := context.Background()
	o := geom.SurfacePatchOptions{}
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 4, 8, o)
	if e != nil {
		t.Fatal(e)
	}
	source, _, e := trimmed.NewWrappedCylinder(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	source, _, e = trimmed.FilletWrappedCylinderRims(ctx, source, .5, 1, o)
	if e != nil {
		t.Fatal(e)
	}
	faces := source.Faces()
	faces = append(faces[2:], faces[:2]...)
	source, e = trimmed.NewSolid(source.Vertices(), source.Edges(), faces, source.Tolerance())
	if e != nil {
		t.Fatal(e)
	}
	originalV, originalE, originalF := source.Vertices(), source.Edges(), source.Faces()
	before, e := source.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil {
		t.Fatal(e)
	}
	for _, fromUpper := range []bool{false, true} {
		result, e := BlindBoreWrappedRoundedCylinder(ctx, source, geom.Vec2{X: .5, Y: .25}, 1, 3, fromUpper, o)
		if e != nil {
			t.Fatal(e)
		}
		out := result.Solid
		if out.Topology().Faces != source.Topology().Faces+2 || out.Topology().EulerCharacteristic != 2 {
			t.Fatal(out.Topology())
		}
		if !reflect.DeepEqual(originalV, out.Vertices()[:len(originalV)]) || !reflect.DeepEqual(originalE, out.Edges()[:len(originalE)]) {
			t.Fatal("original entity identity changed")
		}
		for i, f := range out.Faces()[:len(originalF)] {
			if f.Surface != originalF[i].Surface || !reflect.DeepEqual(f.Outer, originalF[i].Outer) || f.Reversed != originalF[i].Reversed || !reflect.DeepEqual(result.SourceFaces[i], []int{i}) {
				t.Fatal("original support/face changed", i)
			}
		}
		if len(result.SourceFaces[result.BoreFace]) != 0 || len(result.SourceFaces[result.FloorFace]) != 0 {
			t.Fatal("new wall falsely attributed")
		}
		for end, id := range result.SourceCaps {
			wantHoles := 0
			if (end == 1) == fromUpper {
				wantHoles = 1
			}
			if len(out.Faces()[id].Holes) != wantHoles || len(result.BoreRims[end]) != 4 {
				t.Fatal(result)
			}
		}
		after, e := out.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(after.Volume-(before.Volume-3*math.Pi)) > 1e-7 || math.Abs(after.Area-(before.Area+6*math.Pi)) > 1e-7 {
			t.Fatal(before, after)
		}
		if math.Abs(after.Centroid.X+.5*3*math.Pi/after.Volume) > 1e-7 || math.Abs(after.Centroid.Y+.25*3*math.Pi/after.Volume) > 1e-7 {
			t.Fatal(after.Centroid)
		}
		var b bytes.Buffer
		if e = trimmed.WriteJSON(&b, out); e != nil {
			t.Fatal(e)
		}
		back, e := trimmed.ReadJSONContext(ctx, &b, trimmed.JSONLimits{})
		if e != nil || back.Topology().Faces != out.Topology().Faces {
			t.Fatal(e)
		}
		floorHeight, removedZ := 3.0, 1.5
		if fromUpper {
			floorHeight, removedZ = 5, 6.5
		}
		floor := out.Faces()[result.FloorFace]
		plane, ok := floor.Surface.(geom.PlaneSurface)
		if !ok || plane.Frame().Origin().Z != floorHeight || result.FloorZ.Cmp(new(big.Rat).SetFloat64(floorHeight)) != 0 || floor.Reversed == fromUpper {
			t.Fatal("donor floor orientation/support", floor, result.FloorZ)
		}
		expectedZ := (before.Volume*before.Centroid.Z - 3*math.Pi*removedZ) / after.Volume
		if math.Abs(after.Centroid.Z-expectedZ) > 1e-7 {
			t.Fatal("independent axial first moment", after.Centroid.Z, expectedZ)
		}
		for _, depth := range []float64{0, .5, 7.5, 8, 9, math.NaN()} {
			bad, e := BlindBoreWrappedRoundedCylinder(ctx, source, geom.Vec2{}, 1, depth, fromUpper, o)
			if e == nil || bad.Solid.Topology().Faces != 0 {
				t.Fatal("depth boundary", depth, fromUpper, e)
			}
		}
		shoulderDepth := .5
		if fromUpper {
			shoulderDepth = 1
		}
		if bad, e := BlindBoreWrappedRoundedCylinder(ctx, source, geom.Vec2{}, 1, shoulderDepth, fromUpper, o); e == nil || bad.Solid.Topology().Faces != 0 {
			t.Fatal("exact blend shoulder accepted", e)
		}
		if fromUpper {
			if bad, e := BlindBoreWrappedRoundedCylinder(ctx, source, geom.Vec2{}, 1, math.Nextafter(3, 4), true, o); e == nil || bad.Solid.Topology().Faces != 0 {
				t.Fatal("nonrepresentable floor accepted", e)
			}
		}
		// Clearance also checks the narrower opposite flat cap for lower entry.
		if bad, e := BlindBoreWrappedRoundedCylinder(ctx, source, geom.Vec2{X: 1}, 2, 3, fromUpper, o); e == nil || bad.Solid.Topology().Faces != 0 {
			t.Fatal("narrower cap clearance", e)
		}
	}

	if !reflect.DeepEqual(source.Faces(), originalF) {
		t.Fatal("input mutated")
	}
}
