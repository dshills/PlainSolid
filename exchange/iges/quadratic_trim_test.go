package iges

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"plainsolid/geom"
	"testing"
)

func quadraticOvalModel(t *testing.T) Model {
	t.Helper()
	m := planarMappingModel(t)
	got, e := NewQuadraticOvalSurface(m.Entities[0].Surface.Geometry, QuadraticOval{Center: geom.Vec2{X: .5, Y: .5}, Radii: geom.Vec2{X: .25, Y: .25}, QuarterWeight: .75}, .001)
	if e != nil {
		t.Fatal(e)
	}
	return got
}
func TestQuadraticOvalSurface(t *testing.T) {
	m := quadraticOvalModel(t)
	var b bytes.Buffer
	if e := Write(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := Read(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := got.QuadraticOvalBoundary(got.TrimmedSurfaces[0])
	if e != nil || q.QuarterWeight != .75 || q.Radii.X != .25 {
		t.Fatal(q, e)
	}
	for _, kind := range []string{"weight", "reverse", "model", "hole", "touch"} {
		t.Run(kind, func(t *testing.T) {
			m := quadraticOvalModel(t)
			uv := m.Entities[1].Spline
			points := uv.Curve.Poles()
			weights := uv.Curve.Weights()
			switch kind {
			case "weight":
				weights[3] = .5
			case "reverse":
				for i, j := 0, len(points)-1; i < j; i, j = i+1, j-1 {
					points[i], points[j] = points[j], points[i]
				}
			case "model":
				m.Entities[2].Spline = uv
			case "hole":
				m.TrimmedSurfaces[0].Inner = []int{0}
			case "touch":
				for i := range points {
					points[i].X *= 2
				}
			}
			var e error
			uv.Curve, e = geom.NewBSpline(2, points, uv.Curve.Knots(), weights)
			if e != nil {
				t.Fatal(e)
			}
			if e = m.ValidateTrimmedSurface(m.TrimmedSurfaces[0]); e == nil {
				t.Fatal("accepted", kind)
			}
		})
	}
}
func TestOCCTQuadraticOvalSurface(t *testing.T) {
	source := t.TempDir() + "/source.igs"
	oracleCommand(t, source, "write-iges-quadratic-trim")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := Read(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.TrimmedSurfaces) != 1 {
		t.Fatal(m)
	}
	if _, e = m.QuadraticOvalBoundary(m.TrimmedSurfaces[0]); e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = Write(&b, quadraticOvalModel(t)); e != nil {
		t.Fatal(e)
	}
	native := t.TempDir() + "/native.igs"
	if e = os.WriteFile(native, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	os.WriteFile("/tmp/quadratic-native.igs", b.Bytes(), 0600)
	os.WriteFile("/tmp/quadratic-source.igs", raw, 0600)
	areas := [2]float64{}
	for i, p := range []string{source, native} {
		out := t.TempDir() + "/area.json"
		oracleCommand(t, out, "iges-surface", p)
		data, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var result struct {
			Faces int
			Area  float64
		}
		if e = json.Unmarshal(data, &result); e != nil {
			t.Fatal(e)
		}
		if result.Faces != 1 || result.Area <= 0 {
			t.Fatal(result)
		}
		areas[i] = result.Area
	}
	if math.Abs(areas[0]-areas[1]) > 1e-9 {
		t.Fatal(areas)
	}
}

func TestIndependentQuadraticOvalFixture(t *testing.T) {
	raw, e := os.ReadFile("testdata/quadratic-trim.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := Read(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	q, e := m.QuadraticOvalBoundary(m.TrimmedSurfaces[0])
	if e != nil || q.Center != (geom.Vec2{X: .5, Y: .5}) || q.QuarterWeight != .75 {
		t.Fatal(q, e)
	}
}
func TestParameterRecordsPreserveNumbers(t *testing.T) {
	m := quadraticOvalModel(t)
	var b bytes.Buffer
	if e := Write(&b, m); e != nil {
		t.Fatal(e)
	}
	for _, line := range bytes.Split(b.Bytes(), []byte("\n")) {
		if len(line) == 80 && line[72] == 'P' {
			data := bytes.TrimSpace(line[:64])
			if len(data) == 0 || data[len(data)-1] != ',' && data[len(data)-1] != ';' {
				t.Fatalf("split numeric parameter: %q", data)
			}
		}
	}
}
