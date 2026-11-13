package step

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"os"
	"plainsolid/geom"
	"sync"
	"testing"
)

func rationalExtrusionFixture(t *testing.T, weighted bool) RationalLinearExtrusionSurface {
	t.Helper()
	var weights []float64
	if weighted {
		weights = []float64{1, .75, 1}
	}
	c, e := geom.NewBSpline(2, []geom.Vec3{{}, {X: -4, Y: 3, Z: 1}, {Z: 2}}, []float64{0, 0, 0, 1, 1, 1}, weights)
	if e != nil {
		t.Fatal(e)
	}
	s, e := NewRationalLinearExtrusionSurface("oblique", SplineCurve{Name: "profile", Geometry: c}, [3]*big.Rat{big.NewRat(3, 1), big.NewRat(4, 1), new(big.Rat)}, 0, 5)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestRationalLinearExtrusionExactIdentity(t *testing.T) {
	s := rationalExtrusionFixture(t, true)
	g, e := s.Surface()
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(g, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, u := range []*big.Rat{big.NewRat(1, 3), big.NewRat(2, 7)} {
		v := big.NewRat(7, 3)
		jet, e := p.EvaluateRational(context.Background(), u, v, geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		one := new(big.Rat).Sub(big.NewRat(1, 1), u)
		b0 := new(big.Rat).Mul(one, one)
		b1 := new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(u, one))
		b2 := new(big.Rat).Mul(u, u)
		den := new(big.Rat).Add(b0, new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(3, 4), b1), b2))
		x := new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(-3, 1), b1), den)
		y := new(big.Rat).Quo(new(big.Rat).Mul(big.NewRat(9, 4), b1), den)
		z := new(big.Rat).Quo(new(big.Rat).Add(new(big.Rat).Mul(big.NewRat(3, 4), b1), new(big.Rat).Mul(big.NewRat(2, 1), b2)), den)
		x.Add(x, new(big.Rat).Mul(v, big.NewRat(3, 5)))
		y.Add(y, new(big.Rat).Mul(v, big.NewRat(4, 5)))
		for i, q := range []*big.Rat{x, y, z} {
			if jet.Point[i].Cmp(q) != 0 {
				t.Fatal("exact nonbinary interior", i, jet.Point[i], q)
			}
		}
		for i, q := range []*big.Rat{big.NewRat(3, 5), big.NewRat(4, 5), new(big.Rat)} {
			if jet.Dv[i].Cmp(q) != 0 {
				t.Fatal("continuous derivative", i)
			}
		}
	}
	var b bytes.Buffer
	m := RationalLinearExtrusionModel{Surfaces: []RationalLinearExtrusionSurface{s}, LengthUnit: .001}
	if e = WriteRationalLinearExtrusions(&b, m); e != nil {
		t.Fatal(e)
	}
	out, e := ReadRationalLinearExtrusions(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil || out.LengthUnit != m.LengthUnit || len(out.Surfaces) != 1 {
		t.Fatal(out, e)
	}
	if _, e = ReadLinearExtrusions(bytes.NewReader(b.Bytes()), Limits{}); e == nil {
		t.Fatal("old strict codec broadened")
	}
	var again bytes.Buffer
	if e = WriteRationalLinearExtrusions(&again, out); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
		t.Fatal("canonical roundtrip", e)
	}
}
func TestRationalLinearExtrusionOwnershipAndLimits(t *testing.T) {
	s := rationalExtrusionFixture(t, false)
	ratios := [3]*big.Rat{big.NewRat(3, 5), big.NewRat(4, 5), new(big.Rat)}
	copy, e := NewRationalLinearExtrusionSurface("copy", s.Profile, ratios, 0, 5)
	if e != nil {
		t.Fatal(e)
	}
	ratios[0].SetInt64(99)
	returned := copy.DirectionRatios()
	returned[1].SetInt64(99)
	if copy.DirectionRatios()[0].Cmp(big.NewRat(3, 5)) != 0 || copy.DirectionRatios()[1].Cmp(big.NewRat(4, 5)) != 0 {
		t.Fatal("mutable alias")
	}
	for _, q := range [][3]*big.Rat{{big.NewRat(1, 1), big.NewRat(1, 1), new(big.Rat)}, {new(big.Rat), new(big.Rat), new(big.Rat)}, {big.NewRat(1, 3), big.NewRat(4, 9), new(big.Rat)}, {new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 300)), big.NewRat(1, 1), new(big.Rat)}, {new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 200)), new(big.Rat), new(big.Rat)}, {nil, big.NewRat(1, 1), new(big.Rat)}} {
		if _, e = NewRationalLinearExtrusionSurface("bad", s.Profile, q, 0, 5); e == nil {
			t.Fatal("unsupported direction", q)
		}
	}
	if _, e = NewRationalLinearExtrusionSurface("rounded", s.Profile, s.DirectionRatios(), 0, 1); !errors.Is(e, geom.ErrUnrepresentable) {
		t.Fatal("endpoint rounding", e)
	}
	bad := s
	bad.VEnd = 1
	var buf bytes.Buffer
	if e = WriteRationalLinearExtrusions(&buf, RationalLinearExtrusionModel{Surfaces: []RationalLinearExtrusionSurface{bad}, LengthUnit: .001}); !errors.Is(e, geom.ErrUnrepresentable) || buf.Len() != 0 {
		t.Fatal("atomic failure", e)
	}
	if e = WriteRationalLinearExtrusions(&buf, RationalLinearExtrusionModel{Surfaces: []RationalLinearExtrusionSurface{s}, LengthUnit: .001}); e != nil {
		t.Fatal(e)
	}
	for _, l := range []Limits{{MaxVertices: 5}, {MaxBytes: 16}, {MaxEntities: 4}} {
		if _, e = ReadRationalLinearExtrusions(bytes.NewReader(buf.Bytes()), l); e == nil {
			t.Fatal("budget")
		}
	}
	f, e := Parse(bytes.NewReader(buf.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for id, en := range f.Entities {
		if len(en.Records) == 1 && en.Records[0].Type == "DIRECTION" {
			en.Records[0].Args[1].Items[0] = Value{Kind: Number, Text: "3.00000000000000000001"}
			f.Entities[id] = en
		}
	}
	if _, e = ReadRationalLinearExtrusions(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
		t.Fatal("near direction silently snapped")
	}
}
func FuzzRationalLinearExtrusions(f *testing.F) {
	raw, _ := os.ReadFile("testdata/linear-extrusion.step")
	f.Add(raw)
	f.Add([]byte("bad"))
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadRationalLinearExtrusions(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 512, MaxValues: 8192, MaxVertices: 256, MaxFaces: 16})
	})
}

