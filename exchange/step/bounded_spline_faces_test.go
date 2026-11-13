package step

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"testing"
)

func boundedSplineFixture(t *testing.T) BoundedSplineFaceModel {
	t.Helper()
	raw, e := os.ReadFile("testdata/bounded-spline-face.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadBoundedSplineFaces(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Faces) != 1 || m.Faces[0].Domain != (geom.ParameterRectangle{UMin: .3125, UMax: .75, VMin: .25, VMax: .75}) {
		t.Fatal(m)
	}
	if _, e := ReadSplineFaces(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("full-domain reader accepted trim")
	}
	return m
}
func TestBoundedSplineFaceRoundtrip(t *testing.T) {
	m := boundedSplineFixture(t)
	var out bytes.Buffer
	if e := WriteBoundedSplineFaces(&out, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadBoundedSplineFaces(&out, Limits{})
	if e != nil || len(got.Faces) != 1 || got.Faces[0].Domain != m.Faces[0].Domain {
		t.Fatal(got, e)
	}
	for _, uv := range [][2]float64{{.3125, .25}, {.5, .5}, {.75, .75}} {
		p, e := got.Faces[0].Geometry.Eval(uv[0], uv[1])
		x := 84 * uv[0] / (1 + uv[0])
		if e != nil || math.Abs(p.X-x) > 1e-12 || math.Abs(p.Y-2*uv[1]) > 1e-12 || math.Abs(p.Z-x*uv[1]/2) > 1e-12 {
			t.Fatal(p, e)
		}
	}
}
func TestOCCTBoundedSplineFace(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, source, "write-step-bounded-spline-face")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadBoundedSplineFaces(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e := WriteBoundedSplineFaces(&b, m); e != nil {
		t.Fatal(e)
	}
	ours := filepath.Join(t.TempDir(), "ours.step")
	if e := os.WriteFile(ours, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	area := 0.
	for _, path := range []string{source, ours} {
		out := filepath.Join(t.TempDir(), "result.json")
		stepOracle(t, out, "step-surface", path)
		raw, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var result struct {
			Faces int
			Area  float64
		}
		if e := json.Unmarshal(raw, &result); e != nil || result.Faces != 1 || result.Area <= 0 {
			t.Fatal(result, e)
		}
		if area == 0 {
			area = result.Area
		} else if math.Abs(result.Area-area) > 1e-8 {
			t.Fatal(area, result)
		}
	}
}
func TestBoundedSplineFaceUnrepresentable(t *testing.T) {
	m := boundedSplineFixture(t)
	m.Faces[0].Domain.UMin = .25
	var b bytes.Buffer
	e := WriteBoundedSplineFaces(&b, m)
	if !errors.Is(e, geom.ErrUnrepresentable) || b.Len() != 0 {
		t.Fatal(e, b.Len())
	}
}

func TestBoundedSplineFaceMalformed(t *testing.T) {
	raw, e := os.ReadFile("testdata/bounded-spline-face.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"uv-outside", "uv-collapsed", "vertex", "model", "sense"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			mutate := func(id int64, fn func(*Record)) { ent := f.Entities[id]; fn(&ent.Records[0]); f.Entities[id] = ent }
			switch kind {
			case "uv-outside":
				mutate(38, func(r *Record) { r.Args[1].Items[0] = Value{Kind: Number, Text: "-0.25"} })
			case "uv-collapsed":
				mutate(38, func(r *Record) { r.Args[1].Items[0] = Value{Kind: Number, Text: "0.75"} })
			case "vertex":
				mutate(23, func(r *Record) { r.Args[1].Items[0] = Value{Kind: Number, Text: "20.000000000000004"} })
			case "model":
				mutate(49, func(r *Record) { r.Args[1].Items[2] = Value{Kind: Number, Text: "5.5"} })
			case "sense":
				mutate(20, func(r *Record) { r.Args[4].Text = "T" })
			}
			if _, e := decodeBoundedSplineFace(f, 17, 100); e == nil {
				t.Fatal("accepted malformed bounded face")
			}
		})
	}
	m := boundedSplineFixture(t)
	m.Faces[0].Domain.UMax = 2
	var out bytes.Buffer
	if e := WriteBoundedSplineFaces(&out, m); e == nil || out.Len() != 0 {
		t.Fatal(e)
	}
	if e := WriteBoundedSplineFaces(shortWriter{}, boundedSplineFixture(t)); e == nil {
		t.Fatal("short writer")
	}
}
func FuzzReadBoundedSplineFaces(f *testing.F) {
	raw, e := os.ReadFile("testdata/bounded-spline-face.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadBoundedSplineFaces(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 2048, MaxValues: 16384, MaxDepth: 24, MaxVertices: 64, MaxFaces: 4, MaxSolids: 4, MaxEdgeUses: 32})
	})
}
