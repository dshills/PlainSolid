package step

import (
	"bytes"
	"context"
	"math/big"
	"os"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestEditNestedMixedConicalAP242STEP(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := nestedMixedConicalAP242Fixture(t, "exact")
	name := "Plant A's annulus"
	translation := [3]*big.Rat{big.NewRat(-799, 8), big.NewRat(-161, 8), big.NewRat(123, 4)}
	edits := []NestedMixedConicalAP242OccurrenceEdit{{NAUOID: 416, Name: &name}, {NAUOID: 422, Translation: &translation}}
	var out bytes.Buffer
	if err := WriteNestedMixedConicalAP242Edits(ctx, &out, bytes.NewReader(raw), edits, Limits{}, geom.SurfacePatchOptions{}); err != nil {
		t.Fatal(err)
	}
	got, err := ReadNestedMixedConicalAP242Assembly(ctx, bytes.NewReader(out.Bytes()), Limits{}, geom.SurfacePatchOptions{})
	if err != nil || len(got.Instances) != 6 {
		t.Fatal("edited STEP did not reopen strictly", err)
	}
	for _, o := range got.Occurrences {
		switch o.NAUOID {
		case 416:
			if o.Name != name {
				t.Fatal("shared local relationship name changed in transit", o.Name)
			}
		case 422:
			mat := o.Matrix()
			for i := range translation {
				if mat[i][3].Cmp(translation[i]) != 0 {
					t.Fatal("exact new outer group origin changed", mat)
				}
			}
		}
	}
	// The translated outer group retains its non-cardinal rotation. For
	// this original child the authored old world point was independent of
	// the other copy of the same group.
	leaf := got.Instances[5]
	pt, err := leaf.ApplyExact(geom.Vec3{X: 1.5, Y: 2, Z: 4})
	if err != nil || leaf.Path != [2]int64{422, 416} || pt[0].RatString() != "-4023/40" ||
		pt[1].RatString() != "-9257/200" || pt[2].RatString() != "1587/100" {
		t.Fatal("edited original path lost exact pose", leaf.Path, pt, err)
	}
	if oracle := os.Getenv("PLAINSOLID_OCCT_MIXED_CONICAL_AP242"); oracle != "" {
		properties := map[int64]parametric.Properties{}
		for id, part := range got.Parts {
			var body trimmed.Solid
			if part.Rounded != nil {
				body = part.Rounded.Solid
			} else {
				body = part.Chamfered.Solid
			}
			p, err := body.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
			if err != nil {
				t.Fatal(err)
			}
			properties[id] = p
		}
		expected := make([]nestedMixedOccurrenceMeasure, 0, 6)
		for i, leaf := range got.Instances {
			p := properties[leaf.ProductID]
			m := nestedMixedOccurrenceMeasure{Path: [2]int{i/3 + 1, i%3 + 1}}
			m.Volume, m.Area, m.SharedWithFirst = p.Volume, p.Area, i%3 < 2
			m.Topology = [4]int{1, 6, 10, 6}
			if i%3 == 2 {
				m.Topology = [4]int{1, 8, 14, 8}
			}
			for j, row := range leaf.Matrix() {
				for k, q := range row {
					m.Pose[j*4+k], _ = q.Float64()
				}
			}
			centroid, err := leaf.ApplyExact(p.Centroid)
			if err != nil {
				t.Fatal(err)
			}
			for j, q := range centroid {
				m.Centroid[j], _ = q.Float64()
			}
			expected = append(expected, m)
		}
		checkNestedMixedConicalAP242Live(t, oracle, out.Bytes(), expected)
	}
}

func TestEditNestedMixedConicalAP242STEPReject(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := nestedMixedConicalAP242Fixture(t, "exact")
	name := "new"
	third := [3]*big.Rat{big.NewRat(1, 3), big.NewRat(2, 1), big.NewRat(3, 1)}
	for _, edits := range [][]NestedMixedConicalAP242OccurrenceEdit{
		{{NAUOID: 62, Name: &name}},
		{{NAUOID: 416, Name: &name}, {NAUOID: 416, Name: &name}},
		{{NAUOID: 422, Translation: &third}},
	} {
		var out bytes.Buffer
		if err := WriteNestedMixedConicalAP242Edits(ctx, &out, bytes.NewReader(raw), edits, Limits{}, geom.SurfacePatchOptions{}); err == nil || out.Len() != 0 {
			t.Fatal("invalid relationship edit published output", err, out.Len())
		}
	}
}
