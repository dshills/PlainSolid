package iges

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"plainsolid/geom"
	"strings"
	"testing"
)

func igesRevolutionFixture(t *testing.T) Model {
	t.Helper()
	b, e := os.ReadFile("testdata/revolution-surface.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadRevolutionSurfaces(bytes.NewReader(b), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func TestIGESRevolutionRoundtrip(t *testing.T) {
	m := igesRevolutionFixture(t)
	if len(m.Revolutions) != 1 || len(m.Entities) != 2 || m.LengthUnit != .001 {
		t.Fatal(m)
	}
	g, e := m.RevolutionGeometry(0)
	if e != nil {
		t.Fatal(e)
	}
	p, e := g.Eval(.5, .5)
	if e != nil || p.Distance(geom.Vec3{X: 3 * math.Cos(.5), Y: 3 * math.Sin(.5), Z: 4. / 3}) > 1e-13 {
		t.Fatal(p, e)
	}
	m.Revolutions[0].Label = "REVOLVE"
	m.Revolutions[0].Subscript = 2
	var b bytes.Buffer
	if e = WriteRevolutionSurfaces(&b, m); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), b.Bytes()...)
	got, e := ReadRevolutionSurfaces(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if got.Revolutions[0].Label != "REVOLVE" || got.Revolutions[0].Subscript != 2 {
		t.Fatal(got.Revolutions)
	}
	var again bytes.Buffer
	if e = WriteRevolutionSurfaces(&again, got); e != nil || !bytes.Equal(raw, again.Bytes()) {
		t.Fatal(e, "unstable")
	}
	if _, e = Read(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("generic broadened")
	}
	if e = Write(&bytes.Buffer{}, m); e == nil {
		t.Fatal("generic writer broadened")
	}
	for _, l := range []Limits{{MaxBytes: 80}, {MaxEntities: 2}, {MaxControlPoints: 2}} {
		if _, e = ReadRevolutionSurfaces(bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit")
		}
	}
}
func TestIGESRevolutionRejects(t *testing.T) {
	for _, mutate := range []func(*Model){
		func(m *Model) { m.Revolutions[0].Axis = m.Revolutions[0].Generatrix }, func(m *Model) { m.Revolutions[0].Axis = 99 }, func(m *Model) { m.Revolutions = append(m.Revolutions, m.Revolutions[0]) }, func(m *Model) { m.Revolutions[0].AngleEnd = 6.28 }, func(m *Model) { m.Revolutions[0].AngleStart = m.Revolutions[0].AngleEnd }, func(m *Model) { m.Revolutions[0].AngleStart = .1 }, func(m *Model) { m.Entities[m.Revolutions[0].Axis].Line.End.Z = 2 }, func(m *Model) { m.Entities[m.Revolutions[0].Generatrix].Spline.Start = .25 }, func(m *Model) { m.Entities[m.Revolutions[0].Generatrix].Spline.Periodic = true }, func(m *Model) { a := geom.Identity(); m.Entities[m.Revolutions[0].Generatrix].Transform = &a }, func(m *Model) {
			m.Entities = append(m.Entities, Entity{Line: &Line{Start: geom.Vec3{}, End: geom.Vec3{X: 1}}})
		},
	} {
		m := igesRevolutionFixture(t)
		mutate(&m)
		var b bytes.Buffer
		if e := WriteRevolutionSurfaces(&b, m); e == nil || b.Len() != 0 {
			t.Fatal("invalid not atomic", e)
		}
	}
}
func TestIGESRevolutionNegativeAxisAndUnits(t *testing.T) {
	m := igesRevolutionFixture(t)
	m.LengthUnit = .3048
	m.Entities[m.Revolutions[0].Axis].Line.End.Z = -1
	m.Revolutions[0].AngleStart = -1.25
	m.Revolutions[0].AngleEnd = -.25
	var b bytes.Buffer
	if e := WriteRevolutionSurfaces(&b, m); e != nil {
		t.Fatal(e)
	}
	m, e := ReadRevolutionSurfaces(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	g, e := m.RevolutionGeometry(0)
	if e != nil {
		t.Fatal(e)
	}
	p, e := g.Eval(-.5, .5)
	if e != nil || p.Y <= 0 || m.LengthUnit != .3048 {
		t.Fatal(p, e)
	}
}
func TestIGESRevolutionSourceMutation(t *testing.T) {
	raw, e := os.ReadFile("testdata/revolution-surface.igs")
	if e != nil {
		t.Fatal(e)
	}
	lines := strings.Split(string(raw), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, "120,") {
			end := strings.IndexByte(line, ';')
			v := strings.Split(line[:end], ",")
			v[3] = "0.25000000000000000001"
			data := strings.Join(v, ",") + ";"
			if len(data) > 64 {
				t.Fatal(data)
			}
			lines[i] = fmt.Sprintf("%-64s", data) + line[64:]
			found = true
		}
	}
	if !found {
		t.Fatal("mutation missed")
	}
	if _, e = ReadRevolutionSurfaces(strings.NewReader(strings.Join(lines, "\n")), Limits{}); e == nil {
		t.Fatal("rounded source angle")
	}
}
func FuzzIGESRevolution(f *testing.F) {
	if b, e := os.ReadFile("testdata/revolution-surface.igs"); e == nil {
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadRevolutionSurfaces(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxRecords: 512, MaxEntities: 32, MaxValues: 4096, MaxControlPoints: 64})
	})
}
