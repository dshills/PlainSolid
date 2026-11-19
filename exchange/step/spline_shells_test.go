package step

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

func TestIndependentSplineShell(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/spline-shell.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplineShells(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Bodies) != 1 || len(m.Bodies[0].Faces) != 6 {
		t.Fatal(m)
	}
}

func splineShellFixture(t *testing.T) SplineShellModel {
	t.Helper()
	raw, e := os.ReadFile("testdata/spline-shell.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplineShells(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func TestSplineShellRoundtrip(t *testing.T) {
	t.Parallel()
	m := splineShellFixture(t)
	var out bytes.Buffer
	if e := WriteSplineShells(&out, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadSplineShells(&out, Limits{})
	if e != nil || len(got.Bodies) != 1 {
		t.Fatal(got, e)
	}
	proof, e := got.Bodies[0].Solid.ValidateSplineRectangleEmbedding(context.Background(), trimmed.SplineRectangleEmbeddingOptions{})
	if e != nil || !proof.Complete {
		t.Fatal(proof, e)
	}
}
func TestOCCTSplineShell(t *testing.T) {
	t.Parallel()
	source := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, source, "write-step-spline-shell")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplineShells(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e := WriteSplineShells(&b, m); e != nil {
		t.Fatal(e)
	}
	ours := filepath.Join(t.TempDir(), "ours.step")
	if e := os.WriteFile(ours, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	referenceArea := 0.
	for _, path := range []string{source, ours} {
		result := filepath.Join(t.TempDir(), "mass.json")
		stepOracle(t, result, path)
		raw, e := os.ReadFile(result)
		if e != nil {
			t.Fatal(e)
		}
		var p struct {
			Volume, Area float64
			Centroid     [3]float64
		}
		if e := json.Unmarshal(raw, &p); e != nil || math.Abs(p.Volume-5) > 1e-7 || math.Abs(p.Centroid[0]-16.0/15) > 1e-7 || math.Abs(p.Centroid[1]-16.0/15) > 1e-7 || math.Abs(p.Centroid[2]-29.0/45) > 1e-7 {
			t.Fatal(p, e)
		}
		if referenceArea == 0 {
			referenceArea = p.Area
		} else if math.Abs(p.Area-referenceArea) > 1e-7 {
			t.Fatal(p, referenceArea)
		}
	}
}

func TestSplineShellInvalidGraph(t *testing.T) {
	t.Parallel()
	raw, e := os.ReadFile("testdata/spline-shell.step")
	if e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"inward", "missing-pcurve", "wrong-owner", "wrong-sense", "invalid-representation"} {
		t.Run(kind, func(t *testing.T) {
			f, e := Parse(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			mutate := func(id int64, fn func(*Record)) { ent := f.Entities[id]; fn(&ent.Records[0]); f.Entities[id] = ent }
			switch kind {
			case "inward":
				for id, ent := range f.Entities {
					if len(ent.Records) == 1 && ent.Records[0].Type == "CARTESIAN_POINT" && len(ent.Records[0].Args[1].Items) == 3 {
						r := ent.Records[0]
						x, e := numeric(r.Args[1].Items[0])
						if e != nil {
							t.Fatal(e)
						}
						r.Args[1].Items[0] = Value{Kind: Number, Text: real(-x)}
						ent.Records[0] = r
						f.Entities[id] = ent
					}
				}
			case "missing-pcurve":
				mutate(26, func(r *Record) { r.Args[2].Items = r.Args[2].Items[:1] })
			case "wrong-owner":
				mutate(42, func(r *Record) { r.Args[1] = Value{Kind: Reference, Ref: 31} })
			case "wrong-sense":
				mutate(20, func(r *Record) { r.Args[4].Text = "T" })
			case "invalid-representation":
				mutate(26, func(r *Record) { r.Args[3].Text = "INVALID" })
			}
			l, _ := (Limits{}).defaults()
			if _, _, e := decodeSplineShell(f, 15, l); e == nil {
				t.Fatal("accepted invalid shell")
			}
		})
	}
	if _, e := ReadSplineShells(bytes.NewReader(raw), Limits{MaxFaces: 5}); e == nil {
		t.Fatal("face budget")
	}
}
func TestSplineShellDisconnected(t *testing.T) {
	t.Parallel()
	m := splineShellFixture(t)
	extra := []BoundedSplineFace{}
	for _, face := range m.Bodies[0].Faces {
		d := face.Geometry.Data()
		for i := range d.Poles {
			d.Poles[i].X += 10
		}
		g, e := geom.NewBSplineSurface(d.DegreeU, d.DegreeV, d.CountU, d.CountV, d.Poles, d.KnotsU, d.KnotsV, d.Weights)
		if e != nil {
			t.Fatal(e)
		}
		extra = append(extra, BoundedSplineFace{face.Name, g, face.Domain})
	}
	m.Bodies[0].Faces = append(m.Bodies[0].Faces, extra...)
	var out bytes.Buffer
	if e := WriteSplineShells(&out, m); e == nil || out.Len() != 0 {
		t.Fatal("accepted disconnected shell", e)
	}
	if e := WriteSplineShells(shortWriter{}, splineShellFixture(t)); e == nil {
		t.Fatal("short writer")
	}
}
func FuzzReadSplineShells(f *testing.F) {
	raw, e := os.ReadFile("testdata/spline-shell.step")
	if e != nil {
		f.Fatal(e)
	}
	f.Add(raw)
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadSplineShells(bytes.NewReader(b), Limits{MaxBytes: 65536, MaxEntities: 2048, MaxValues: 16384, MaxDepth: 24, MaxVertices: 128, MaxFaces: 16, MaxSolids: 2, MaxEdgeUses: 64})
	})
}
