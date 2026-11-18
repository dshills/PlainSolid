package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
)

func authoredPlacedParabolaTwoCirclesStock(t *testing.T) [3]*geom.BSplineSurface {
	t.Helper()
	return placeParabolaTwoCirclesTestStock(t, parabolaTwoCirclesStock(t, 0, twoCirclePrimaryStock(), false))
}

func placeParabolaTwoCirclesTestStock(t *testing.T, contacts [3]*geom.BSplineSurface) [3]*geom.BSplineSurface {
	t.Helper()
	var placed [3]*geom.BSplineSurface
	for i, source := range contacts {
		data := source.Data()
		for j, p := range data.Poles {
			// World poles are authored directly in the rotated and translated stock.
			data.Poles[j] = geom.Vec3{X: 5 - p.Z, Y: p.X - 3, Z: 17.0/8 - p.Y}
		}
		var err error
		placed[i], err = geom.NewBSplineSurfaceWithExactWeights(data.DegreeU, data.DegreeV, data.CountU, data.CountV,
			data.Poles, data.KnotsU, data.KnotsV, data.Weights)
		if err != nil {
			t.Fatal(err)
		}
	}
	return placed
}

func TestPlacedParabolaTwoCirclesOriginalCutDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	source := authoredPlacedParabolaTwoCirclesStock(t)
	volumes := [2][]float64{
		{90040463765.0 / 44220983279616, 39745125.0 * math.Pi / 17592186044416, 259794325.0 * math.Pi / 2251799813685248},
		{20635608245.0/44220983279616 + 5347170325.0*math.Pi/2251799813685248},
	}
	for order := range 2 {
		cut, err := CutPlacedParabolaTwoCirclesSplineGraphs(ctx, source[0], source[1+order], source[2-order], 0, 1e-9, ParabolaCircleGraphOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if len(cut.world) != [2]int{3, 1}[order] {
			t.Fatal("wrong placed component count", len(cut.world))
		}
		for i, part := range cut.Components() {
			h := cut.History()[i]
			if part.faces[h.RoofFaces[0]].Surface != source[1+order] || part.faces[h.RoofFaces[1]].Surface != source[2-order] {
				t.Fatal("did not retain both original world owners")
			}
		}
		if order == 1 {
			cut = reopenPlacedParabolaTwoCircles(t, cut)
		}
		displays, err := cut.TessellateCertified(ctx, ParabolaTwoCirclesMeshOptions{NormalTurn: .2})
		if err != nil || len(displays) != len(cut.world) {
			t.Fatal(len(displays), err)
		}
		for i, display := range displays {
			s := cut.world[i]
			want := volumes[order][i]
			lower, _ := display.SourceVolume[0].Float64()
			upper, _ := display.SourceVolume[1].Float64()
			volume, _ := display.SignedDisplayVolume.Float64()
			distance, _ := display.DistanceUpper.Float64()
			verifyParabolaTwoCirclesSourceTurnInChart(t, s, display, .2, func(p geom.Vec3) geom.Vec2 {
				return geom.Vec2{X: p.Y + 3, Y: 17.0/8 - p.Z}
			})
			turnSquared, _ := display.SquaredSourceNormalTurn.Float64()
			euler := [2]int{2, -2}[order]
			if math.Abs(lower-want) > want*1e-14 || math.Abs(upper-want) > want*1e-14 ||
				math.Abs(volume-want) > want*.02 || distance <= 0 || distance > .05 ||
				turnSquared <= 0 || turnSquared > .2*.2 || !display.Collision.Complete ||
				len(display.Collision.Contacts) != 0 || display.Mesh.ValidateSolid() != nil ||
				s.Topology().EulerCharacteristic != euler {
				t.Fatal("failed world bounds or closure", display.Region, volume, want, distance, turnSquared)
			}
			for id, v := range s.Vertices() {
				if p := display.Mesh.Vertices[display.OriginalVertices[id]]; p != v {
					t.Fatal("original world vertex lost", id, p, v)
				}
			}
			for id, edge := range s.Edges() {
				proof := display.OriginalEdges[id]
				k := len(proof.Parameters) / 2
				parameter, _ := proof.Parameters[k].Float64()
				raw, err := edge.Curve.Eval(parameter)
				if err != nil || raw.Distance(display.Mesh.Vertices[proof.Vertices[k]]) > 1e-12 {
					t.Fatal("raw world model edge lost at its own station", id, parameter, err)
				}
			}
			t.Logf("region=%d Euler=%d triangles=%d volume=%.15g distance=%.12g turn=%.12g candidates=%d",
				display.Region, s.Topology().EulerCharacteristic, len(display.Mesh.Triangles), volume, distance,
				math.Sqrt(turnSquared), display.Collision.BroadPhasePairs)
		}
	}
}

func reopenPlacedParabolaTwoCircles(t *testing.T, cut *PlacedParabolaTwoCirclesGraphCut) *PlacedParabolaTwoCirclesGraphCut {
	t.Helper()
	ctx := context.Background()
	var original, encoded bytes.Buffer
	if err := cut.WriteJSON(ctx, &original); err != nil {
		t.Fatal(err)
	}
	replay, err := ReadPlacedParabolaTwoCirclesGraphCutJSON(ctx, bytes.NewReader(original.Bytes()), JSONLimits{}, ParabolaCircleGraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i, s := range cut.sources {
		if !geom.SameSplineSurface(s, replay.sources[i]) {
			t.Fatal("placed recipe changed original spline", i)
		}
	}
	if err := replay.WriteJSON(ctx, &encoded); err != nil || !bytes.Equal(original.Bytes(), encoded.Bytes()) {
		t.Fatal("placed native recipe is unstable", err)
	}
	return replay
}

func TestPlacedParabolaTwoCirclesRejectsPerturbedOwnerChart(t *testing.T) {
	ctx := context.Background()
	source := authoredPlacedParabolaTwoCirclesStock(t)
	d := source[1].Data()
	k := 4*d.CountV + 4
	d.Poles[k].Y = math.Nextafter(d.Poles[k].Y, math.Inf(1))
	moved, err := geom.NewBSplineSurfaceWithExactWeights(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
	if err != nil {
		t.Fatal(err)
	}
	if cut, err := CutPlacedParabolaTwoCirclesSplineGraphs(ctx, source[0], moved, source[2], 0, 1e-9, ParabolaCircleGraphOptions{}); cut != nil ||
		!errors.Is(err, geom.ErrInvalid) {
		t.Fatal("accepted a world owner with a nonshared chart", cut, err)
	}
}