func TestRationalLinearExtrusionSourceTrims(t *testing.T) {
	s := rationalExtrusionFixture(t, false)
	var b bytes.Buffer
	if e := WriteRationalLinearExtrusions(&b, RationalLinearExtrusionModel{Surfaces: []RationalLinearExtrusionSurface{s}, LengthUnit: .001}); e != nil {
		t.Fatal(e)
	}
	for _, mutation := range []string{"sense", "partial", "magnitude"} {
		f, e := Parse(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		for id, en := range f.Entities {
			if len(en.Records) != 1 {
				continue
			}
			r := &en.Records[0]
			if r.Type == "RECTANGULAR_TRIMMED_SURFACE" {
				if mutation == "sense" {
					r.Args[6] = Value{Kind: Enumeration, Text: "F"}
				}
				if mutation == "partial" {
					r.Args[2] = Value{Kind: Number, Text: "0.25"}
				}
			}
			if r.Type == "VECTOR" && mutation == "magnitude" {
				r.Args[2] = Value{Kind: Number, Text: "2"}
			}
			f.Entities[id] = en
		}
		if _, e = ReadRationalLinearExtrusions(bytes.NewReader(serializeWrappedProofFile(f)), Limits{}); e == nil {
			t.Fatal("unsupported source", mutation)
		}
	}
}

func TestRationalLinearExtrusionMultiSpan(t *testing.T) {
	c, e := geom.NewBSpline(1, []geom.Vec3{{}, {X: -4, Y: 3, Z: 1}, {Z: 2}}, []float64{0, 0, .5, 1, 1}, []float64{1, .75, 1})
	if e != nil {
		t.Fatal(e)
	}
	s, e := NewRationalLinearExtrusionSurface("multi", SplineCurve{Geometry: c}, [3]*big.Rat{big.NewRat(3, 1), big.NewRat(4, 1), new(big.Rat)}, 0, 5)
	if e != nil {
		t.Fatal(e)
	}
	g, e := s.Surface()
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(g, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := p.EvaluateRational(context.Background(), big.NewRat(3, 4), big.NewRat(7, 3), geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i, want := range []*big.Rat{big.NewRat(-11, 35), big.NewRat(331, 105), big.NewRat(11, 7)} {
		if q.Point[i].Cmp(want) != 0 {
			t.Fatal(i, q.Point[i], want)
		}
	}
	var b bytes.Buffer
	if e = WriteRationalLinearExtrusions(&b, RationalLinearExtrusionModel{Surfaces: []RationalLinearExtrusionSurface{s}, LengthUnit: .001}); e != nil {
		t.Fatal(e)
	}
	out, e := ReadRationalLinearExtrusions(&b, Limits{})
	if e != nil || out.Surfaces[0].Profile.Geometry.Knots()[2] != .5 {
		t.Fatal("knot lineage", e)
	}
}

func TestRationalLinearExtrusionConcurrentCopies(t *testing.T) {
	s := rationalExtrusionFixture(t, true)
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for n := 0; n < 4; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			copy := s.DirectionRatios()
			copy[0].SetInt64(999)
			_, e := s.Surface()
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	if s.DirectionRatios()[0].Cmp(big.NewRat(3, 1)) != 0 {
		t.Fatal("mutable shared ratio")
	}
}
