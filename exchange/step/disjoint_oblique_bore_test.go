package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

func TestDisjointObliqueSTEP(t *testing.T) {
	m := DisjointObliqueBoreModel{Name: "mixed", Stock: geom.Box{Max: geom.Vec3{X: 20, Y: 20, Z: 20}}, LengthUnit: .001, Cuts: []trimmed.CylinderCut{{Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: 2, Y: 4}, Direction: geom.Vec3{X: 1, Z: 4}, Radius: 1}, {Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: 16, Y: 16}, Direction: geom.Vec3{Z: 1}, Radius: 1}}}
	var b bytes.Buffer
	if e := WriteDisjointObliqueBores(&b, m); e != nil {
		t.Fatal(e)
	}
	t.Log(b.Len())
	if os.Getenv("PLAINSOLID_OCCT_STEP") != "" {
		dir := t.TempDir()
		src, dst := filepath.Join(dir, "mixed.step"), filepath.Join(dir, "mass.json")
		if e := os.WriteFile(src, b.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		stepOracle(t, dst, "step-mass-precise", src)
		raw, e := os.ReadFile(dst)
		if e != nil {
			t.Fatal(e)
		}
		var got struct{ Volume, Area float64 }
		if e = json.Unmarshal(raw, &got); e != nil {
			t.Fatal(e)
		}
		length := 5 * math.Sqrt(17)
		v := 8000 - math.Pi*(20+length)
		a := 2400 + 2*math.Pi*(20+length) - 2*math.Pi*(1+math.Sqrt(17)/4)
		if math.Abs(got.Volume-v) > 1e-7 || math.Abs(got.Area-a) > 1e-7 {
			t.Fatal(got, v, a)
		}
	}
	r, e := ReadDisjointObliqueBores(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var c bytes.Buffer
	if e = WriteDisjointObliqueBores(&c, r); e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(b.Bytes(), c.Bytes()) {
		t.Fatal("unstable")
	}
}

func TestDisjointObliqueSTEPVariants(t *testing.T) {
	for _, kind := range []string{"reversed", "multiple-oblique", "three", "translated"} {
		t.Run(kind, func(t *testing.T) {
			m := DisjointObliqueBoreModel{Name: kind, Stock: geom.Box{Max: geom.Vec3{X: 20, Y: 20, Z: 20}}, LengthUnit: .001, Cuts: []trimmed.CylinderCut{{Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: 2, Y: 4}, Direction: geom.Vec3{X: 1, Z: 4}, Radius: 1}, {Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: 16, Y: 16}, Direction: geom.Vec3{Z: 1}, Radius: 1}}}
			switch kind {
			case "reversed":
				m.Cuts[0], m.Cuts[1] = m.Cuts[1], m.Cuts[0]
			case "multiple-oblique":
				m.Cuts[1].Direction = geom.Vec3{X: -1, Z: 8}
			case "three":
				m.Cuts = append(m.Cuts, trimmed.CylinderCut{Entry: 0, Exit: 1, AxisPoint: geom.Vec3{X: 3, Y: 16}, Direction: geom.Vec3{Z: 1}, Radius: 1})
			case "translated":
				d := geom.Vec3{X: 10.25, Y: -7.5, Z: 3.125}
				m.Stock.Min = m.Stock.Min.Add(d)
				m.Stock.Max = m.Stock.Max.Add(d)
				for i := range m.Cuts {
					m.Cuts[i].AxisPoint = m.Cuts[i].AxisPoint.Add(d)
				}
			}
			var b bytes.Buffer
			if e := WriteDisjointObliqueBores(&b, m); e != nil {
				t.Fatal(e)
			}
			r, e := ReadDisjointObliqueBores(bytes.NewReader(b.Bytes()), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			var c bytes.Buffer
			if e = WriteDisjointObliqueBores(&c, r); e != nil || !bytes.Equal(b.Bytes(), c.Bytes()) {
				t.Fatal(e, "unstable")
			}
		})
	}
}
