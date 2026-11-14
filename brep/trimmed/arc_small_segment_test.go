package trimmed

import (
	"bytes"
	"context"
	"math"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestSmallCircularSegmentExtrusion(t *testing.T) {
	ctx := context.Background()
	frame, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	for _, sweep := range []float64{.53, -.53, .01, -.01} {
		for _, height := range []float64{2, -2} {
			arc := ExtrusionSegment{Center: geom.Vec2{X: 5, Y: 7}, Radius: 3, StartAngle: .37, Sweep: sweep}
			arc.Start = arcLoopPoint(arc, arc.StartAngle)
			arc.End = arcLoopPoint(arc, arc.StartAngle+sweep)
			input := []ExtrusionSegment{arc, {Start: arc.End, End: arc.Start}}
			r, err := ExtrudeArcLoop(ctx, frame, input, height, 1e-10)
			if err != nil {
				t.Fatal(sweep, height, err)
			}
			if input[0] != arc {
				t.Fatal("mutated caller arc")
			}
			want := 9 * (math.Abs(sweep) - math.Sin(math.Abs(sweep))) / 2
			if math.Abs(r.Area-want) > 1e-12 {
				t.Fatal("analytic area", r.Area, want)
			}
			foundStart, foundEnd := false, false
			for _, p := range r.Solid.Vertices() {
				if p == (geom.Vec3{X: arc.Start.X, Y: arc.Start.Y}) {
					foundStart = true
				}
				if p == (geom.Vec3{X: arc.End.X, Y: arc.End.Y}) {
					foundEnd = true
				}
			}
			if !foundStart || !foundEnd {
				t.Fatal("authored endpoints lost")
			}
			for _, f := range r.Solid.Faces() {
				if len(f.Outer) < 3 {
					t.Fatal("two-coedge face")
				}
			}
			mass, err := r.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
			if err != nil || math.Abs(mass.Volume-want*math.Abs(height)) > 1e-9 {
				t.Fatal("mass", mass.Volume, want, err)
			}
			mesh, err := r.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .01, Angle: .1, MaxTriangles: 10000, MaxSubdivisions: 128})
			if err != nil {
				t.Fatal("mesh", sweep, err)
			}
			if err := mesh.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
			var data bytes.Buffer
			if err := WriteJSON(&data, r.Solid); err != nil {
				t.Fatal(err)
			}
			restored, err := ReadJSON(&data, JSONLimits{})
			if err != nil {
				t.Fatal(err)
			}
			mass, err = restored.MassProperties(ctx, parametric.IntegrationOptions{})
			if err != nil || math.Abs(mass.Volume-want*math.Abs(height)) > 1e-9 {
				t.Fatal("reload", err)
			}
		}
	}
}
