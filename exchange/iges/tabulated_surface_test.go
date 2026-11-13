package iges

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/big"
	"os"
	"plainsolid/geom"
	"strings"
	"testing"
)

func tabulatedFixture(t *testing.T) Model {
	t.Helper()
	b, e := os.ReadFile("testdata/tabulated-surface.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadTabulatedSurfaces(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func TestIGESTabulatedRoundtrip(t *testing.T) {
	m := tabulatedFixture(t)
	s, e := m.TabulatedGeometry(0)
	if e != nil {
		t.Fatal(e)
	}
	p, e := s.Eval(.5, .25)
	if e != nil || p.Distance(geom.Vec3{X: 3, Y: .75, Z: 7. / 3}) > 1e-14 {
		t.Fatal(p, e)
	}
	du, dv, e := s.Partials(.5, .25)
	if e != nil || du.Distance(geom.Vec3{X: 4. / 3, Z: 4}) > 1e-14 || dv != (geom.Vec3{Y: 3, Z: 4}) {
		t.Fatal(du, dv, e)
	}
	end, _ := s.Eval(0, 1)
	if end != m.TabulatedSurfaces[0].End {
		t.Fatal("terminal point was interpreted as vector", end)
	}
	m.TabulatedSurfaces[0].Label = "SWEEP"
	m.TabulatedSurfaces[0].Subscript = 7
	var b bytes.Buffer
	if e = WriteTabulatedSurfaces(&b, m); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), b.Bytes()...)
	got, e := ReadTabulatedSurfaces(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if got.TabulatedSurfaces[0].Label != "SWEEP" || got.TabulatedSurfaces[0].Subscript != 7 || got.LengthUnit != m.LengthUnit {
		t.Fatal(got)
	}
	var again bytes.Buffer
	if e = WriteTabulatedSurfaces(&again, got); e != nil || !bytes.Equal(raw, again.Bytes()) {
		t.Fatal(e, "noncanonical")
	}
	if _, e = Read(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("generic reader broadened")
	}
	if e = Write(&bytes.Buffer{}, m); e == nil {
		t.Fatal("generic writer broadened")
	}
	for _, l := range []Limits{{MaxBytes: 80}, {MaxEntities: 1}, {MaxControlPoints: 2}} {
		if _, e = ReadTabulatedSurfaces(bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
}
func TestIGESTabulatedRejects(t *testing.T) {
	for _, mutate := range []func(*Model){
		func(m *Model) { m.TabulatedSurfaces[0].End = m.Entities[0].Spline.Curve.Poles()[0] }, func(m *Model) { m.TabulatedSurfaces[0].End.X = .1 }, func(m *Model) { m.TabulatedSurfaces[0].End.X = math.Inf(1) }, func(m *Model) { m.TabulatedSurfaces[0].Directrix = 99 }, func(m *Model) { m.TabulatedSurfaces = append(m.TabulatedSurfaces, m.TabulatedSurfaces[0]) }, func(m *Model) { m.Entities = append(m.Entities, m.Entities[0]) }, func(m *Model) { m.Entities[0].Spline.Start = .25 }, func(m *Model) { m.Entities[0].Spline.Periodic = true }, func(m *Model) { a := geom.Identity(); m.Entities[0].Transform = &a }, func(m *Model) { m.Revolutions = []RevolutionSurface{{}} },
	} {
		m := tabulatedFixture(t)
		mutate(&m)
		var b bytes.Buffer
		if e := WriteTabulatedSurfaces(&b, m); e == nil || b.Len() != 0 {
			t.Fatal("invalid writer not atomic", e)
		}
	}
	m := tabulatedFixture(t)
	c, _ := geom.NewBSpline(1, []geom.Vec3{{X: math.Ldexp(1, -100)}, {X: 1}}, []float64{0, 0, 1, 1}, nil)
	m.Entities[0].Spline = &Spline{Curve: c, Start: 0, End: 1}
	m.TabulatedSurfaces[0].End = geom.Vec3{X: 2}
	if _, e := m.TabulatedGeometry(0); e == nil {
		t.Fatal("unrepresentable translated pole")
	}
}
func TestIGESTabulatedSignedUnitsAndSource(t *testing.T) {
	m := tabulatedFixture(t)
	m.LengthUnit = .3048
	m.TabulatedSurfaces[0].End = geom.Vec3{X: 2, Y: -3, Z: -4}
	var b bytes.Buffer
	if e := WriteTabulatedSurfaces(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadTabulatedSurfaces(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	s, e := got.TabulatedGeometry(0)
	if e != nil {
		t.Fatal(e)
	}
	_, dv, _ := s.Partials(.5, .5)
	if dv != (geom.Vec3{Y: -3, Z: -4}) || got.LengthUnit != m.LengthUnit {
		t.Fatal(dv)
	}
	raw, e := os.ReadFile("testdata/tabulated-surface.igs")
	if e != nil {
		t.Fatal(e)
	}
	lines := strings.Split(string(raw), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, "122,") {
			end := strings.IndexByte(line, ';')
			v := strings.Split(line[:end], ",")
			v[2] = "2.00000000000000000001"
			data := strings.Join(v, ",") + ";"
			lines[i] = fmt.Sprintf("%-64s", data) + line[64:]
			found = true
		}
	}
	if !found {
		t.Fatal("mutation missed")
	}
	if _, e = ReadTabulatedSurfaces(strings.NewReader(strings.Join(lines, "\n")), Limits{}); e == nil {
		t.Fatal("rounded source terminal")
	}
}
func FuzzIGESTabulated(f *testing.F) {
	if b, e := os.ReadFile("testdata/tabulated-surface.igs"); e == nil {
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadTabulatedSurfaces(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxRecords: 512, MaxEntities: 32, MaxValues: 4096, MaxControlPoints: 64})
	})
}

func TestIGESTabulatedExactRationalIdentity(t *testing.T) {
	m := tabulatedFixture(t)
	s, e := m.TabulatedGeometry(0)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	jet, e := p.EvaluateRational(context.Background(), big.NewRat(1, 3), big.NewRat(2, 7), geom.SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i, q := range []*big.Rat{big.NewRat(19, 7), big.NewRat(6, 7), big.NewRat(13, 7)} {
		if jet.Point[i].Cmp(q) != 0 {
			t.Fatal(jet.Point)
		}
	}
	for i, q := range []*big.Rat{big.NewRat(99, 49), new(big.Rat), big.NewRat(162, 49)} {
		if jet.Du[i].Cmp(q) != 0 {
			t.Fatal(jet.Du)
		}
	}
	for i, q := range []*big.Rat{new(big.Rat), big.NewRat(3, 1), big.NewRat(4, 1)} {
		if jet.Dv[i].Cmp(q) != 0 {
			t.Fatal(jet.Dv)
		}
	}
}
func TestIGESTabulatedContinuousKnotGate(t *testing.T) {
	m := tabulatedFixture(t)
	for _, discontinuous := range []bool{false, true} {
		p := []geom.Vec3{{X: 2}, {X: 3, Z: 1}, {X: 4, Z: 2}}
		knots := []float64{0, 0, .5, 1, 1}
		if discontinuous {
			p = append(p, geom.Vec3{X: 5, Z: 3})
			knots = []float64{0, 0, .5, .5, 1, 1}
		}
		c, e := geom.NewBSpline(1, p, knots, nil)
		if e != nil {
			t.Fatal(e)
		}
		m.Entities[0].Spline = &Spline{Curve: c, Start: 0, End: 1, Polynomial: true}
		s, e := m.TabulatedGeometry(0)
		if discontinuous {
			if e == nil {
				t.Fatal("discontinuous accepted")
			}
			continue
		}
		if e != nil {
			t.Fatal(e)
		}
		for _, u := range []float64{.25, .5, .75} {
			point, _ := s.Eval(u, .5)
			direct, _ := c.Eval(u)
			if point.Distance(direct.Add(geom.Vec3{Y: 1.5, Z: 2})) > 1e-14 {
				t.Fatal(point, direct)
			}
		}
	}
}
