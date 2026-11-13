package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"plainsolid/brep"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"testing"
)

func TestCylinderToActualRoundedFace(t *testing.T) {
	ctx := context.Background()
	for _, sign := range []float64{1, -1} {
		frame, _ := geom.FrameFromNormal(geom.Vec3{Z: 2 * sign}, geom.Vec3{Z: sign})
		profile := RoundedRectangle{Min: geom.Vec2{X: -3, Y: -2}, Max: geom.Vec2{X: 3, Y: 2}, Radii: [4]float64{.5, .5, .5, .5}}
		stock, e := ExtrudeRoundedRectangle(ctx, frame, profile, []ExtrusionCircle{{Center: geom.Vec2{X: 1}, Radius: .25}}, nil, 1)
		if e != nil {
			t.Fatal(e)
		}
		face := -1
		for i, f := range stock.faces {
			p, ok := f.Surface.(geom.PlaneSurface)
			if ok && p.Frame().Origin().Z == 2*sign {
				face = i
				break
			}
		}
		if face < 0 {
			t.Fatal("cap")
		}
		recognition, e := RecognizeRoundedPlanarFace(ctx, stock, face, geom.SurfacePatchEvaluationOptions{})
		if e != nil || len(recognition.Holes) != 1 {
			t.Fatal(recognition, e)
		}
		solid, proof, e := CylinderToRoundedFace(ctx, geom.Vec3{X: -1}, geom.Vec3{Z: sign}, .5, stock, face, geom.SurfacePatchEvaluationOptions{})
		if e != nil || proof.Height != 2 {
			t.Fatal(proof, e)
		}
		mass, e := solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(mass.Volume-math.Pi*.5) > 1e-8 {
			t.Fatal(mass, e)
		}
		var buffer bytes.Buffer
		if e := WriteJSON(&buffer, stock); e != nil {
			t.Fatal(e)
		}
		reloaded, e := ReadJSON(&buffer, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		if _, e := RecognizeRoundedPlanarFace(ctx, reloaded, face, geom.SurfacePatchEvaluationOptions{}); e != nil {
			t.Fatal(e)
		}
		if _, _, e := CylinderToRoundedFace(ctx, geom.Vec3{X: 1}, geom.Vec3{Z: sign}, .1, stock, face, geom.SurfacePatchEvaluationOptions{}); !errors.Is(e, brep.ErrTerminationUncovered) {
			t.Fatal("hole", e)
		}
		bad := stock
		bad.edges = append([]parametric.Edge(nil), stock.edges...)
		id := stock.faces[face].Outer[0].Edge
		curve := bad.edges[id].Curve.(surfaceTrimCurve)
		surface := curve.surface.(geom.CylinderSurface)
		f, r := surface.Parameters()
		curve.surface, _ = geom.NewCylinderSurface(f, r+.125)
		bad.edges[id].Curve = curve
		if _, e := RecognizeRoundedPlanarFace(ctx, bad, face, geom.SurfacePatchEvaluationOptions{}); e == nil {
			t.Fatal("wrong model radius")
		}
		profile.Radii[0] = .75
		unequal, e := ExtrudeRoundedRectangle(ctx, frame, profile, nil, nil, 1)
		if e != nil {
			t.Fatal(e)
		}
		if _, e := RecognizeRoundedPlanarFace(ctx, unequal, face, geom.SurfacePatchEvaluationOptions{}); e == nil {
			t.Fatal("separate radii silently equalized")
		}
		profile.Radii = [4]float64{}
		sharp, e := ExtrudeRoundedRectangle(ctx, frame, profile, nil, nil, 1)
		if e != nil {
			t.Fatal(e)
		}
		if _, _, e := CylinderToRoundedFace(ctx, geom.Vec3{}, geom.Vec3{Z: sign}, 1, sharp, face, geom.SurfacePatchEvaluationOptions{}); e != nil {
			t.Fatal("sharp rectangle", e)
		}
	}
}

func TestRoundedFaceRecognitionAllAxes(t *testing.T) {
	ctx := context.Background()
	for _, axis := range []geom.Vec3{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {Z: 1}, {Z: -1}} {
		frame, _ := geom.FrameFromNormal(axis.Scale(2), axis)
		p := RoundedRectangle{Min: geom.Vec2{X: -2, Y: -2}, Max: geom.Vec2{X: 2, Y: 2}, Radii: [4]float64{.5, .5, .5, .5}}
		stock, e := ExtrudeRoundedRectangle(ctx, frame, p, nil, nil, 1)
		if e != nil {
			t.Fatal(e)
		}
		face := -1
		for i, f := range stock.faces {
			surface, ok := f.Surface.(geom.PlaneSurface)
			if ok && surface.Frame().Origin() == axis.Scale(2) {
				face = i
				break
			}
		}
		if _, proof, e := CylinderToRoundedFace(ctx, geom.Vec3{}, axis, 1, stock, face, geom.SurfacePatchEvaluationOptions{}); e != nil || proof.Height != 2 {
			t.Fatal(axis, proof, e)
		}
		if _, e := RecognizeRoundedPlanarFace(ctx, stock, face, geom.SurfacePatchEvaluationOptions{MaxOperations: 1}); e == nil {
			t.Fatal("budget")
		}
	}
}
