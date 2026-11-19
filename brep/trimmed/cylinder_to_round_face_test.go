//go:build research

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

func TestCylinderToRecognizedRoundFace(t *testing.T) {
	ctx := context.Background()
	frame, _ := geom.NewFrame(geom.Vec3{Z: 2}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	stock, e := ExtrudeDiskWithHoles(ctx, frame, ExtrusionCircle{Radius: 4}, []ExtrusionCircle{{Center: geom.Vec2{X: 1}, Radius: .5}}, nil, 1)
	if e != nil {
		t.Fatal(e)
	}
	face := -1
	for i, f := range stock.faces {
		p, ok := f.Surface.(geom.PlaneSurface)
		if ok && p.Frame().Origin().Z == 2 {
			face = i
			break
		}
	}
	if face < 0 {
		t.Fatal("cap")
	}
	recognition, e := RecognizeCircularPlanarFace(ctx, stock, face, geom.SurfacePatchEvaluationOptions{})
	if e != nil || len(recognition.Holes) != 1 {
		t.Fatal(recognition, e)
	}
	solid, proof, e := CylinderToRoundFace(ctx, geom.Vec3{X: -1}, geom.Vec3{Z: 2}, 1, stock, face, geom.SurfacePatchEvaluationOptions{})
	if e != nil || proof.Height != 2 || len(proof.Holes) != 1 {
		t.Fatal(proof, e)
	}
	mass, e := solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(mass.Volume-2*math.Pi) > 1e-8 {
		t.Fatal(mass, e)
	}
	var buffer bytes.Buffer
	if e = WriteJSON(&buffer, stock); e != nil {
		t.Fatal(e)
	}
	reloaded, e := ReadJSON(&buffer, JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = RecognizeCircularPlanarFace(ctx, reloaded, face, geom.SurfacePatchEvaluationOptions{}); e != nil {
		t.Fatal("reload recognition", e)
	}
	if _, _, e := CylinderToRoundFace(ctx, geom.Vec3{X: 1}, geom.Vec3{Z: 1}, .25, stock, face, geom.SurfacePatchEvaluationOptions{}); !errors.Is(e, brep.ErrTerminationUncovered) {
		t.Fatal("hole", e)
	}
	if _, _, e := CylinderToRoundFace(ctx, geom.Vec3{}, geom.Vec3{Z: 1}, .5, stock, face, geom.SurfacePatchEvaluationOptions{}); e != nil {
		t.Fatal("hole tangency", e)
	}
	// Tamper actual model geometry while retaining every circular trim claim.
	bad := stock
	bad.edges = append([]parametric.Edge(nil), stock.edges...)
	edgeID := stock.faces[face].Outer[0].Edge
	curve := bad.edges[edgeID].Curve.(surfaceTrimCurve)
	cyl := curve.surface.(geom.CylinderSurface)
	cf, r := cyl.Parameters()
	curve.surface, _ = geom.NewCylinderSurface(cf, r+.25)
	bad.edges[edgeID].Curve = curve
	if _, e := RecognizeCircularPlanarFace(ctx, bad, face, geom.SurfacePatchEvaluationOptions{}); e == nil {
		t.Fatal("claimed trims bypassed model proof")
	}
	if _, e := RecognizeCircularPlanarFace(ctx, stock, face, geom.SurfacePatchEvaluationOptions{MaxOperations: 1}); e == nil {
		t.Fatal("budget")
	}
}

func TestRecognizedRoundFaceAxisFrames(t *testing.T) {
	ctx := context.Background()
	for _, axis := range []geom.Vec3{{X: 1}, {X: -1}, {Y: 1}, {Y: -1}, {Z: 1}, {Z: -1}} {
		f, e := geom.FrameFromNormal(axis.Scale(2), axis)
		if e != nil {
			t.Fatal(e)
		}
		stock, e := ExtrudeDiskWithHoles(ctx, f, ExtrusionCircle{Radius: 2}, nil, nil, 1)
		if e != nil {
			t.Fatal(e)
		}
		face := -1
		for i, item := range stock.faces {
			p, ok := item.Surface.(geom.PlaneSurface)
			if ok && p.Frame().Origin() == axis.Scale(2) {
				face = i
				break
			}
		}
		if face < 0 {
			t.Fatal("cap")
		}
		_, proof, e := CylinderToRoundFace(ctx, geom.Vec3{}, axis, 2, stock, face, geom.SurfacePatchEvaluationOptions{})
		if e != nil || proof.Height != 2 || proof.Outer.Relation != geom.CircularProfilesCoincident {
			t.Fatal(axis, proof, e)
		}
	}
}
