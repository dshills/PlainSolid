package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
)

func TestCertifiedParabolaTwoCirclesCutMeshes(t *testing.T) {
	source := parabolaTwoCirclesStock(t, 0, twoCirclePrimaryStock(), false)
	diskVolume := 5347170325.0 * math.Pi / 2251799813685248
	expected := [2][]float64{
		{90040463765.0 / 44220983279616, 39745125.0 * math.Pi / 17592186044416, 259794325.0 * math.Pi / 2251799813685248},
		{20635608245.0/44220983279616 + diskVolume},
	}
	checkCertifiedParabolaTwoCirclesCutMeshes(t, source, 0, expected, ParabolaTwoCirclesMeshOptions{}, true)
}

func TestCertifiedLowerParabolaTwoCirclesCutMeshes(t *testing.T) {
	source := parabolaTwoCirclesStock(t, 1, [2]twoCircleStockProbe{
		twoCircleProbe(4, 2, 1, 16), twoCircleProbe(24, 4, 1, 32),
	}, false)
	expected := [2][]float64{
		{53390532055.0/1061303598710784 + 71494362625.0*math.Pi/1729382256910270464},
		{14292355992775.0 / 1061303598710784, 512149225.0 * math.Pi / 13510798882111488, 5939261825.0 * math.Pi / 1729382256910270464},
	}
	// The lower cap requires slightly over one million broad-phase pairs.
	options := ParabolaTwoCirclesMeshOptions{Collision: collision.Options{MaxPairs: 1100000}}
	checkCertifiedParabolaTwoCirclesCutMeshes(t, source, 1, expected, options, false)
}

func checkCertifiedParabolaTwoCirclesCutMeshes(t *testing.T, source [3]*geom.BSplineSurface, freeAxis int,
	expected [2][]float64, options ParabolaTwoCirclesMeshOptions, reopenReverse bool) {
	t.Helper()
	ctx := context.Background()
	for order, want := range expected {
		cut, err := CutParabolaTwoCirclesSplineGraphs(ctx, source[0], source[1+order], source[2-order], freeAxis, 1e-9, ParabolaCircleGraphOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if reopenReverse && order == 1 {
			var encoded bytes.Buffer
			if err := cut.WriteJSON(ctx, &encoded); err != nil {
				t.Fatal(err)
			}
			cut, err = ReadParabolaTwoCirclesGraphCutJSON(ctx, &encoded, JSONLimits{}, ParabolaCircleGraphOptions{})
			if err != nil {
				t.Fatal(err)
			}
		}
		got, err := cut.TessellateCertified(ctx, options)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(want) {
			t.Fatal("lost independently positive source components")
		}
		for i, display := range got {
			s := cut.Components()[i]
			left, _ := display.SourceVolume[0].Float64()
			right, _ := display.SourceVolume[1].Float64()
			volume, _ := display.SignedDisplayVolume.Float64()
			distance, _ := display.DistanceUpper.Float64()
			if !(distance > 0 && distance <= .05) || math.Abs(left-want[i]) > want[i]*1e-14 ||
				math.Abs(right-want[i]) > want[i]*1e-14 || math.Abs(volume-want[i]) > want[i]*.02 ||
				display.SignedDisplayVolume.Sign() <= 0 || !display.Collision.Complete || len(display.Collision.Contacts) != 0 {
				t.Fatalf("region %d lost a source proof: volume %.16g vs %.16g..%.16g, expected %.16g, distance %g",
					display.Region, volume, left, right, want[i], distance)
			}
			euler := len(display.Mesh.Vertices) - len(display.Mesh.Triangles)/2
			if euler != s.Topology().EulerCharacteristic || display.Mesh.ValidateSolid() != nil {
				t.Fatalf("region %d changed the source topology to Euler %d", display.Region, euler)
			}
			if err := auditParabolaTwoCirclesMesh(ctx, s, display); err != nil {
				t.Fatal(err)
			}
			for original, model := range s.vertices {
				if display.Mesh.Vertices[display.OriginalVertices[original]] != model {
					t.Fatal("original model vertex changed", original)
				}
			}
			t.Logf("region=%d triangles=%d Euler=%d signed volume=%.16g reference=%.16g distance bound=%g contact candidates=%d",
				display.Region, len(display.Mesh.Triangles), euler, volume, want[i], distance, display.Collision.BroadPhasePairs)
		}
	}
}

func TestCertifiedParabolaTwoCirclesCutMeshBudgetIsAtomic(t *testing.T) {
	ctx := context.Background()
	source := parabolaTwoCirclesStock(t, 0, twoCirclePrimaryStock(), false)
	cut, err := CutParabolaTwoCirclesSplineGraphs(ctx, source[0], source[1], source[2], 0, 1e-9, ParabolaCircleGraphOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := cut.TessellateCertified(ctx, ParabolaTwoCirclesMeshOptions{MaxTriangles: 4})
	if got != nil || !errors.Is(err, parametric.ErrBudget) {
		t.Fatalf("insufficient budget yielded an incomplete display: %v, %v", got, err)
	}
}
