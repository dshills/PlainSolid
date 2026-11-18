package document

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/geom"
)

func TestNestedMixedConicalAP242Document(t *testing.T) {
	slowtest.Skip(t)
	raw, err := os.ReadFile("../exchange/step/testdata/occt-nested-mixed-conical-curved-ap242-exact.step")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	doc, err := ReadNestedMixedConicalAP242STEP(ctx, bytes.NewReader(raw), STEPAssemblyOptions{MaxNodes: 9, MaxDepth: 3}, geom.SurfacePatchOptions{}, 2)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := doc.Snapshot()
	if before.LengthUnit != .001 || len(before.Nodes) != 9 || len(before.Trimmed) != 2 || len(before.Assets) != 0 ||
		len(before.Trimmed["step-product-62"].Faces()) != 6 || len(before.Trimmed["step-product-227"].Faces()) != 8 {
		t.Fatal("document lost local solids, sharing or units")
	}
	const groupA, groupB = "step-product-5/occ-58", "step-product-5/occ-422"
	const roundedA, roundedB, chamferB = groupA + "/occ-218", groupB + "/occ-218", groupB + "/occ-416"
	if before.Nodes[roundedA].Asset != "step-product-62" || before.Nodes[roundedB].Asset != before.Nodes[roundedA].Asset ||
		before.Nodes[chamferB].Asset != "step-product-227" || before.Nodes[chamferB].Parent != groupB ||
		before.Nodes[groupB].Asset != "" || before.Nodes[groupB].Name == "" {
		t.Fatal("document lost the repeated original group and leaf paths")
	}
	label := before.Labels["source-node:"+chamferB]
	if label.Attributes["step.source.nauo"].Integer != 416 || label.Attributes["step.source.path"].Text != chamferB ||
		label.Attributes["step.source.world-pose"].Text != "3/5,-4/5,0,-100;12/25,9/25,-4/5,-222/5;16/25,12/25,3/5,54/5" {
		t.Fatal("exact source occurrence label changed", label)
	}
	faceLabels := 0
	for _, f := range before.Labels {
		if f.Target == FaceLabel {
			faceLabels++
			if f.Face == nil || f.Parent != "source:"+f.TargetID || f.Attributes["step.source.face"].Integer <= 0 {
				t.Fatal("original face ownership changed", f)
			}
		}
	}
	if faceLabels != 14 {
		t.Fatal("face labels should be owned by shared definitions", faceLabels)
	}
	worldPoint := func(s State, key string) geom.Vec3 {
		t.Helper()
		pose, err := s.WorldTransform(key)
		if err != nil {
			t.Fatal(err)
		}
		return pose.Apply(geom.Vec3{X: 1.5, Y: 2, Z: 4})
	}
	wantPoint := geom.Vec3{X: -1007. / 10, Y: -1154. / 25, Z: 378. / 25}
	if p := worldPoint(before, chamferB); math.Abs(p.X-wantPoint.X)+math.Abs(p.Y-wantPoint.Y)+math.Abs(p.Z-wantPoint.Z) > 5e-13 {
		t.Fatal("document display pose disagrees with exact AP242 composition", p)
	}
	tx := doc.Begin()
	group := tx.State.Nodes[groupB]
	group.Transform[1][3] += 17
	tx.State.Nodes[groupB] = group
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	changed, _ := doc.Snapshot()
	if got, old := worldPoint(changed, chamferB), worldPoint(before, chamferB); math.Abs(got.Y-old.Y-17) > 1e-13 || got.X != old.X || got.Z != old.Z {
		t.Fatal("moving one group did not move its descendant", got, old)
	}
	if worldPoint(changed, roundedA) != worldPoint(before, roundedA) || !reflect.DeepEqual(changed.Labels["source-node:"+chamferB], label) {
		t.Fatal("moving one group changed a sibling or the source provenance")
	}
	if err := doc.Undo(); err != nil {
		t.Fatal(err)
	}
	undone, _ := doc.Snapshot()
	if !reflect.DeepEqual(before.Nodes, undone.Nodes) {
		t.Fatal("undo did not restore the original placements")
	}
	if err := doc.Redo(); err != nil {
		t.Fatal(err)
	}

	var native, again bytes.Buffer
	if err := doc.Write(&native); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Read(bytes.NewReader(native.Bytes()), 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if err := reloaded.Write(&again); err != nil || !bytes.Equal(native.Bytes(), again.Bytes()) {
		t.Fatal("edited occurrence, provenance or local exact material changed in native replay", err)
	}
	for _, limits := range []STEPAssemblyOptions{{MaxDepth: 2}, {MaxNodes: 8}} {
		if got, err := ReadNestedMixedConicalAP242STEP(ctx, bytes.NewReader(raw), limits, geom.SurfacePatchOptions{}, 1); got != nil || !errors.Is(err, geom.ErrConvergence) {
			t.Fatal("short expansion limit returned a partial document", err)
		}
	}
}
