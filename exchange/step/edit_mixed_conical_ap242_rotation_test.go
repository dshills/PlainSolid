package step

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"os"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestEditMixedConicalAP242DAGRationalRotation(t *testing.T) {
	ctx, options := context.Background(), geom.SurfacePatchOptions{}
	raw := mixedConicalAP242DAGFixture(t, "exact")
	l := MixedConicalAP242DAGLimits{MaxOccurrenceDepth: 3, MaxLeafInstances: 7}
	base, err := ReadMixedConicalAP242DAG(ctx, bytes.NewReader(raw), l, options)
	if err != nil {
		t.Fatal(err)
	}
	rotation := conicalRotationAboutZ(5, 12, 13)
	var out bytes.Buffer
	if err := WriteMixedConicalAP242DAGEdits(ctx, &out, bytes.NewReader(raw),
		[]MixedConicalAP242OccurrenceEdit{{NAUOID: 251, Rotation: rotation}}, l, options); err != nil {
		t.Fatal(err)
	}
	m, err := ReadMixedConicalAP242DAG(ctx, bytes.NewReader(out.Bytes()), l, options)
	if err != nil {
		t.Fatal(err)
	}
	for i, got := range m.Instances {
		prior := base.Instances[i]
		if got.ProductID != prior.ProductID || !slices.Equal(got.Path, prior.Path) {
			t.Fatal("rotation replaced an original product or expanded path", got.Path)
		}
		want := prior.Matrix()
		switch i {
		case 1:
			want = conicalRotationMatrix(t, [3][4]string{
				{"-4/13", "48/65", "3/5", "649/5"},
				{"3/13", "-36/65", "4/5", "82/5"},
				{"12/13", "5/13", "0", "30"},
			})
		case 4:
			want = conicalRotationMatrix(t, [3][4]string{
				{"3/13", "-36/65", "4/5", "-598/5"},
				{"4/5", "-9/25", "-12/25", "-731/25"},
				{"36/65", "244/325", "9/25", "1892/25"},
			})
		}
		if !reflect.DeepEqual(got.Matrix(), want) {
			t.Fatalf("wrong rational world frame on %v: %v", got.Path, got.Matrix())
		}
	}
	if !slices.Equal(base.Assembly.Source.Order, m.Assembly.Source.Order) {
		t.Fatal("private frame rotation replaced an original entity ID")
	}
	for _, id := range base.Assembly.Source.Order {
		if id != 70 && id != 71 && !reflect.DeepEqual(base.Assembly.Source.Entities[id], m.Assembly.Source.Entities[id]) {
			t.Fatalf("rotation unexpectedly changed original entity #%d", id)
		}
	}
	if !bytes.Contains(out.Bytes(), []byte("#71=DIRECTION('',(5.,12.,0.));")) {
		t.Fatal("nonterminating rational rotation was not represented by exact STEP direction ratios")
	}
	local := map[int64]trimmed.Solid{89: m.Parts[89].Rounded.Solid, 254: m.Parts[254].Chamfered.Solid}
	for id, before := range map[int64]trimmed.Solid{89: base.Parts[89].Rounded.Solid, 254: base.Parts[254].Chamfered.Solid} {
		x, xErr := prismNativeBytes(before)
		y, yErr := prismNativeBytes(local[id])
		if xErr != nil || yErr != nil || !bytes.Equal(x, y) {
			t.Fatal("rotation changed local native material", id, xErr, yErr)
		}
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242_DAG"); oracle != "" {
		checkMixedConicalAP242DAGOracle(t, oracle, out.Bytes(), m.Instances, local, 254)
	}

	// A later ordinary translation keeps even a nonterminating rotation.
	var later bytes.Buffer
	origin := [3]*big.Rat{big.NewRat(-199, 8), big.NewRat(24, 1), big.NewRat(3, 1)}
	if err := WriteMixedConicalAP242DAGEdits(ctx, &later, bytes.NewReader(out.Bytes()),
		[]MixedConicalAP242OccurrenceEdit{{NAUOID: 251, Translation: &origin}}, l, options); err != nil {
		t.Fatal("ordinary translation lost its newly written rational axes", err)
	}
	translated, err := ReadMixedConicalAP242DAG(ctx, &later, l, options)
	if err != nil {
		t.Fatal(err)
	}
	for _, use := range translated.Occurrences {
		if use.NAUOID == 251 {
			got := use.Matrix()
			for i := range got {
				for j, q := range rotation[i] {
					if got[i][j].Cmp(q) != 0 {
						t.Fatal("later translation altered the exact orientation", got)
					}
				}
				if got[i][3].Cmp(origin[i]) != 0 {
					t.Fatal("later translation lost its exact origin", got)
				}
			}
		}
	}
}

