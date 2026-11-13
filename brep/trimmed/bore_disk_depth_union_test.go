package trimmed

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestBoreDiskDepthUnionOpposed(t *testing.T) {
	ctx := context.Background()
	frame, err := geom.FrameFromNormal(geom.Vec3{X: 10, Y: 20, Z: 3}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	outer := ExtrusionCircle{Center: geom.Vec2{Y: 2}, Radius: 6}
	blind := []ExtrusionBlindHole{
		{ExtrusionCircle: ExtrusionCircle{Center: geom.Vec2{X: -1, Y: 2}, Radius: 2}, Depth: 6},
		{ExtrusionCircle: ExtrusionCircle{Center: geom.Vec2{X: 1, Y: 2}, Radius: 2}, Depth: 5, OppositeCap: true},
	}
	lens := 8*math.Pi/3 - 2*math.Sqrt(3)
	wantRemoved := 44*math.Pi - 3*lens
	wantAreaDelta := 44*math.Pi - 8*math.Pi - 2*lens
	for _, height := range []float64{8, -8} {
		solid, proof, err := BoreDiskDepthUnion(ctx, frame, outer, nil, blind, height)
		if err != nil {
			t.Fatal(height, err)
		}
		if len(proof.FloorFaces) != 2 || math.Abs(proof.RemovedVolume-wantRemoved) > 1e-8 || math.Abs(proof.SurfaceAreaDelta-wantAreaDelta) > 1e-8 {
			t.Fatalf("%g: incorrect layer measures: %+v", height, proof)
		}
		properties, err := solid.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-10})
		if err != nil || math.Abs(properties.Volume-(288*math.Pi-wantRemoved)) > 1e-7 || math.Abs(properties.Area-(168*math.Pi+wantAreaDelta)) > 1e-7 {
			t.Fatalf("%g: wrong integrated measures: %+v: %v", height, properties, err)
		}
		mesh, err := solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .05, Angle: .1, MaxSubdivisions: 128, MaxTriangles: 200000})
		if err != nil {
			t.Fatal(height, err)
		}
		if err := mesh.ValidateSolid(); err != nil {
			t.Fatal(height, err)
		}
	}
}
