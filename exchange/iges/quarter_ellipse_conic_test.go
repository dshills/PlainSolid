package iges

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"os"
	"plainsolid/geom"
	"testing"
)

func TestQuarterEllipseConic(t *testing.T) {
	for _, file := range []string{"occt-quarter-ellipse-conic.igs", "occt-reverse-quarter-ellipse-conic.igs"} {
		raw, e := os.ReadFile("testdata/" + file)
		if e != nil {
			t.Fatal(e)
		}
		m, e := ReadQuarterEllipseConic(bytes.NewReader(raw), Limits{})
		expected := 0
		if file == "occt-reverse-quarter-ellipse-conic.igs" {
			expected = 3
		}
		if e != nil || m.StartQuarter != expected {
			t.Fatal(e)
		}
		if _, e = ReadEllipseConic(bytes.NewReader(raw), Limits{}); e == nil {
			t.Fatal("full reader widened")
		}
		for q := 0; q < 4; q++ {
			m.StartQuarter = q
			var b bytes.Buffer
			if e = WriteQuarterEllipseConic(&b, m); e != nil {
				t.Fatal(e)
			}
			n, e := ReadQuarterEllipseConic(bytes.NewReader(b.Bytes()), Limits{})
			if e != nil || n.StartQuarter != q {
				t.Fatal(e)
			}
			exact, e := n.ExactCurve(context.Background(), geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			analytic, e := n.Curve.Geometry()
			if e != nil {
				t.Fatal(e)
			}
			lo, hi, e := n.AngularRange()
			if e != nil || lo != float64(q)*math.Pi/2 || hi != float64(q+1)*math.Pi/2 {
				t.Fatal("range")
			}
			for _, u := range []float64{0, .25, .5, .75, 1} {
				p, e := exact.Eval(u)
				if e != nil {
					t.Fatal(e)
				}
				want, _ := analytic.Eval(lo + 2*math.Atan(u))
				if p.Distance(want) > 1e-12 {
					t.Fatal(q, u, p, want)
				}
			}
		}
		m.StartQuarter = 0
		c, e := m.ExactCurve(context.Background(), geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		span, _, e := c.BezierControlNet(context.Background(), geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		var hom [4]*big.Rat
		for j := 0; j < 4; j++ {
			hom[j] = new(big.Rat).Add(span.Control[0][j], new(big.Rat).Mul(big.NewRat(2, 1), span.Control[1][j]))
			hom[j].Add(hom[j], span.Control[2][j])
		}
		p := [3]*big.Rat{}
		for j := range p {
			p[j] = new(big.Rat).Quo(hom[j], hom[3])
		}
		if _, e = c.ExactPoint(.5); e == nil {
			t.Fatal("nonbinary exact point rounded")
		}

		z := big.NewRat(23, 5)
		if file == "occt-reverse-quarter-ellipse-conic.igs" {
			z = big.NewRat(7, 5)
		}
		if p[0].Cmp(big.NewRat(1, 1)) != 0 || p[1].Cmp(big.NewRat(5, 1)) != 0 || p[2].Cmp(z) != 0 {
			t.Fatal(p)
		}
		if _, e = m.ExactCurve(context.Background(), geom.SurfacePatchOptions{MaxOperations: 1}); e == nil {
			t.Fatal("budget")
		}
		mutations := [][2]string{{"5.,0.,0.,2.;", "0.,2.,5.,0.;"}, {"5.,0.,0.,2.;", "5.,0.,5.,0.;"}, {"5.,0.,0.,2.;", "5.,0.,0.,1.;"}}
		if expected == 3 {
			mutations = [][2]string{{"0.,-2.,5.,0.;", "5.,0.,0.,-2.;"}, {"0.,-2.,5.,0.;", "0.,-2.,0.,-2.;"}, {"0.,-2.,5.,0.;", "0.,-2.,4.,0.;"}}
		}
		for _, pair := range mutations {
			bad := bytes.Replace(raw, []byte(pair[0]), []byte(pair[1]), 1)
			if bytes.Equal(bad, raw) {
				t.Fatal("missing mutation")
			}
			if _, e = ReadQuarterEllipseConic(bytes.NewReader(bad), Limits{}); e == nil {
				t.Fatal("complement/full/invalid endpoint accepted")
			}
		}
		m.StartQuarter = 4
		var b bytes.Buffer
		if e = WriteQuarterEllipseConic(&b, m); e == nil || b.Len() != 0 {
			t.Fatal("invalid quarter")
		}
	}
}
func FuzzQuarterEllipseConic(f *testing.F) {
	for _, name := range []string{"occt-quarter-ellipse-conic.igs", "occt-reverse-quarter-ellipse-conic.igs"} {
		b, e := os.ReadFile("testdata/" + name)
		if e != nil {
			f.Fatal(e)
		}
		f.Add(b)
	}
	f.Add([]byte("invalid"))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 16384 {
			t.Skip()
		}
		_, _ = ReadQuarterEllipseConic(bytes.NewReader(b), Limits{MaxBytes: 16384, MaxRecords: 128, MaxEntities: 2, MaxValues: 256})
	})
}

func TestQuarterEllipseConicReversedImage(t *testing.T) {
	var curves [2]*geom.ExactSplineCurve
	for i, file := range []string{"occt-quarter-ellipse-conic.igs", "occt-reverse-quarter-ellipse-conic.igs"} {
		raw, e := os.ReadFile("testdata/" + file)
		if e != nil {
			t.Fatal(e)
		}
		m, e := ReadQuarterEllipseConic(bytes.NewReader(raw), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		curves[i], e = m.ExactCurve(context.Background(), geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
	}
	for _, t0 := range []float64{0, .25, .5, .75, 1} {
		reverse, e := curves[1].Eval(t0)
		if e != nil {
			t.Fatal(e)
		}
		forward, e := curves[0].Eval((1 - t0) / (1 + t0))
		if e != nil || reverse.Distance(forward) > 1e-12 {
			t.Fatal(reverse, forward, e)
		}
	}
	a, e := curves[0].ExactPoint(0)
	if e != nil {
		t.Fatal(e)
	}
	b, e := curves[1].ExactPoint(1)
	if e != nil || a != b {
		t.Fatal(a, b, e)
	}
	a, e = curves[0].ExactPoint(1)
	if e != nil {
		t.Fatal(e)
	}
	b, e = curves[1].ExactPoint(0)
	if e != nil || a != b {
		t.Fatal(a, b, e)
	}
}
