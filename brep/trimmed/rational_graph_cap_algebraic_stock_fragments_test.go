//go:build research

package trimmed

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	"plainsolid/geom"
)

func TestAlgebraicGraphStockSeveralRetainedFaces(t *testing.T) {
	ctx := context.Background()
	stock := makeGraphDisconnectedStripStock(t)
	// This parallels the existing rational strip fixture, but the two
	// intersections with the original floor are now irrational.
	const lo, hi = 11. / 256, 19. / 256
	planes := [2][3]geom.Vec3{
		{{Z: lo}, {Y: 1, Z: lo}, {X: 1, Z: lo}},
		{{Z: hi}, {X: 1, Z: hi}, {Y: 1, Z: hi - 1./16}},
	}
	opts := RationalGraphShellSectionOptions{ComposeSectionCurves: true}
	pair, e := stock.ClipAlgebraicGraphCapPair(ctx, planes, opts, RationalGraphShellRegionOptions{})
	if e != nil || len(pair.Seams) != 1 {
		t.Fatal("construct exact repeated stock cuts", len(pair.Seams), e)
	}
	irrational := 0
	for _, x := range pair.Caps[0].Section.Contacts {
		proof, e := x.Parameter.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if x.Source.Face == 4 && proof.Kind == geom.ExactRootKindIrrational {
			irrational++
		}
	}
	if irrational != 2 {
		t.Fatal("both floor junctions must retain their irrational roots", irrational)
	}
	out, e := pair.ExactIndexedBoundary(ctx, opts, geom.ExactCurvePlaneEventOptions{})
	if e != nil || out.BoundaryComponents != 1 || out.EulerCharacteristic != 2 {
		t.Fatal("closed hull with repeated original stock faces", out.EulerCharacteristic, e)
	}
	count := 0
	for _, f := range out.Faces {
		if f.StockFace == 4 {
			count++
		}
	}
	if count != 2 {
		t.Fatal("both disconnected retained floor faces must retain original chart ownership", count)
	}
	var data bytes.Buffer
	if e := WriteAlgebraicGraphStockJSON(&data, pair); e != nil {
		t.Fatal(e)
	}
	restored, topology, e := ReadAlgebraicGraphStockJSON(ctx, &data, JSONLimits{}, RationalGraphShellSectionOptions{},
		RationalGraphShellRegionOptions{}, geom.ExactCurvePlaneEventOptions{})
	if e != nil || !reflect.DeepEqual(topology, out) {
		t.Fatal("both original face fragments must survive native reload", e)
	}
	for i := range pair.Caps[0].Section.Contacts {
		a := pair.Caps[0].Section.Contacts[i]
		b := restored.Caps[0].Section.Contacts[i]
		order, e := a.Parameter.Compare(ctx, b.Parameter, geom.ExactCurvePlaneEventOptions{})
		if e != nil || !order.Proven || order.Order != 0 || a.Source != b.Source {
			t.Fatal("different root or face after reload", i, e)
		}
	}

}
