package iges

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"plainsolid/geom"
	"testing"
)

func quadraticSheetModel(t *testing.T) Model {
	t.Helper()
	a := ovalAnnulus(t)
	s, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{}, {Y: 6}, {X: 4}, {X: 4, Y: 6}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	b, e := NewQuadraticOvalSurface(s, QuadraticOval{Center: geom.Vec2{X: .25, Y: .25}, Radii: geom.Vec2{X: .0625, Y: .0625}, QuarterWeight: .75}, .001)
	if e != nil {
		t.Fatal(e)
	}
	joined, e := NewQuadraticOvalSheet(a, b)
	if e != nil {
		t.Fatal(e)
	}
	a = joined.Model
	return a
}
func TestQuadraticOvalSheet(t *testing.T) {
	m := quadraticSheetModel(t)
	q, e := CertifyQuadraticOvalSheet(m)
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteQuadraticOvalSheet(&b, q); e != nil {
		t.Fatal(e)
	}
	got, e := ReadQuadraticOvalSheet(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if got.Annulus != 0 || got.Disk != 1 || got.SeamModelCurves[0] == got.SeamModelCurves[1] {
		t.Fatal(got)
	}
	for _, kind := range []string{"unowned", "wrong-hole", "wrong-model", "same-owner"} {
		t.Run(kind, func(t *testing.T) {
			m := quadraticSheetModel(t)
			switch kind {
			case "unowned":
				m.Entities = append(m.Entities, m.Entities[0])
			case "wrong-hole":
				m.TrimmedSurfaces[0].Inner = []int{2}
			case "wrong-model":
				m.CurvesOnSurfaces[2].ModelCurve = m.CurvesOnSurfaces[0].ModelCurve
			case "same-owner":
				m.TrimmedSurfaces[1].Surface = m.TrimmedSurfaces[0].Surface
			}
			var out bytes.Buffer
			if e := WriteQuadraticOvalSheet(&out, QuadraticOvalSheet{Model: m}); e == nil || out.Len() != 0 {
				t.Fatal("invalid sheet emitted")
			}
		})
	}
}
func TestQuadraticOvalSheetOCCT(t *testing.T) {
	if os.Getenv("PLAINSOLID_OCCT_IGES") == "" {
		t.Skip("optional OCCT")
	}
	var b bytes.Buffer
	if e := WriteQuadraticOvalSheet(&b, QuadraticOvalSheet{Model: quadraticSheetModel(t)}); e != nil {
		t.Fatal(e)
	}
	path := t.TempDir() + "/sheet.igs"
	if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := t.TempDir() + "/area.json"
	oracleCommand(t, out, "iges-surface", path)
	raw, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var result struct {
		Faces int
		Area  float64
	}
	if e = json.Unmarshal(raw, &result); e != nil || result.Faces != 2 {
		t.Fatal(result, e)
	}
	area := result.Area
	source, e := os.ReadFile("testdata/quadratic-trim.igs")
	if e != nil {
		t.Fatal(e)
	}
	ref := t.TempDir() + "/outer.igs"
	if e = os.WriteFile(ref, source, 0600); e != nil {
		t.Fatal(e)
	}
	if want := rectangleOracleArea(t, ref); math.Abs(area-want) > 1e-6 {
		t.Fatal(area, want)
	}
}
