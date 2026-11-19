package step

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"os"
	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"strings"
	"testing"
)

func pythagoreanPrismFixture(t testing.TB) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/occt-pythagorean-elliptical-prism.step")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestPythagoreanEllipticalPrism(t *testing.T) {
	t.Parallel()
	raw := pythagoreanPrismFixture(t)
	m, e := ReadPythagoreanEllipticalPrism(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	_, x, y, z := m.RationalFrame()
	if x[0].Cmp(big.NewRat(3, 5)) != 0 || x[1].Cmp(big.NewRat(4, 5)) != 0 || y[0].Cmp(big.NewRat(-4, 5)) != 0 || z[2].Cmp(big.NewRat(1, 1)) != 0 {
		t.Fatal(x, y, z)
	}
	x[0].SetInt64(999)
	_, xx, _, _ := m.RationalFrame()
	if xx[0].Cmp(big.NewRat(3, 5)) != 0 {
		t.Fatal("mutable basis")
	}
	if _, e = ReadModelOnlyEllipticalPrism(context.Background(), bytes.NewReader(raw), Limits{}, 1e-9); e == nil {
		t.Fatal("cardinal gate widened")
	}
	var b bytes.Buffer
	if e = trimmed.WriteJSON(&b, m.Solid); e != nil {
		t.Fatal(e)
	}
	loaded, e := trimmed.ReadJSON(&b, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := loaded.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil || math.Abs(p.Volume-150*math.Pi) > 1e-6 || p.Centroid.Distance(geom.Vec3{X: 1, Y: 2, Z: 4.5}) > 1e-7 {
		t.Fatal(p, e)
	}
	proof, e := loaded.ValidateEdgeAgreement(context.Background(), edgecheck.Options{})
	if e != nil || !proof.Complete {
		t.Fatal(proof, e)
	}
	for _, u := range proof.Uses {
		if u.Report.SquaredUpperBound == nil || u.Report.SquaredUpperBound.Sign() != 0 {
			t.Fatal(u)
		}
	}
}

func TestWritePythagoreanEllipticalPrism(t *testing.T) {
	t.Parallel()
	d := PythagoreanEllipticalPrismDefinition{Name: "Exact 3:4 prism", Origin: geom.Vec3{X: 1, Y: 2, Z: 3}, Reference: geom.Vec3{X: 3, Y: 4}, Axis: geom.Vec3{Z: 1}, MajorRadius: 10, MinorRadius: 5, Height: 3}
	var out bytes.Buffer
	if e := WritePythagoreanEllipticalPrismSTEP(context.Background(), &out, d, 1e-9); e != nil {
		t.Fatal(e)
	}
	m, e := ReadPythagoreanEllipticalPrism(context.Background(), bytes.NewReader(out.Bytes()), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	if m.Name != d.Name {
		t.Fatal(m.Name)
	}
	independent, e := ReadPythagoreanEllipticalPrism(context.Background(), bytes.NewReader(pythagoreanPrismFixture(t)), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	a, _ := prismNativeBytes(m.Solid)
	b, _ := prismNativeBytes(independent.Solid)
	if !bytes.Equal(a, b) {
		t.Fatal("independent original controls differ")
	}
	for _, change := range []func(*PythagoreanEllipticalPrismDefinition){func(d *PythagoreanEllipticalPrismDefinition) { d.Reference.X = 1; d.Reference.Y = 1 }, func(d *PythagoreanEllipticalPrismDefinition) { d.Reference.Z = 1e-12 }, func(d *PythagoreanEllipticalPrismDefinition) { d.MajorRadius = 5; d.MinorRadius = 2 }, func(d *PythagoreanEllipticalPrismDefinition) { d.Reference = geom.Vec3{X: .6, Y: .8} }, func(d *PythagoreanEllipticalPrismDefinition) { d.Axis.Z = 2 }} {
		bad := d
		change(&bad)
		var b bytes.Buffer
		if e := WritePythagoreanEllipticalPrismSTEP(context.Background(), &b, bad, 1e-9); e == nil || b.Len() != 0 {
			t.Fatal("accepted or partial", bad)
		}
	}
}

func TestPythagoreanEllipticalPrismSourceMutations(t *testing.T) {
	t.Parallel()
	raw := string(pythagoreanPrismFixture(t))
	for _, x := range [][2]string{{"(0.6,0.8,-0.)", "(0.600000000000001,0.8,0.)"}, {"#68 = CARTESIAN_POINT('',(1.,2.,6.));", "#68 = CARTESIAN_POINT('',(1.,2.,7.));"}, {"#23 = CARTESIAN_POINT('',(7.,10.,3.));", "#23 = CARTESIAN_POINT('',(7.,11.,3.));"}, {"(0.6,0.8,-0.)", "(-0.6,-0.8,0.)"}} {
		s := strings.Replace(raw, x[0], x[1], 1)
		if s == raw {
			t.Fatal("stale mutation", x)
		}
		m, e := ReadPythagoreanEllipticalPrism(context.Background(), strings.NewReader(s), Limits{}, 1e-9)
		if e == nil || m.Source != nil {
			t.Fatal("accepted rounded/foreign frame", x)
		}
	}
	for _, l := range []Limits{{MaxBytes: 128}, {MaxEntities: 75}, {MaxValues: 64}, {MaxDepth: 1}, {MaxFaces: 5}, {MaxVertices: 7}, {MaxEdgeUses: 23}} {
		if _, e := ReadPythagoreanEllipticalPrism(context.Background(), strings.NewReader(raw), l, 1e-9); e == nil {
			t.Fatal("limits")
		}
	}
	// A proper signed pose of the independent source preserves the exact ratio
	// semantics and changes which global plane is cardinal.
	f, e := Parse(strings.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	for id, en := range f.Entities {
		if id >= 12 && id <= 14 || len(en.Records) != 1 {
			continue
		}
		r := en.Records[0]
		if (r.Type != "CARTESIAN_POINT" && r.Type != "DIRECTION") || len(r.Args[1].Items) != 3 {
			continue
		}
		v, e := mappedRationalCoordinate(f, id, r.Type, 3)
		if e != nil {
			t.Fatal(e)
		}
		q := [3]*big.Rat{new(big.Rat).Neg(v[1]), new(big.Rat).Set(v[2]), new(big.Rat).Neg(v[0])}
		if r.Type == "CARTESIAN_POINT" {
			for i, x := range []int64{5, 7, 9} {
				q[i].Add(q[i], big.NewRat(x, 1))
			}
		}
		for i, x := range q {
			s, e := mappedTerminatingDecimal(x)
			if e != nil {
				t.Fatal(e)
			}
			en.Records[0].Args[1].Items[i] = Value{Kind: Number, Text: s}
		}
		f.Entities[id] = en
	}
	b, e := mappedPlanarSourceBytes(f)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadPythagoreanEllipticalPrism(context.Background(), bytes.NewReader(b), Limits{}, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	props, e := m.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-8})
	if e != nil || props.Centroid.Distance(geom.Vec3{X: 3, Y: 11.5, Z: 8}) > 1e-7 {
		t.Fatal(props, e)
	}
}
func FuzzPythagoreanEllipticalPrism(f *testing.F) {
	f.Add(pythagoreanPrismFixture(f))
	f.Add(modelOnlyPrismFixture(f))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 65536 {
			return
		}
		_, _ = ReadPythagoreanEllipticalPrism(context.Background(), bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 76, MaxValues: 4096, MaxDepth: 16}, 1e-9)
	})
}