func TestEditMixedConicalAP242DAGStyledTerminalRationalRotation(t *testing.T) {
	ctx, options := context.Background(), geom.SurfacePatchOptions{}
	l := MixedConicalAP242DAGLimits{}
	source := mixedDAGAppearanceFixture(t)
	before, err := ReadMixedConicalAP242DAGAppearance(ctx, bytes.NewReader(source), l, options)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := WriteMixedConicalAP242DAGAppearanceEdits(ctx, &out, bytes.NewReader(source),
		[]MixedConicalAP242OccurrenceEdit{{NAUOID: 251, Rotation: conicalRotationAboutZ(5, 12, 13)}}, nil, l, options); err != nil {
		t.Fatal(err)
	}
	after, err := ReadMixedConicalAP242DAGAppearance(ctx, bytes.NewReader(out.Bytes()), l, options)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(before.ProductStyles, after.ProductStyles) || !slices.Equal(before.FaceStyles, after.FaceStyles) ||
		before.PathOverride != after.PathOverride || after.PathOverride.Path != [3]int64{54, 85, 251} ||
		after.PathOverride.Context.PlacementID != 68 {
		t.Fatal("rotation moved an original style owner or occurrence path")
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242_APPEARANCE"); oracle != "" {
		checkMixedDAGAppearanceOracle(t, oracle, out.Bytes())
		local := map[int64]trimmed.Solid{89: after.DAG.Parts[89].Rounded.Solid, 254: after.DAG.Parts[254].Chamfered.Solid}
		checkMixedConicalAP242DAGOracle(t, oracle, out.Bytes(), after.DAG.Instances, local, 254)
	}
}

func TestEditMixedConicalAP242DAGRationalRotationReject(t *testing.T) {
	ctx, options := context.Background(), geom.SurfacePatchOptions{}
	l := MixedConicalAP242DAGLimits{}
	raw := mixedConicalAP242DAGFixture(t, "exact")
	reflection := conicalRotationAboutZ(5, 12, 13)
	reflection[2][2] = big.NewRat(-1, 1)
	// Quaternion (1,2,2,4) induces a proper rotation with terminating
	// unit axes. Where the ordinary projection differs, these decimal
	// axes do not supply the new primitive integer source proof.
	unsupported := &[3][3]*big.Rat{
		{big.NewRat(-3, 5), new(big.Rat), big.NewRat(4, 5)},
		{big.NewRat(16, 25), big.NewRat(-3, 5), big.NewRat(12, 25)},
		{big.NewRat(12, 25), big.NewRat(4, 5), big.NewRat(9, 25)},
	}
	for _, tc := range []struct {
		name  string
		edits []MixedConicalAP242OccurrenceEdit
		cause error
	}{
		{"reflection", []MixedConicalAP242OccurrenceEdit{{NAUOID: 251, Rotation: reflection}}, geom.ErrUnrepresentable},
		{"strict frame projection", []MixedConicalAP242OccurrenceEdit{{NAUOID: 251, Rotation: unsupported}}, geom.ErrUnrepresentable},
		{"conflicting shared relationship", []MixedConicalAP242OccurrenceEdit{
			{NAUOID: 251, Rotation: conicalRotationAboutZ(5, 12, 13)}, {NAUOID: 251, Rotation: conicalRotationAboutZ(3, 4, 5)},
		}, geom.ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := bytes.NewBufferString("untouched")
			if err := WriteMixedConicalAP242DAGEdits(ctx, out, bytes.NewReader(raw), tc.edits, l, options); !errors.Is(err, tc.cause) || out.String() != "untouched" {
				t.Fatal("invalid rotation request published STEP", err)
			}
		})
	}
}

func conicalRotationAboutZ(x, y, d int64) *[3][3]*big.Rat {
	return &[3][3]*big.Rat{
		{big.NewRat(x, d), big.NewRat(-y, d), new(big.Rat)},
		{big.NewRat(y, d), big.NewRat(x, d), new(big.Rat)},
		{new(big.Rat), new(big.Rat), big.NewRat(1, 1)},
	}
}

func conicalRotationMatrix(t *testing.T, tokens [3][4]string) (m [3][4]*big.Rat) {
	t.Helper()
	for i, row := range tokens {
		for j, token := range row {
			q, ok := new(big.Rat).SetString(token)
			if !ok {
				t.Fatal("invalid exact fixture coordinate", token)
			}
			m[i][j] = q
		}
	}
	return m
}
