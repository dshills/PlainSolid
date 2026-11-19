package step

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep"
	"plainsolid/geom"
	"strings"
	"testing"
)

func advancedSolids(t *testing.T) []brep.Solid {
	t.Helper()
	box, e := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 3, Z: 4})
	if e != nil {
		t.Fatal(e)
	}
	outer := []geom.Vec3{{}, {X: 4}, {X: 4, Y: 4}, {Y: 4}}
	hole := []geom.Vec3{{X: 1, Y: 1}, {X: 2, Y: 1}, {X: 2, Y: 2}, {X: 1, Y: 2}}
	prism, e := brep.PrismWithHoles(outer, [][]geom.Vec3{hole}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	return []brep.Solid{box, prism}
}
func TestWriteAdvancedTopology(t *testing.T) {
	t.Parallel()
	for _, s := range advancedSolids(t) {
		m := Model{LengthUnit: .001, Bodies: []Body{{"solid", s}}}
		var b, again bytes.Buffer
		if e := WriteAdvanced(&b, m); e != nil {
			t.Fatal(e)
		}
		if e := WriteAdvanced(&again, m); e != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
			t.Fatal("nondeterministic", e)
		}
		for kind, n := range map[string]int{"EDGE_CURVE": len(s.Edges()), "VERTEX_POINT": len(s.Vertices()), "ADVANCED_FACE": len(s.Faces())} {
			if strings.Count(b.String(), "="+kind+"(") != n {
				t.Fatal(kind)
			}
		}
		if strings.Contains(b.String(), "POLY_LOOP(") {
			t.Fatal("faceted graph")
		}
		got, e := Read(&b, Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if got.Bodies[0].Solid.Topology() != s.Topology() {
			t.Fatal("topology changed")
		}
		want, _ := s.MassProperties()
		actual, _ := got.Bodies[0].Solid.MassProperties()
		if math.Abs(want.Volume-actual.Volume) > 1e-10 || math.Abs(want.SurfaceArea-actual.SurfaceArea) > 1e-10 {
			t.Fatal(want, actual)
		}
	}
	if e := WriteAdvanced(nil, Model{}); e == nil {
		t.Fatal("nil writer")
	}
}
func TestOCCTAdvancedSTEPExport(t *testing.T) {
	t.Parallel()
	for i, s := range advancedSolids(t) {
		var b bytes.Buffer
		if e := WriteAdvanced(&b, Model{LengthUnit: .001, Bodies: []Body{{"solid", s}}}); e != nil {
			t.Fatal(e)
		}
		dir := t.TempDir()
		path := filepath.Join(dir, "advanced.step")
		if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		result := filepath.Join(dir, "result.json")
		stepOracle(t, result, path)
		data, e := os.ReadFile(result)
		if e != nil {
			t.Fatal(e)
		}
		var got struct{ Area, Volume float64 }
		if e = json.Unmarshal(data, &got); e != nil {
			t.Fatal(e)
		}
		want, e := s.MassProperties()
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(got.Area-want.SurfaceArea) > 1e-6 || math.Abs(got.Volume-want.Volume) > 1e-6 {
			t.Fatal(i, got, want)
		}
	}
}
