package step

import (
	"context"
	"errors"
	"math"
	"os"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestRationalPosedConvexArcThroughBoreNative(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(tiltedBoreSource + "through-bore.step")
	if err != nil {
		t.Fatal(err)
	}
	m := checkRationalPosedConvexArcBore(t, raw)
	ctx := context.Background()
	center := geom.Vec2{X: -1, Y: 1}
	// Evidence access must leave the certified model recipe immutable.
	m.Bore.Center[0].SetInt64(99)
	native, err := m.Native(ctx)
	if err != nil {
		t.Fatal(err)
	}
	faces := native.Faces()
	if len(faces) != 11 || len(native.Vertices()) != 18 || len(native.Edges()) != 27 {
		t.Fatal("native material lost exterior segments or through-hole quarter patches")
	}
	for i, face := range faces[:2] {
		if len(face.Outer) != 5 || len(face.Holes) != 1 || len(face.Holes[0]) != 4 || face.Reversed != (i == 0) {
			t.Fatal("native cap lost its original material orientation or interior loop", i)
		}
		location, proof, err := trimmed.LocateInFaceTrim(ctx, face, center, trimvalidate.Options{})
		if err != nil || proof.Status != trimvalidate.Certified || location != trimvalidate.Outside {
			t.Fatal("native cap covers the bore center", i, location, err)
		}
	}
	for i, face := range faces[7:] {
		if !face.Reversed || len(face.Holes) != 0 || len(face.Outer) != 4 {
			t.Fatal("native cylindrical bore patch has incorrect material sense", i)
		}
	}
	properties, err := native.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-9})
	if err != nil {
		t.Fatal(err)
	}
	centerWorld, err := m.Pose().TransformPoint(ctx, occtLoopVec(properties.Centroid), false, 512)
	if err != nil {
		t.Fatal(err)
	}
	want := [5]float64{28.096231102869041, 73.08423752236105, 2.7684492062631412, 1.028061804597145, .47540105831581114}
	got := [5]float64{properties.Volume, properties.Area}
	for i, q := range centerWorld {
		got[i+2], _ = q.Float64()
	}
	for i, q := range got {
		if !finiteNumber(q) || math.Abs(q-want[i]) > 1e-10 {
			t.Fatal("native through-hole world measure differs from the independently transferred OCCT solid", i, q, want[i])
		}
	}
	d := m.outer.LocalDescriptor()
	bad := trimmed.ExtrusionCircle{Center: center, Radius: 3. / 2}
	if s, err := trimmed.ExtrudeExactCircularLoopWithThroughHole(ctx, d.Segments, bad, d.Height, trimmed.ExactCircularLoopOptions{}); err == nil || len(s.Faces()) != 0 {
		t.Fatal("overlapping native hole produced a solid", err)
	}
	bad = trimmed.ExtrusionCircle{Center: geom.Vec2{X: math.Ldexp(1, 55)}, Radius: .25}
	if s, err := trimmed.ExtrudeExactCircularLoopWithThroughHole(ctx, d.Segments, bad, d.Height, trimmed.ExactCircularLoopOptions{}); !errors.Is(err, geom.ErrUnrepresentable) || len(s.Faces()) != 0 {
		t.Fatal("inexact native quarter endpoints produced a solid", err)
	}
}
