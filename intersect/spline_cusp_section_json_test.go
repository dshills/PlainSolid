package intersect

import (
	"context"
	"encoding/json"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestCuspSectionExactNonbinaryPersistence(t *testing.T) {
	ctx := context.Background()
	a, b := cuspSectionFixtureCenter(t, big.NewRat(4, 9))
	s, e := CertifySplinePairCuspSection(ctx, a, b, big.NewRat(1, 4), big.NewRat(4, 9), big.NewRat(1, 3), big.NewRat(1, 2), geom.SurfacePatchOptions{}, geom.GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	raw, e := MarshalCuspSection(ctx, s, CuspSectionPersistenceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	restored, e := UnmarshalCuspSection(ctx, raw, CuspSectionPersistenceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i, q := range []*big.Rat{big.NewRat(-1, 2), new(big.Rat), big.NewRat(1, 2)} {
		original, e := s.SampleRational(ctx, q, geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		got, e := restored.SampleRational(ctx, q, geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for j := range got.Point {
			if got.Point[j].Cmp(original.Point[j]) != 0 {
				t.Fatal("model changed")
			}
		}
		for owner := range got.Parameters {
			for j := range got.Parameters[owner] {
				if got.Parameters[owner][j].Cmp(original.Parameters[owner][j]) != 0 {
					t.Fatal("UV changed")
				}
			}
		}
		if i != 1 {
			want := big.NewRat(5, 72)
			if i == 2 {
				want = big.NewRat(59, 72)
			}
			if got.Parameters[0][1].Cmp(want) != 0 || !got.Terminal {
				t.Fatal(got)
			}
			if _, exact := want.Float64(); exact {
				t.Fatal("fixture is binary")
			}
		}
	}
	var recipe cuspSectionRecipe
	if e = json.Unmarshal(raw, &recipe); e != nil {
		t.Fatal(e)
	}
	for _, mutate := range []func(*cuspSectionRecipe){func(q *cuspSectionRecipe) { q.Version = 2 }, func(q *cuspSectionRecipe) { q.Tau = "1/4" }, func(q *cuspSectionRecipe) { q.Center[1] = "1/2" }, func(q *cuspSectionRecipe) { q.Owners[0] = q.Owners[1] }, func(q *cuspSectionRecipe) { q.K = "1e999999999" }} {
		q := recipe
		mutate(&q)
		bad, _ := json.Marshal(q)
		if got, e := UnmarshalCuspSection(ctx, bad, CuspSectionPersistenceOptions{}); e == nil || got != nil {
			t.Fatal("tampered recipe", string(bad))
		}
	}
	for _, o := range []CuspSectionPersistenceOptions{{MaxBytes: len(raw) - 1}, {MaxControlPoints: 4}, {Proof: geom.GraphPointContactOptions{MaxOperations: 1}}} {
		if got, e := UnmarshalCuspSection(ctx, raw, o); e == nil || got != nil {
			t.Fatal("read budget")
		}
		if got, e := MarshalCuspSection(ctx, s, o); e == nil || got != nil {
			t.Fatal("write budget")
		}
	}
	if got, e := UnmarshalCuspSection(ctx, append(raw, []byte(" {}")...), CuspSectionPersistenceOptions{}); e == nil || got != nil {
		t.Fatal("trailing")
	}
}
