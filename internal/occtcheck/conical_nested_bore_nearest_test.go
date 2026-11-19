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
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
	"time"
)

// The oracle independently fillets an OCCT cone and cuts its off-center
// through-cylinder; no PlainSolid mesh or native body is supplied to OCCT.
func TestOCCTConicalNestedBoreNearest(t *testing.T) {
	program := os.Getenv("PLAINSOLID_OCCT_CONICAL_FILLET")
	if program == "" {
		t.Skip("set PLAINSOLID_OCCT_CONICAL_FILLET to rebuilt pinned oracle")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	path := filepath.Join(t.TempDir(), "pair.jsonl")
	if b, e := exec.CommandContext(ctx, program, path, "--nested-bore").CombinedOutput(); e != nil {
		t.Fatalf("%v: %s", e, b)
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	type row struct {
		Name                             string
		SquaredDistance, Volume, Area    float64
		Point                            [3]float64
		Solutions, Solids, Shells, Euler int
	}
	refs := map[string]row{}
	dec := json.NewDecoder(f)
	for {
		var v row
		e := dec.Decode(&v)
		if e == io.EOF {
			break
		}
		if e != nil {
			t.Fatal(e)
		}
		if _, ok := refs[v.Name]; ok {
			t.Fatal("duplicate reference")
		}
		refs[v.Name] = v
	}
	if len(refs) != 5 {
		t.Fatal("wrong oracle version", len(refs))
	}
	stock, body := refs["stock"], refs["bore-cone"]
	if body.Solids != 1 || body.Shells != 1 || body.Euler != 0 {
		t.Fatal("independent pierced topology", body)
	}
	if math.Abs(stock.Volume-body.Volume-6*math.Pi) > 3e-6 || math.Abs(body.Area-stock.Area-10*math.Pi) > 3e-6 {
		t.Fatal("independent through-bore measures", stock, body)
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
	s, _, e := trimmed.NewWrappedFrustum(ctx, w, 1e-9, o)
	if e != nil {
		t.Fatal(e)
	}
	q, e := trimmed.PrepareFilletedFrustumMaterial(ctx, s, 15./16, 15./8, []trimmed.FilletedFrustumBoreCut{{Center: geom.Vec2{X: 6}, Radius: 1}}, o)
	if e != nil {
		t.Fatal(e)
	}
	var encoded bytes.Buffer
	if e = trimmed.WriteFilletedFrustumMaterialJSON(ctx, &encoded, q); e != nil {
		t.Fatal(e)
	}
	q, e = trimmed.ReadFilletedFrustumMaterialJSON(ctx, &encoded, trimmed.FilletedFrustumMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name string
		p    [3]float64
		sign int
	}{{"pair-torus", [3]float64{77. / 10, 1. / 100, 69. / 100}, -1}, {"pair-bore", [3]float64{77. / 10, 1. / 100, 71. / 100}, 1}, {"pair-tie", [3]float64{991. / 128, 0, 3. / 4}, 0}} {
		reference, ok := refs[tc.name]
		if !ok || reference.Solutions < 1 {
			t.Fatal("missing nearest reference", tc.name)
		}
		p := [3]*big.Rat{}
		for i := range p {
			p[i] = new(big.Rat).SetFloat64(tc.p[i])
		}
		got, e := q.ClassifyNestedTorusBoreFacesRational(ctx, p, trimmed.FilletedFrustumNestedBoreOptions{})
		if tc.sign == 0 {
			if e != nil || got.Complete || len(got.Tied) != 0 || got.Nested != nil || got.Bore != nil {
				t.Fatal("nested equality must remain unresolved", e)
			}
			exact, e := q.ClassifyTorusBoreFacesRational(ctx, p, trimmed.FilletedFrustumTorusBoreOptions{})
			if e != nil || !exact.Complete || len(exact.Tied) != 2 || exact.Comparison != 0 || math.Abs(reference.SquaredDistance-9025./16384) > 3e-7 {
				t.Fatal("independent exact tie reference", e)
			}
			continue
		}
		if e != nil || !got.Complete || got.Comparison != tc.sign {
			t.Fatal(tc.name, got, e)
		}
		owners := 1
		if tc.sign == 0 {
			owners = 2
		}
		if len(got.Tied) != owners {
			t.Fatal("proved owner count")
		}
		lo, _ := got.Lower.Float64()
		hi, _ := got.Upper.Float64()
		rho := math.Hypot(tc.p[0], tc.p[1])
		meridian := math.Hypot(rho-123./16, tc.p[2]-15./16)
		torus := math.Pow(meridian-15./16, 2)
		bore := math.Pow(math.Hypot(tc.p[0]-6, tc.p[1])-1, 2)
		expected := math.Min(torus, bore)
		tol := 3e-7 * math.Max(1, expected)
		if reference.SquaredDistance < lo-tol || reference.SquaredDistance > hi+tol || math.Abs(reference.SquaredDistance-expected) > tol {
			t.Fatal("independent distance", tc.name, reference.SquaredDistance, lo, hi, expected)
		}
		d := 0.
		for i := range tc.p {
			d += math.Pow(reference.Point[i]-tc.p[i], 2)
		}
		if math.Abs(d-reference.SquaredDistance) > tol {
			t.Fatal("reported OCCT witness")
		}
		torusFoot := func() [3]float64 {
			rp := 123./16 + (15./16)*(rho-123./16)/meridian
			return [3]float64{tc.p[0] * rp / rho, tc.p[1] * rp / rho, 15./16 + (15./16)*(tc.p[2]-15./16)/meridian}
		}

		boreFoot := func() [3]float64 {
			rr := math.Hypot(tc.p[0]-6, tc.p[1])
			return [3]float64{6 + (tc.p[0]-6)/rr, tc.p[1] / rr, tc.p[2]}
		}
		near := func(v [3]float64) bool {
			d := 0.
			for i := range v {
				d += math.Pow(v[i]-reference.Point[i], 2)
			}
			return d < 1e-12
		}
		if tc.sign < 0 && !near(torusFoot()) || tc.sign > 0 && !near(boreFoot()) || tc.sign == 0 && !near(torusFoot()) && !near(boreFoot()) {
			t.Fatal("independent retained foot", reference)
		}
		// OCCT's finite solution count is deliberately not an exhaustive tie claim.
	}
}
