package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestIndependentCircularFrustumHierarchyMass(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	var source bytes.Buffer
	if e := WriteIndependentFrustumHierarchy(ctx, &source, IndependentFrustumHierarchyModel(frustumHierarchyModel(t)), Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	m, e := ReadIndependentFrustumHierarchy(ctx, bytes.NewReader(source.Bytes()), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	var native bytes.Buffer
	if e = WriteIndependentFrustumHierarchyJSON(ctx, &native, m); e != nil {
		t.Fatal(e)
	}
	m, e = ReadIndependentFrustumHierarchyJSON(ctx, bytes.NewReader(native.Bytes()), IndependentMaterialJSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := MeasureIndependentCircularFrustumHierarchy(ctx, m, RationalHierarchyMassOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Products) != 5 || len(r.Occurrences) != 9 {
		t.Fatal("counts")
	}
	expected := []*big.Rat{big.NewRat(24, 1), big.NewRat(206, 3), big.NewRat(12, 1), new(big.Rat), new(big.Rat)}
	for i, c := range r.Sum.Volume().Coefficients() {
		if c.Cmp(expected[i]) != 0 {
			t.Fatal("sum volume", i, c)
		}
	}
	count := 0
	for _, p := range r.Products {
		if p.Family != "circular-frustum" {
			continue
		}
		count++
		if len(p.SourceFaceIDs) != 3 || p.SourceBodyID == 0 {
			t.Fatal("source roles")
		}
		checks := []struct {
			p    PiPolynomial
			want *big.Rat
		}{{p.Moments.Volume(), big.NewRat(35, 3)}, {p.Moments.FirstMoments()[2], big.NewRat(-5, 12)}, {p.Moments.SecondMoments()[0][0], big.NewRat(31, 4)}, {p.Moments.SecondMoments()[1][1], big.NewRat(31, 4)}, {p.Moments.SecondMoments()[2][2], big.NewRat(65, 3)}}
		for _, q := range checks {
			cs := q.p.Coefficients()
			for i, c := range cs {
				want := new(big.Rat)
				if i == 1 {
					want = q.want
				}
				if c.Cmp(want) != 0 {
					t.Fatal("local exact moment", i, c, want)
				}
			}
		}
	}
	if count != 1 {
		t.Fatal("local frustum")
	}
	if bin := os.Getenv("PLAINSOLID_OCCT_RATIONAL_HIERARCHY_MASS"); bin != "" {
		file := filepath.Join(t.TempDir(), "mass.step")
		if e = os.WriteFile(file, source.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		b, e := exec.Command(bin, "read", file).CombinedOutput()
		if e != nil {
			t.Fatal(e, string(b))
		}
		at := bytes.LastIndex(b, []byte("{"))
		var q struct {
			Volume                        float64
			Solids                        int
			Center                        [3]float64
			CentralInertia, OriginInertia [3][3]float64
		}
		if at < 0 || json.Unmarshal(b[at:], &q) != nil {
			t.Fatal(string(b))
		}
		equal := func(a, b float64) bool { return math.Abs(a-b) < 1e-8*math.Max(1, math.Abs(b)) }
		if q.Solids != 9 || !equal(massValue(r.Sum.Volume()), q.Volume) {
			t.Fatal("oracle volume", q)
		}
		center := r.Sum.Centroid()
		central := r.Sum.InertiaAboutCentroid()
		origin := r.Sum.InertiaAboutOrigin()
		for i := 0; i < 3; i++ {
			if !equal(massValue(center[i].Numerator)/massValue(center[i].Denominator), q.Center[i]) {
				t.Fatal("oracle centroid", i)
			}
			for j := 0; j < 3; j++ {
				if !equal(massValue(central[i][j].Numerator)/massValue(central[i][j].Denominator), q.CentralInertia[i][j]) || !equal(massValue(origin[i][j]), q.OriginInertia[i][j]) {
					t.Fatal("oracle inertia", i, j)
				}
			}
		}
	}
	v := r.Sum.Volume().Coefficients()
	v[1].SetInt64(999)
	if r.Sum.Volume().Coefficients()[1].Cmp(expected[1]) != 0 {
		t.Fatal("mutable moment")
	}
	for _, o := range []RationalHierarchyMassOptions{{MaxTriangles: 1}, {Source: Limits{MaxSolids: 4}}} {
		if q, e := MeasureIndependentCircularFrustumHierarchy(ctx, m, o); e == nil || len(q.Products) != 0 {
			t.Fatal("partial budget")
		}
	}
	cancel, c := context.WithCancel(ctx)
	c()
	if q, e := MeasureIndependentCircularFrustumHierarchy(cancel, m, RationalHierarchyMassOptions{}); e == nil || len(q.Products) != 0 {
		t.Fatal("cancel")
	}
	m.Instances[0].Path[0]++
	if q, e := MeasureIndependentCircularFrustumHierarchy(ctx, m, RationalHierarchyMassOptions{}); e == nil || len(q.Products) != 0 {
		t.Fatal("forged provenance")
	}
}
func TestIndependentCircularFrustumMassRejectsNoncircular(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	model := frustumHierarchyModel(t)
	raw, e := os.ReadFile("testdata/wrapped-frustum.step")
	if e != nil {
		t.Fatal(e)
	}
	f, e := Parse(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for i := range model.Products {
		if model.Products[i].ID == "frustum" {
			model.Products[i].Source = f
		}
	}
	var b bytes.Buffer
	if e = WriteIndependentFrustumHierarchy(ctx, &b, IndependentFrustumHierarchyModel(model), Limits{}, 1e-9); e != nil {
		t.Fatal(e)
	}
	m, e := ReadIndependentFrustumHierarchy(ctx, bytes.NewReader(b.Bytes()), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if r, e := MeasureIndependentCircularFrustumHierarchy(ctx, m, RationalHierarchyMassOptions{}); e == nil || len(r.Products) != 0 {
		t.Fatal("noncircular approximate promotion")
	}
	for _, p := range m.Products {
		if p.Frustum != nil {
			p.Frustum.Embedding.Circular = true
		}
	}
	if r, e := MeasureIndependentCircularFrustumHierarchy(ctx, m, RationalHierarchyMassOptions{}); e == nil || len(r.Products) != 0 {
		t.Fatal("forged circular certificate")
	}
}
