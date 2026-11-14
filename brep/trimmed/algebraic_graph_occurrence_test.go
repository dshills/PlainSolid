package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestAlgebraicGraphRationalOccurrence(t *testing.T) {
	ctx := context.Background()
	source := graphMaterialBooleanFixture(t, false, false)
	var pose [3][4]*big.Rat
	for i := range pose {
		for j := range pose[i] {
			pose[i][j] = new(big.Rat)
		}
	}
	pose[0][0] = big.NewRat(2, 1)
	pose[0][1] = big.NewRat(1, 3)
	pose[0][3] = big.NewRat(1, 7)
	pose[1][0] = big.NewRat(1, 5)
	pose[1][2] = big.NewRat(-3, 1)
	pose[1][3] = big.NewRat(2, 7)
	pose[2][1] = big.NewRat(1, 1)
	pose[2][3] = big.NewRat(-1, 11)
	axes := [2]int{0, 2}
	occurrence, e := NewAlgebraicGraphOccurrence(ctx, source, pose, axes, AlgebraicGraphMaterialOptions{})
	if e != nil {
		t.Fatal(e)
	}
	mapPoint := func(p [3]*big.Rat) (world [3]*big.Rat) {
		for i := range world {
			world[i] = new(big.Rat).Set(pose[i][3])
			for j := 0; j < 3; j++ {
				world[i].Add(world[i], new(big.Rat).Mul(pose[i][j], p[j]))
			}
		}
		return world
	}
	rr := big.NewRat(25, 4096)
	cases := []struct {
		point    [3]*big.Rat
		location parametric.Location
	}{
		{queryPoint(big.NewRat(5, 16), big.NewRat(1, 2), big.NewRat(25, 8192)), parametric.Inside},
		{queryPoint(big.NewRat(5, 16), big.NewRat(1, 2), new(big.Rat)), parametric.Boundary},
		{queryPoint(big.NewRat(5, 16), big.NewRat(1, 2), rr), parametric.Boundary},
		{queryPoint(big.NewRat(5, 16), big.NewRat(1, 2), big.NewRat(25, 2048)), parametric.Outside},
		{queryPoint(big.NewRat(25, 64), big.NewRat(1, 2), new(big.Rat)), parametric.Boundary},
		{queryPoint(new(big.Rat), new(big.Rat), new(big.Rat)), parametric.Outside},
	}
	var inside AlgebraicGraphMaterialPoint
	for i, c := range cases {
		got, e := occurrence.LocateRational(ctx, mapPoint(c.point), geom.ExactCurvePlaneEventOptions{})
		if e != nil || !got.Complete || got.Location != c.location {
			t.Fatal("analytic image location", i, got, e)
		}
		original, e := occurrence.query.LocateRational(ctx, c.point, geom.ExactCurvePlaneEventOptions{})
		if e != nil || got.Faces != original.Faces || !reflect.DeepEqual(got.BoundaryFaces, original.BoundaryFaces) || !reflect.DeepEqual(got.RimEdges, original.RimEdges) {
			t.Fatal("source ancestry", i, e)
		}
		if i == 0 {
			inside = got
			offset := big.NewRat(39, 112)
			lower := new(big.Rat).Sub(offset, new(big.Rat).Mul(big.NewRat(3, 1), rr))
			if got.Lower.Cmp(lower) != 0 || got.Upper.Cmp(offset) != 0 {
				t.Fatal("negative residual scale ordering", got.Lower, got.Upper)
			}
		}
	}
	world := mapPoint(cases[0].point)
	var native bytes.Buffer
	if e = WriteAlgebraicGraphOccurrenceJSON(&native, occurrence); e != nil {
		t.Fatal(e)
	}
	section := AlgebraicShellSectionOptions{Trace: nestedTubeSectionOptions().Trace}
	section.Trace.Coverage.OverlappingAffineXYFallback = true
	boolean := AlgebraicShellBooleanOptions{}
	boolean.Imprint.Exact.MaxOperations = 512000000
	back, e := ReadAlgebraicGraphOccurrenceJSON(ctx, bytes.NewReader(native.Bytes()), JSONLimits{}, section, boolean, AlgebraicGraphMaterialOptions{})
	if e != nil {
		t.Fatal("replayed exact occurrence", e)
	}
	got, e := back.LocateRational(ctx, world, geom.ExactCurvePlaneEventOptions{})
	if e != nil || !reflect.DeepEqual(got, inside) {
		t.Fatal("reload query", got, e)
	}
	var again bytes.Buffer
	if e = WriteAlgebraicGraphOccurrenceJSON(&again, back); e != nil || !bytes.Equal(again.Bytes(), native.Bytes()) {
		t.Fatal("canonical rational/source roundtrip", e)
	}
	pose[0][3].SetInt64(999)
	copy := occurrence.Pose()
	copy[0][0].SetInt64(999)
	got, e = occurrence.LocateRational(ctx, world, geom.ExactCurvePlaneEventOptions{})
	if e != nil || got.Location != parametric.Inside {
		t.Fatal("pose input/getter mutation", e)
	}
	for _, limit := range []int{1, inside.ExactOperations - 1} {
		bad, e := occurrence.LocateRational(ctx, world, geom.ExactCurvePlaneEventOptions{MaxOperations: limit})
		if e == nil || bad.Complete || bad.Lower != nil || bad.Faces != [2]int{-1, -1} {
			t.Fatal("atomic query budget", e)
		}
	}
	var envelope nativeAlgebraicGraphOccurrence
	if e = json.Unmarshal(native.Bytes(), &envelope); e != nil {
		t.Fatal(e)
	}
	for _, mutate := range []func(*nativeAlgebraicGraphOccurrence){func(f *nativeAlgebraicGraphOccurrence) { f.Pose[0][2] = "1" }, func(f *nativeAlgebraicGraphOccurrence) { f.Pose[0][0] = "0"; f.Pose[0][1] = "0" }, func(f *nativeAlgebraicGraphOccurrence) { f.Pose[0][0] = "2/2" }, func(f *nativeAlgebraicGraphOccurrence) { f.Source = json.RawMessage(`{"version":0}`) }} {
		f := envelope
		mutate(&f)
		raw, _ := json.Marshal(f)
		bad, e := ReadAlgebraicGraphOccurrenceJSON(ctx, bytes.NewReader(raw), JSONLimits{}, section, boolean, AlgebraicGraphMaterialOptions{})
		if e == nil || bad != nil {
			t.Fatal("unverified native envelope accepted")
		}
	}
	if bad, e := ReadAlgebraicGraphOccurrenceJSON(ctx, bytes.NewReader(native.Bytes()), JSONLimits{MaxBytes: 16}, section, boolean, AlgebraicGraphMaterialOptions{}); !errors.Is(e, geom.ErrConvergence) || bad != nil {
		t.Fatal("byte cap", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if bad, e := occurrence.LocateRational(cancelled, world, geom.ExactCurvePlaneEventOptions{}); !errors.Is(e, context.Canceled) || bad.Complete {
		t.Fatal("cancel", e)
	}
	if bad, e := NewAlgebraicGraphOccurrence(ctx, source, occurrence.Pose(), axes, AlgebraicGraphMaterialOptions{Exact: geom.ExactCurvePlaneEventOptions{MaxOperations: 1}}); !errors.Is(e, geom.ErrConvergence) || bad != nil {
		t.Fatal("atomic pose/preparation budget", e)
	}
	var zero AlgebraicGraphOccurrence
	if zero.Pose()[0][0] != nil || zero.ProjectionAxes() != [2]int{-1, -1} {
		t.Fatal("zero occurrence")
	}
	if occurrence.ProjectionAxes() != axes || occurrence.PreparationOperations() <= 0 {
		t.Fatal("occurrence metadata")
	}
	t.Logf("exact nonbinary pose retained; native recipe%d bytes, point%d exact operations", native.Len(), inside.ExactOperations)
}
