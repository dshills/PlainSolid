package document

import (
	"bytes"
	"context"
	"os"
	"reflect"
	"strings"
	"testing"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestRoundedNestedMixedConicalAP242DocumentReceiptAndEdit(t *testing.T) {
	raw, err := os.ReadFile("../exchange/step/testdata/occt-nested-mixed-conical-curved-ap242-rounded.step")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	limits := STEPAssemblyOptions{MaxNodes: 9, MaxDepth: 3}
	if doc, err := ReadNestedMixedConicalAP242STEP(ctx, bytes.NewReader(raw), limits, geom.SurfacePatchOptions{}, 1); err == nil || doc != nil {
		t.Fatal("strict document reader repaired a rounded source", err)
	}
	options := step.NestedMixedConicalAP242RepairOptions{MaxDistance: 2.8e-12}
	if doc, err := ReadRoundedNestedMixedConicalAP242STEP(ctx, bytes.NewReader(raw), limits, options, 1); err == nil || doc.Document != nil || doc.Repair.OriginalSTEP() != "" {
		t.Fatal("short displacement published a partial document or receipt", err)
	}
	options.MaxDistance = 3e-12
	loaded, err := ReadRoundedNestedMixedConicalAP242STEP(ctx, bytes.NewReader(raw), limits, options, 2)
	if err != nil || loaded.Repair.OriginalSTEP() != string(raw) || loaded.Repair.CorrectedModelSTEP() == string(raw) {
		t.Fatal("original repair receipt was not retained", err)
	}
	s, revision := loaded.Document.Snapshot()
	if revision != 0 || s.LengthUnit != .001 || len(s.Trimmed) != 2 || len(s.Nodes) != 9 || len(s.Assets) != 0 {
		t.Fatal("repair did not retain original shared native parts and hierarchy", revision)
	}
	label := s.Labels["source:step-repair"]
	if label.Target != DocumentLabel || len(label.Attributes["step.source.original-sha256"].Text) != 64 ||
		label.Attributes["step.source.original-sha256"] == label.Attributes["step.source.corrected-sha256"] {
		t.Fatal("repair provenance from the original source is missing", label)
	}
	for _, key := range []string{"source:step-product-62", "source:step-product-230"} {
		l := s.Labels[key]
		if l.Parent != "source:step-repair" || len(l.Attributes["step.source.native-sha256"].Text) != 64 {
			t.Fatal("part repair provenance does not follow its original definition", key, l)
		}
	}
	var receipt bytes.Buffer
	if err := step.WriteRepairedNestedMixedConicalAP242JSON(ctx, &receipt, loaded.Repair); err != nil {
		t.Fatal(err)
	}
	replayed, err := ReadRepairedNestedMixedConicalAP242Receipt(ctx, bytes.NewReader(receipt.Bytes()), limits, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	var before, again bytes.Buffer
	if err := loaded.Document.Write(&before); err != nil {
		t.Fatal(err)
	}
	if err := replayed.Document.Write(&again); err != nil || !bytes.Equal(before.Bytes(), again.Bytes()) {
		t.Fatal("original source receipt produced a different initial document", err)
	}
	tx := loaded.Document.Begin()
	const secondGroup = "step-product-5/occ-443"
	group := tx.State.Nodes[secondGroup]
	group.Name = "Moved approved group"
	group.Transform[2][3] += 2.125
	tx.State.Nodes[secondGroup] = group
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	changed, _ := loaded.Document.Snapshot()
	var exported bytes.Buffer
	if err := changed.WriteNestedMixedConicalAP242STEP(ctx, &exported, strings.NewReader(loaded.Repair.CorrectedModelSTEP()), limits, geom.SurfacePatchOptions{}); err != nil {
		t.Fatal(err)
	}
	strict, err := ReadNestedMixedConicalAP242STEP(ctx, bytes.NewReader(exported.Bytes()), limits, geom.SurfacePatchOptions{}, 1)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := strict.Snapshot()
	if !reflect.DeepEqual(got.Nodes, changed.Nodes) || !reflect.DeepEqual(changed.Labels["source:step-repair"], label) || loaded.Repair.OriginalSTEP() != string(raw) {
		t.Fatal("corrected STEP edit changed original repair provenance or lost an occurrence")
	}
}
