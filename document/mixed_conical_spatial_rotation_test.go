package document

import (
	"bytes"
	"context"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestMixedConicalAP242DAGDocumentSpatialRotation(t *testing.T) {
	source, d := openMixedConicalDAGAppearanceDocument(t)
	ctx, limits := context.Background(), mixedConicalAppearanceLimits()
	base, _ := d.Snapshot()
	tx := d.Begin()
	rotation := [3][3]*big.Rat{
		{big.NewRat(-15, 19), big.NewRat(-6, 19), big.NewRat(10, 19)},
		{big.NewRat(10, 19), big.NewRat(-15, 19), big.NewRat(6, 19)},
		{big.NewRat(6, 19), big.NewRat(10, 19), big.NewRat(15, 19)},
	}
	if err := tx.State.SetMixedConicalAP242OccurrenceRotation(251, rotation); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var native, replayed, out bytes.Buffer
	if err := d.Write(&native); err != nil {
		t.Fatal(err)
	}
	r, err := Read(bytes.NewReader(native.Bytes()), 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Write(&replayed); err != nil || !bytes.Equal(native.Bytes(), replayed.Bytes()) {
		t.Fatal("native roundtrip changed the exact spatial rotation edit", err)
	}
	state, _ := r.Snapshot()
	for _, path := range []string{appearanceSelected, appearanceSibling} {
		label := "source-node:" + path
		if !reflect.DeepEqual(base.Labels[label], state.Labels[label]) {
			t.Fatal("native rotation changed an original source path", path)
		}
	}
	if err := state.WriteMixedConicalAP242DAGAppearanceSTEP(ctx, &out, bytes.NewReader(source), limits, geom.SurfacePatchOptions{}); err != nil {
		t.Fatal(err)
	}
	reopened, err := ReadMixedConicalAP242DAGAppearanceSTEP(ctx, &out, limits, geom.SurfacePatchOptions{}, 1)
	if err != nil {
		t.Fatal(err)
	}
	after, _ := reopened.Snapshot()
	if !reflect.DeepEqual(state.Nodes, after.Nodes) || len(after.Trimmed) != 2 {
		t.Fatal("strict reopen changed the edited shared uses")
	}
	for path, want := range map[string]string{
		appearanceSelected: "-15/19,-6/19,10/19,82;10/19,-15/19,6/19,48;6/19,10/19,15/19,9",
		appearanceSibling:  "-21/95,22/95,18/19,-598/5;-162/475,-441/475,14/95,-731/25;434/475,-138/475,27/95,1892/25",
	} {
		if text := after.Labels["source-node:"+path].Attributes["step.source.world-pose"].Text; text != want {
			t.Fatal("strict reopen lost the spatial world frame", path, text)
		}
	}
	checkConicalDocPaint(t, after, appearanceSelected, 0, step.RGB{B: 1})
	checkConicalDocPaint(t, after, appearanceSibling, 0, step.RGB{R: 1})
	checkConicalDocPaint(t, after, appearanceRoot+"/occ-477", 3, step.RGB{R: 1, G: 1})
}
