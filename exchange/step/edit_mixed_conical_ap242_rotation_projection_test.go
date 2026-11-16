package step

import (
	"bytes"
	"context"
	"math/big"
	"os"
	"reflect"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestEditMixedConicalAP242DAGRationalRotationCardinalProjection(t *testing.T) {
	ctx, options := context.Background(), geom.SurfacePatchOptions{}
	l := MixedConicalAP242DAGLimits{MaxOccurrenceDepth: 3, MaxLeafInstances: 7}
	raw := mixedConicalAP242DAGFixture(t, "exact")
	base, err := ReadMixedConicalAP242DAG(ctx, bytes.NewReader(raw), l, options)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := WriteMixedConicalAP242DAGEdits(ctx, &out, bytes.NewReader(raw),
		[]MixedConicalAP242OccurrenceEdit{{NAUOID: 251, Rotation: conicalRotationAboutZ(20, 21, 29)}}, l, options); err != nil {
		t.Fatal(err)
	}
	const integerDirection = "#71=DIRECTION('',(20.,21.,0.));"
	if bytes.Count(out.Bytes(), []byte(integerDirection)) != 1 {
		t.Fatal("the proper rational rotation lost its canonical direction ratios")
	}
	ordinary, err := ReadAssembly(bytes.NewReader(out.Bytes()), l.Source)
	if err != nil {
		t.Fatal(err)
	}
	for _, use := range ordinary.Occurrences {
		if use.ID == 251 && use.Transform[0][0] != 0.68965517241379304 {
			t.Fatal("the independent ordinary projection witness changed", use.Transform)
		}
	}
	// An inexact right angle remains invalid even when the ordinary frame
	//'s perpendicular projection would remove the tiny axial component.
	bad := bytes.Replace(out.Bytes(), []byte(integerDirection), []byte("#71=DIRECTION('',(20.,21.,1.E-18));"), 1)
	if _, err := ReadAssembly(bytes.NewReader(bad), l.Source); err != nil {
		t.Fatal("ordinary frame rejected the nonorthogonal source control", err)
	}
	if _, err := ReadMixedConicalAP242DAG(ctx, bytes.NewReader(bad), l, options); err == nil {
		t.Fatal("strict reader admitted nonorthogonal source direction ratios")
	}
	m, err := ReadMixedConicalAP242DAG(ctx, bytes.NewReader(out.Bytes()), l, options)
	if err != nil {
		t.Fatal(err)
	}
	for _, use := range m.Assembly.Occurrences {
		if use.ID == 251 && use.Transform[0][0] != 20./29 {
			t.Fatal("specialized assembly did not project its proved exact coefficient", use.Transform)
		}
	}
	for i, instance := range m.Instances {
		prior := base.Instances[i]
		want := prior.Matrix()
		switch i {
		case 1:
			want = conicalRotationMatrix(t, [3][4]string{
				{"-16/29", "84/145", "3/5", "649/5"},
				{"12/29", "-63/145", "4/5", "82/5"},
				{"21/29", "20/29", "0", "30"},
			})
		case 4:
			want = conicalRotationMatrix(t, [3][4]string{
				{"12/29", "-63/145", "4/5", "-598/5"},
				{"127/145", "-36/725", "-12/25", "-731/25"},
				{"36/145", "652/725", "9/25", "1892/25"},
			})
		}
		if instance.ProductID != prior.ProductID || !reflect.DeepEqual(instance.Path, prior.Path) ||
			!reflect.DeepEqual(instance.Matrix(), want) {
			t.Fatalf("exact source world or owner changed on %v: %v", instance.Path, instance.Matrix())
		}
	}
	for _, id := range base.Assembly.Source.Order {
		if id != 70 && id != 71 && !reflect.DeepEqual(base.Assembly.Source.Entities[id], m.Assembly.Source.Entities[id]) {
			t.Fatalf("changed an unrelated original record #%d", id)
		}
	}
	if len(base.Assembly.Source.Order) != len(m.Assembly.Source.Order) {
		t.Fatal("changed the number of original STEP owners")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242_DAG"); oracle != "" {
		local := map[int64]trimmed.Solid{89: m.Parts[89].Rounded.Solid, 254: m.Parts[254].Chamfered.Solid}
		checkMixedConicalAP242DAGOracle(t, oracle, out.Bytes(), m.Instances, local, 254)
	}

	// A later translation keeps the same canonical ratio encoding.
	origin := [3]*big.Rat{big.NewRat(-199, 8), big.NewRat(24, 1), big.NewRat(3, 1)}
	var translated bytes.Buffer
	if err := WriteMixedConicalAP242DAGEdits(ctx, &translated, bytes.NewReader(out.Bytes()),
		[]MixedConicalAP242OccurrenceEdit{{NAUOID: 251, Translation: &origin}}, l, options); err != nil {
		t.Fatal("later translation lost its exact scaled direction", err)
	}
	if !bytes.Contains(translated.Bytes(), []byte(integerDirection)) {
		t.Fatal("later translation rewrote the original exact direction")
	}
}
