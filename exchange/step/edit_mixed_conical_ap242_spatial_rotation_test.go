package step

import (
	"bytes"
	"context"
	"math/big"
	"os"
	"plainsolid/internal/slowtest"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestEditMixedConicalAP242DAGSpatialRationalRotation(t *testing.T) {
	slowtest.Skip(t)
	ctx, options := context.Background(), geom.SurfacePatchOptions{}
	l := MixedConicalAP242DAGLimits{MaxOccurrenceDepth: 3, MaxLeafInstances: 7}
	raw := mixedConicalAP242DAGFixture(t, "exact")
	base, err := ReadMixedConicalAP242DAG(ctx, bytes.NewReader(raw), l, options)
	if err != nil {
		t.Fatal(err)
	}
	rotation := &[3][3]*big.Rat{
		{big.NewRat(-15, 19), big.NewRat(-6, 19), big.NewRat(10, 19)},
		{big.NewRat(10, 19), big.NewRat(-15, 19), big.NewRat(6, 19)},
		{big.NewRat(6, 19), big.NewRat(10, 19), big.NewRat(15, 19)},
	}
	var out bytes.Buffer
	if err := WriteMixedConicalAP242DAGEdits(ctx, &out, bytes.NewReader(raw),
		[]MixedConicalAP242OccurrenceEdit{{NAUOID: 251, Rotation: rotation}}, l, options); err != nil {
		t.Fatal(err)
	}
	const zDirection = "#70=DIRECTION('',(10.,6.,15.));"
	const xDirection = "#71=DIRECTION('',(-15.,10.,6.));"
	if !bytes.Contains(out.Bytes(), []byte(zDirection)) || !bytes.Contains(out.Bytes(), []byte(xDirection)) {
		t.Fatal("spatial rotation lost its primitive source directions")
	}
	m, err := ReadMixedConicalAP242DAG(ctx, bytes.NewReader(out.Bytes()), l, options)
	if err != nil {
		t.Fatal(err)
	}
	ordinary, err := ReadAssembly(bytes.NewReader(out.Bytes()), l.Source)
	if err != nil {
		t.Fatal(err)
	}
	ordinaryPose, exactPose := conicalAssemblyOccurrencePose(t, ordinary, 251), conicalAssemblyOccurrencePose(t, m.Assembly, 251)
	if ordinaryPose == exactPose {
		t.Fatal("the ordinary normalization no longer exercises the spatial projection witness")
	}
	for i, row := range rotation {
		if [3]float64(exactPose[i][:3]) != [3]float64{conicalRatFloat(row[0]), conicalRatFloat(row[1]), conicalRatFloat(row[2])} {
			t.Fatal("the specialized display changed the exact spatial projection", exactPose)
		}
	}
	for i, got := range m.Instances {
		prior := base.Instances[i]
		want := prior.Matrix()
		switch i {
		case 1:
			want = conicalRotationMatrix(t, [3][4]string{
				{"78/95", "54/95", "1/19", "649/5"},
				{"-21/95", "22/95", "18/19", "82/5"},
				{"10/19", "-15/19", "6/19", "30"},
			})
		case 4:
			want = conicalRotationMatrix(t, [3][4]string{
				{"-21/95", "22/95", "18/19", "-598/5"},
				{"-162/475", "-441/475", "14/95", "-731/25"},
				{"434/475", "-138/475", "27/95", "1892/25"},
			})
		}
		if got.ProductID != prior.ProductID || !slices.Equal(got.Path, prior.Path) || !reflect.DeepEqual(got.Matrix(), want) {
			t.Fatal("wrong exact world orientation or owner", got.Path, got.Matrix())
		}
	}
	if !slices.Equal(m.Assembly.Source.Order, base.Assembly.Source.Order) {
		t.Fatal("rotation changed original source entity identities")
	}
	for _, id := range base.Assembly.Source.Order {
		if id != 70 && id != 71 && !reflect.DeepEqual(base.Assembly.Source.Entities[id], m.Assembly.Source.Entities[id]) {
			t.Fatalf("changed an unrelated original source record #%d", id)
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242_DAG"); oracle != "" {
		local := map[int64]trimmed.Solid{89: m.Parts[89].Rounded.Solid, 254: m.Parts[254].Chamfered.Solid}
		checkMixedConicalAP242DAGOracle(t, oracle, out.Bytes(), m.Instances, local, 254)
	}

	// Power-of-two scaling leaves both the exact ray and ordinary floating
	// normalization identical. Neither replacement is a primitive integer
	// triple and neither can use the new specialized source projection.
	for _, x := range []string{"#71=DIRECTION('',(-30.,20.,12.));", "#71=DIRECTION('',(-7.5,5.,3.));"} {
		mutant := bytes.Replace(out.Bytes(), []byte(xDirection), []byte(x), 1)
		a, err := ReadAssembly(bytes.NewReader(mutant), l.Source)
		if err != nil || conicalAssemblyOccurrencePose(t, a, 251) != ordinaryPose {
			t.Fatal("invalid ordinary control for the same exact spatial ray", err)
		}
		p, err := mappedRationalFrame(a.Source, 68)
		if err != nil || projectMixedConicalFrame(p) != exactPose {
			t.Fatal("scaled control changed the underlying exact rotation", err)
		}
		if _, err := ReadMixedConicalAP242DAG(ctx, bytes.NewReader(mutant), l, options); err == nil {
			t.Fatal("noncanonical spatial source bypassed its original projection guard", x)
		}
	}
}

func conicalAssemblyOccurrencePose(t *testing.T, a Assembly, id int64) geom.Affine {
	t.Helper()
	for _, use := range a.Occurrences {
		if use.ID == id {
			return use.Transform
		}
	}
	t.Fatal("missing original occurrence", id)
	return geom.Affine{}
}

func conicalRatFloat(x *big.Rat) float64 {
	f, _ := x.Float64()
	return f
}
