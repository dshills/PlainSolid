package step

import (
	"bytes"
	"context"
	"os"
	"testing"
)

func TestIndependentRoundUnitsBoundedExponent(t *testing.T) {
	t.Parallel()
	source, e := os.ReadFile("testdata/independent-round-units.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, text := range []string{"1.E-999999999", "0.E-999999999", "1.E+999999999", "1.E-9999999999999999999999999999999999"} {
		f, e := Parse(bytes.NewReader(source), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		changed := false
		for id, en := range f.Entities {
			if len(en.Records) == 1 && en.Records[0].Type == "UNCERTAINTY_MEASURE_WITH_UNIT" {
				en.Records[0].Args[0].Items[0].Text = text
				f.Entities[id] = en
				changed = true
				break
			}
		}
		if !changed {
			t.Fatal("fixture uncertainty")
		}
		raw, e := mappedPlanarSourceBytes(f)
		if e != nil {
			t.Fatal(e)
		}
		if m, e := ReadNormalizedIndependentRoundMaterials(context.Background(), bytes.NewReader(raw), RoundMillimetre, Limits{}, 1e-9); e == nil || len(m.Members) != 0 {
			t.Fatal(text)
		}
	}
}
