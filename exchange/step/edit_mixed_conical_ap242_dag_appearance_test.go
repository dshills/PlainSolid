package step

import (
	"bytes"
	"context"
	"math/big"
	"slices"
	"testing"

	"plainsolid/geom"
)

func TestEditMixedConicalAP242DAGAppearanceSharedPlacementDatum(t *testing.T) {
	ctx, options := context.Background(), geom.SurfacePatchOptions{}
	l := MixedConicalAP242DAGLimits{}
	raw := mixedDAGAppearanceFixture(t)
	// Keep the two original leaf frames and let them share an exact origin.
	// Replacing the styled leaf origin must leave the other use untouched.
	raw = replaceMixedDAGAppearance(t, raw, [2]string{"#64 = AXIS2_PLACEMENT_3D('',#65,#66,#67);\n#65 = CARTESIAN_POINT('',(30.,2.,4.));", "#64 = AXIS2_PLACEMENT_3D('',#69,#66,#67);"})
	origin := [3]*big.Rat{big.NewRat(-199, 8), big.NewRat(24, 1), big.NewRat(3, 1)}
	var staged bytes.Buffer
	if err := WriteMixedConicalAP242DAGAppearanceEdits(ctx, &staged, bytes.NewReader(raw),
		[]MixedConicalAP242OccurrenceEdit{{NAUOID: 251, Translation: &origin}}, nil, l, options); err != nil {
		t.Fatal(err)
	}
	reopened, err := ReadMixedConicalAP242DAGAppearance(ctx, &staged, l, options)
	if err != nil {
		t.Fatal(err)
	}
	c := reopened.PathOverride
	source := reopened.DAG.Assembly.Source
	if c.Path != [3]int64{54, 85, 251} || c.Context.PlacementID <= 516 || c.Context.Color != (RGB{B: 1}) ||
		source.Entities[64].Records[0].Args[1].Ref != 69 ||
		source.Entities[482].Records[0].Args[1].Items[0].Ref != c.Context.PlacementID ||
		source.Entities[249].Records[0].Args[3].Ref != c.Context.PlacementID {
		t.Fatal("shared translation did not retain the original context and independent owner", c)
	}
	for _, use := range reopened.DAG.Occurrences {
		if use.NAUOID == 251 && use.Matrix()[0][3].Cmp(origin[0]) != 0 ||
			use.NAUOID == 245 && use.Matrix()[0][3].Cmp(big.NewRat(-30, 1)) != 0 {
			t.Fatal("changed the wrong exact occurrence placement", use)
		}
	}
	for i, id := range []int64{245, 251, 443} {
		if reopened.DAG.Instances[i].Path[2] != id || !slices.Equal(reopened.DAG.Instances[i].Path[:2], []int64{54, 85}) {
			t.Fatal("changed expanded path order", reopened.DAG.Instances)
		}
	}
}

func TestEditMixedConicalAP242DAGAppearanceReject(t *testing.T) {
	ctx, options := context.Background(), geom.SurfacePatchOptions{}
	l := MixedConicalAP242DAGLimits{}
	raw := mixedDAGAppearanceFixture(t)
	for _, paints := range [][]MixedConicalAP242SurfaceColorEdit{
		{{StyledItemID: 336, Color: RGB{B: 1}}}, // The original face ID is no authority to repaint.
		{{StyledItemID: 509, Color: RGB{G: 1.01}}},
		{{StyledItemID: 509, Color: RGB{B: 1}}, {StyledItemID: 509, Color: RGB{G: 1}}},
	} {
		var staged bytes.Buffer
		if err := WriteMixedConicalAP242DAGAppearanceEdits(ctx, &staged, bytes.NewReader(raw), nil, paints, l, options); err == nil || staged.Len() != 0 {
			t.Fatal("invalid styled item edit published STEP", paints, err)
		}
	}
}
