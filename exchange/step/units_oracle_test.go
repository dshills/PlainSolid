package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestOCCTConversionUnits(t *testing.T) {
	for _, item := range []struct {
		name string
		unit float64
	}{{"inch", .0254}, {"foot", .3048}} {
		path := filepath.Join(t.TempDir(), "reference.step")
		stepOracle(t, path, "write-step-"+item.name)
		raw, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		m, e := Read(bytes.NewReader(raw), Limits{})
		if e != nil || m.LengthUnit != item.unit {
			t.Fatal(m.LengthUnit, e)
		}
		p, e := m.Bodies[0].Solid.MassProperties()
		if e != nil || math.Abs(p.Volume*math.Pow(item.unit*1000, 3)-24) > 1e-7 {
			t.Fatal(p, e)
		}
		var b bytes.Buffer
		if e = WriteAdvanced(&b, Model{LengthUnit: item.unit, Bodies: []Body{{"part", advancedSolids(t)[0]}}}); e != nil {
			t.Fatal(e)
		}
		ours := filepath.Join(t.TempDir(), "ours.step")
		if e = os.WriteFile(ours, b.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		out := filepath.Join(t.TempDir(), "mass.json")
		stepOracle(t, out, ours)
		raw, e = os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var got struct{ Volume float64 }
		if e = json.Unmarshal(raw, &got); e != nil {
			t.Fatal(e)
		}
		want := 24 * math.Pow(item.unit*1000, 3)
		if math.Abs(got.Volume-want) > 1e-8*want {
			t.Fatal(got, want)
		}
	}
}
