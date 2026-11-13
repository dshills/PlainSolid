package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func TestRationalIGESLens(t *testing.T) {
	for _, name := range []string{"refined-curve-lens", "raw-curve-lens"} {
		t.Run(name, func(t *testing.T) {
			raw, e := os.ReadFile("testdata/" + name + ".igs")
			if e != nil {
				t.Fatal(e)
			}
			m, e := ReadRationalSplineLensSolid(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			if len(m.Curves[0][1].Poles) != 4 || len(m.Curves[0][2].Poles) != 4 || reflect.DeepEqual(m.Curves[0][1].Knots, m.Curves[0][2].Knots) {
				t.Fatal("independent owner bases lost")
			}
			if name == "raw-curve-lens" && exactWeightNormalization(m.Curves[0][0].Weights) {
				t.Fatal("fixture lacks non-normalizable weights")
			}
			var b bytes.Buffer
			if e = WriteRationalSplineLensSolid(&b, m); e != nil {
				t.Fatal(e)
			}
			q, e := ReadRationalSplineLensSolid(bytes.NewReader(b.Bytes()), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			if !reflect.DeepEqual(m.Curves, q.Curves) {
				t.Fatal("raw curve coefficient/basis changed")
			}
			if _, e = ReadSplineLensSolid(bytes.NewReader(raw), Limits{}); e == nil {
				t.Fatal("ordinary lens scope widened")
			}
			b.Reset()
			if e = WriteSplineLensSolid(&b, m.Body); e == nil || b.Len() != 0 {
				t.Fatal("ordinary export discarded raw recipe")
			}
			for _, kind := range []string{"model", "upper", "lower", "weight", "knots"} {
				t.Run(kind, func(t *testing.T) {
					x, e := ReadRationalSplineLensSolid(bytes.NewReader(raw), Limits{})
					if e != nil {
						t.Fatal(e)
					}
					switch kind {
					case "model":
						x.Curves[0][0].Poles[1].Z = math.Nextafter(2, 3)
					case "upper":
						x.Curves[0][1].Poles[1].X = math.Nextafter(x.Curves[0][1].Poles[1].X, 1)
					case "lower":
						x.Curves[0][2].Poles[1].Y = math.Nextafter(x.Curves[0][2].Poles[1].Y, 1)
					case "weight":
						x.Curves[0][0].Weights[1] = 0
					case "knots":
						x.Curves[0][1].Knots[3] = .375
					}
					var out bytes.Buffer
					if e = WriteRationalSplineLensSolid(&out, x); e == nil || out.Len() != 0 {
						t.Fatal("invalid raw recipe emitted")
					}
				})
			}
		})
	}
}
func TestRationalIGESLensOCCT(t *testing.T) {
	if os.Getenv("PLAINSOLID_OCCT_IGES") == "" {
		t.Skip("optional OCCT")
	}
	for _, kind := range []string{"refined", "raw"} {
		t.Run(kind, func(t *testing.T) {
			source := filepath.Join(t.TempDir(), "source.igs")
			oracleCommand(t, source, "write-iges-"+kind+"-curve-lens")
			raw, e := os.ReadFile(source)
			if e != nil {
				t.Fatal(e)
			}
			m, e := ReadRationalSplineLensSolid(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			var b bytes.Buffer
			if e = WriteRationalSplineLensSolid(&b, m); e != nil {
				t.Fatal(e)
			}
			target := filepath.Join(t.TempDir(), "target.igs")
			if e = os.WriteFile(target, b.Bytes(), 0600); e != nil {
				t.Fatal(e)
			}
			p, e := m.Body.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10})
			if e != nil {
				t.Fatal(e)
			}
			for _, path := range []string{source, target} {
				out := filepath.Join(t.TempDir(), "mass.json")
				oracleCommand(t, out, "iges-solid", path)
				data, e := os.ReadFile(out)
				if e != nil {
					t.Fatal(e)
				}
				var r struct {
					Solids, Faces, Edges, Vertices int
					Area, Volume                   float64
				}
				if e = json.Unmarshal(data, &r); e != nil || r.Solids != 1 || r.Faces != 2 || r.Edges != 4 || r.Vertices != 4 || math.Abs(r.Volume-p.Volume) > 1e-10 || math.Abs(r.Area-p.Area) > 1e-9 {
					t.Fatal(r, p, e)
				}
			}
		})
	}
}
func TestRationalIGESLensConcurrentSupport(t *testing.T) {
	raw, e := os.ReadFile("testdata/raw-curve-lens.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadRationalSplineLensSolid(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(m.Body.Lower, curvedOvalOptions())
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertUKnot(.5, 1, curvedOvalOptions())
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertVKnot(.5, 2, curvedOvalOptions())
	if e != nil {
		t.Fatal(e)
	}
	m.Body.Lower, e = p.ToBSplineSurface()
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteRationalSplineLensSolid(&b, m); e != nil {
		t.Fatal(e)
	}
	q, e := ReadRationalSplineLensSolid(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(q.Curves, m.Curves) || !reflect.DeepEqual(q.Body.Lower.Data(), m.Body.Lower.Data()) || q.Body.SourceKnotCells[0] == q.Body.SourceKnotCells[1] {
		t.Fatal("concurrent ancestry lost")
	}
	if os.Getenv("PLAINSOLID_OCCT_IGES") != "" {
		path := filepath.Join(t.TempDir(), "concurrent.igs")
		if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
			t.Fatal(e)
		}
		out := filepath.Join(t.TempDir(), "mass.json")
		oracleCommand(t, out, "iges-solid", path)
		data, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var r struct {
			Solids int
			Volume float64
		}
		if e = json.Unmarshal(data, &r); e != nil || r.Solids != 1 || math.Abs(r.Volume-math.Pi*math.Pow(105./512, 4)/2) > 1e-10 {
			t.Fatal(r, e)
		}
	}
}
func FuzzRationalIGESLens(f *testing.F) {
	for _, n := range []string{"refined-curve-lens", "raw-curve-lens"} {
		b, e := os.ReadFile("testdata/" + n + ".igs")
		if e != nil {
			f.Fatal(e)
		}
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadRationalSplineLensSolid(bytes.NewReader(b), Limits{MaxBytes: 128 << 10, MaxRecords: 2000, MaxEntities: 64, MaxValues: 20000, MaxControlPoints: 1024})
	})
}
