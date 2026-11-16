package step

import (
	"bytes"
	"context"
	"math/big"
	"os"
	"slices"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestEditMixedConicalAP242DAG(t *testing.T) {
	ctx, options := context.Background(), geom.SurfacePatchOptions{}
	raw := mixedConicalAP242DAGFixture(t, "exact")
	l := MixedConicalAP242DAGLimits{MaxOccurrenceDepth: 3, MaxLeafInstances: 7}
	base, err := ReadMixedConicalAP242DAG(ctx, bytes.NewReader(raw), l, options)
	if err != nil {
		t.Fatal(err)
	}
	var origin [3]*big.Rat
	shift := big.NewRat(9, 8)
	for _, u := range base.Occurrences {
		if u.NAUOID == 466 {
			m := u.Matrix()
			origin = [3]*big.Rat{new(big.Rat).Add(m[0][3], shift), m[1][3], m[2][3]}
		}
	}
	if origin[0] == nil {
		t.Fatal("the selected original outer relationship is missing")
	}
	name := "Both copies' shared annulus"
	var out bytes.Buffer
	err = WriteMixedConicalAP242DAGEdits(ctx, &out, bytes.NewReader(raw), []MixedConicalAP242OccurrenceEdit{
		{NAUOID: 466, Translation: &origin}, {NAUOID: 443, Name: &name},
	}, l, options)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ReadMixedConicalAP242DAG(ctx, bytes.NewReader(out.Bytes()), l, options)
	if err != nil || len(m.Instances) != 7 || len(m.Occurrences) != 8 {
		t.Fatal("edited DAG did not reopen with its original sharing", err)
	}
	for _, u := range m.Occurrences {
		if u.NAUOID == 443 && u.Name != name {
			t.Fatal("the shared original relationship name was lost", u.Name)
		}
	}
	for i, got := range m.Instances {
		before := base.Instances[i]
		if got.ProductID != before.ProductID || !slices.Equal(got.Path, before.Path) {
			t.Fatal("a source path was replaced", got.Path, before.Path)
		}
		want := before.Matrix()
		if got.Path[0] == 466 {
			want[0][3].Add(want[0][3], shift)
		}
		for row, values := range got.Matrix() {
			for col, q := range values {
				if q.Cmp(want[row][col]) != 0 {
					t.Fatal("exact moved or unchanged leaf pose differs", got.Path, got.Matrix())
				}
			}
		}
	}
	local := map[int64]trimmed.Solid{89: m.Parts[89].Rounded.Solid, 254: m.Parts[254].Chamfered.Solid}
	for id, old := range map[int64]trimmed.Solid{89: base.Parts[89].Rounded.Solid, 254: base.Parts[254].Chamfered.Solid} {
		a, aErr := prismNativeBytes(old)
		b, bErr := prismNativeBytes(local[id])
		if aErr != nil || bErr != nil || !bytes.Equal(a, b) {
			t.Fatal("an occurrence edit changed original local native material", id, aErr, bErr)
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242_DAG"); oracle != "" {
		checkMixedConicalAP242DAGOracle(t, oracle, out.Bytes(), m.Instances, local)
	}
}
