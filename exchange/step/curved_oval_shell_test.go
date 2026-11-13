package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"plainsolid/geom"
	"testing"
)

func curvedOvalShellFixture(t testing.TB) CurvedOvalOpenShell {
	a := curvedOvalFixture(t)
	a.Holes = []QuadraticOval{a.Outer}
	a.Holes[0].Radii = geom.Vec2{X: 3. / 32, Y: 3. / 32}
	p := []geom.Vec3{}
	sq := []float64{.0625, -.0625, .0625}
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			p = append(p, geom.Vec3{X: .25 + float64(i)/4, Y: .25 + float64(j)/4, Z: 3 * (sq[i] + sq[j])})
		}
	}
	s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	d := CurvedOvalFace{Name: "independent curved fill", Geometry: s, Outer: a.Outer, LengthUnit: a.LengthUnit}
	return CurvedOvalOpenShell{Annulus: a, Disk: d, LengthUnit: a.LengthUnit}
}
func TestCurvedOvalOpenShell(t *testing.T) {
	m := curvedOvalShellFixture(t)
	var b bytes.Buffer
	if e := WriteCurvedOvalOpenShell(&b, m, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	got, e := ReadCurvedOvalOpenShell(bytes.NewReader(b.Bytes()), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if got.Annulus.Name != m.Annulus.Name || got.Disk.Name != m.Disk.Name || got.LengthUnit != m.LengthUnit {
		t.Fatal(got)
	}
	seen := map[int64]bool{}
	for _, id := range got.SourceSharedEdges {
		if id == 0 || seen[id] {
			t.Fatal("seams", got.SourceSharedEdges)
		}
		seen[id] = true
	}
	for _, id := range got.Disk.SourceEdges {
		if !seen[id] {
			t.Fatal("disk ancestry")
		}
	}
	if _, e = ReadCurvedOvalFace(bytes.NewReader(b.Bytes()), Limits{}, CurvedOvalOptions{}); e == nil {
		t.Fatal("single face accepted shell")
	}
}
func TestCurvedOvalOpenShellOCCT(t *testing.T) {
	path := t.TempDir() + "/source.step"
	stepOracle(t, path, "write-step-curved-oval-shell")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	got, e := ReadCurvedOvalOpenShell(bytes.NewReader(raw), Limits{}, CurvedOvalOptions{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteCurvedOvalOpenShell(&b, got, CurvedOvalOptions{}); e != nil {
		t.Fatal(e)
	}
	target := t.TempDir() + "/native.step"
	if e = os.WriteFile(target, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	areas := []float64{}
	for _, p := range []string{path, target} {
		out := t.TempDir() + "/mass.json"
		stepOracle(t, out, "step-mass-precise", p)
		data, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var m struct{ Area float64 }
		if e = json.Unmarshal(data, &m); e != nil || m.Area <= 0 {
			t.Fatal(m, e)
		}
		areas = append(areas, m.Area)
	}
	if math.Abs(areas[0]-areas[1]) > 1e-10*math.Max(1, areas[0]) {
		t.Fatal(areas)
	}
}
