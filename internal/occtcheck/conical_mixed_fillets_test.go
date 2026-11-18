//go:build research

package occtcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
	"time"
)

// The oracle independently fillets a native OCCT cone before cutting cylinders.
func TestOCCTConicalMixedFillets(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_CONICAL_FILLET")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_CONICAL_FILLET to standalone oracle")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "reference.jsonl")
	if b, e := exec.CommandContext(ctx, program, path).CombinedOutput(); e != nil {
		t.Fatalf("%v: %s", e, b)
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	type row struct {
		record
		Solids, Euler int
		Parameters    []float64
	}
	refs := map[string]row{}
	dec := json.NewDecoder(f)
	for {
		var r row
		e = dec.Decode(&r)
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		if _, ok := refs[r.Name]; ok {
			t.Fatal("duplicate", r.Name)
		}
		refs[r.Name] = r
	}
	if len(refs) != 5 {
		t.Fatal(refs)
	}
	o := geom.SurfacePatchOptions{}
	w, e := geom.NewCanonicalWrappedCylinder(ctx, geom.Vec3{}, 8, 6, o)
	if e != nil {
		t.Fatal(e)
	}
	w, _, e = geom.DraftWrappedFrustum(ctx, w, 0, 8, o)
	if e != nil {
		t.Fatal(e)
	}
	source, _, e := trimmed.NewWrappedFrustum(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	stock, _, e := trimmed.FilletWrappedFrustumRims(ctx, source, 15./16, 15./8, o)
	if e != nil {
		t.Fatal(e)
	}
	cuts := []trimmed.FilletedFrustumBoreCut{{Center: geom.Vec2{X: -2}, Radius: .5}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 1}, {Center: geom.Vec2{X: 2}, Radius: .5, Depth: 4, FromUpper: true}}
	q, e := trimmed.PrepareFilletedFrustumMaterial(ctx, source, 15./16, 15./8, cuts, o)
	if e != nil {
		t.Fatal(e)
	}
	for name, s := range map[string]trimmed.Solid{"stock": stock, "mixed": q.Solid()} {
		m, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		if e != nil {
			t.Fatal(e)
		}
		r := refs[name]
		compare(t, r.record, m.Area, m.Volume, m.Centroid)
		wantEuler := 2
		if name == "mixed" {
			wantEuler = 0
		}
		if r.Solids != 1 || r.Shells != 1 || r.Euler != wantEuler {
			t.Fatal("oracle topology", r)
		}
	}
	if math.Abs(refs["stock"].Volume-refs["mixed"].Volume-11*math.Pi/4) > 1e-6 || math.Abs(refs["mixed"].Area-refs["stock"].Area-21*math.Pi/2) > 1e-6 {
		t.Fatal("independent cylinder removal formula")
	}
	// Reprove the persisted recipe before comparing its exact line ledger.
	var encoded bytes.Buffer
	if e := trimmed.WriteFilletedFrustumMaterialJSON(ctx, &encoded, q); e != nil {
		t.Fatal(e)
	}
	q, e = trimmed.ReadFilletedFrustumMaterialJSON(ctx, bytes.NewReader(encoded.Bytes()), trimmed.FilletedFrustumMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	rat := big.NewRat
	for _, tc := range []struct {
		name   string
		p, d   [3]*big.Rat
		lo, hi int64
	}{
		{"oblique", [3]*big.Rat{rat(837, 104), rat(0, 1), rat(15, 208)}, [3]*big.Rat{rat(1, 1), rat(1, 1), rat(1, 1)}, -1, 8},
		{"mixed-zones", [3]*big.Rat{rat(-4, 1), rat(0, 1), rat(1, 2)}, [3]*big.Rat{rat(1, 1), rat(0, 1), rat(1, 4)}, -3, 20},
		{"mixed-reverse", [3]*big.Rat{rat(-4, 1), rat(0, 1), rat(1, 2)}, [3]*big.Rat{rat(-1, 1), rat(0, 1), rat(-1, 4)}, -20, 3},
	} {
		hits, e := q.IntersectLineRational(ctx, tc.p, tc.d, rat(tc.lo, 1), rat(tc.hi, 1), geom.ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		want := refs[tc.name].Parameters
		if len(hits.Hits) != len(want) {
			t.Fatal(tc.name, len(hits.Hits), want)
		}
		for i, h := range hits.Hits {
			b, e := h.Parameter.Enclose(ctx, rat(1, 1000000000), geom.ExactCurvePlaneEventOptions{})
			if e != nil {
				t.Fatal(e)
			}
			lo, _ := b.Interval.Lower.Float64()
			hi, _ := b.Interval.Upper.Float64()
			if want[i] < lo-3e-7 || want[i] > hi+3e-7 {
				t.Fatal(tc.name, i, want[i], lo, hi)
			}
		}
	}
}
