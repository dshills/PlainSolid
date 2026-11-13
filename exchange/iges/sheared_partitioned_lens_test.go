package iges

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"plainsolid/brep/parametric"
	"reflect"
	"testing"
)

func TestShearedPartitionedIGESLens(t *testing.T) {
	raw, e := os.ReadFile("testdata/sheared-partitioned-lens.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadPartitionedRationalSplineLensSolid(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.Body.Shear != -15.0/128 || len(m.Pieces) != 6 {
		t.Fatal(m.Body.Shear, len(m.Pieces))
	}
	if len(m.Body.Lower.Data().KnotsV) <= 6 {
		t.Fatal("source chart not refined")
	}
	for _, native := range []bool{false, true} {
		q := m
		if native {
			q, e = NewPartitionedSplineLensSolid(m.Body)
			if e != nil {
				t.Fatal(e)
			}
		}
		var b bytes.Buffer
		if e = WritePartitionedRationalSplineLensSolid(&b, q); e != nil {
			t.Fatal(e)
		}
		z, e := ReadPartitionedRationalSplineLensSolid(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if z.Body.Shear != m.Body.Shear || !reflect.DeepEqual(z.Body.Upper.Data(), m.Body.Upper.Data()) || !reflect.DeepEqual(z.Body.Lower.Data(), m.Body.Lower.Data()) {
			t.Fatal("original support lost")
		}
		if !native && !reflect.DeepEqual(z.Curves, m.Curves) {
			t.Fatal("source curve bases lost")
		}
		for i, p := range m.Pieces {
			v := z.Pieces[i]
			if p.Quarter != v.Quarter || p.Start.Cmp(v.Start) != 0 || p.End.Cmp(v.End) != 0 || p.Cells != v.Cells {
				t.Fatal("cut lineage lost")
			}
		}
	}
	for _, kind := range []string{"shear", "owner", "model", "ordering", "same-image-reparameterized"} {
		t.Run(kind, func(t *testing.T) {
			q, e := ReadPartitionedRationalSplineLensSolid(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "shear":
				q.Body.Shear = math.Nextafter(q.Body.Shear, 0)
			case "owner":
				q.Curves[0][1].Poles[1].X = math.Nextafter(q.Curves[0][1].Poles[1].X, 1)
			case "model":
				q.Curves[0][0].Poles[1].X = math.Nextafter(q.Curves[0][0].Poles[1].X, 1)
			case "same-image-reparameterized":
				// Reversed opposite-quarter weights describe the same conic arc,
				// but do not preserve the required source quarter interval.
				for owner := range q.Curves[2] {
					q.Curves[2][owner].Weights = []float64{8, 6, 5}
				}
			case "ordering":
				q.Curves[0], q.Curves[1] = q.Curves[1], q.Curves[0]
			}
			var b bytes.Buffer
			if e = WritePartitionedRationalSplineLensSolid(&b, q); e == nil || b.Len() != 0 {
				t.Fatal("invalid source emitted")
			}
		})
	}
}
func TestShearedPartitionedIGESLensOCCT(t *testing.T) {
	if os.Getenv("PLAINSOLID_OCCT_IGES") == "" {
		t.Skip("optional OCCT")
	}
	source := filepath.Join(t.TempDir(), "source.igs")
	oracleCommand(t, source, "write-iges-sheared-partitioned-lens")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadPartitionedRationalSplineLensSolid(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := m.Body.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	r := 15.0 / 64
	want := math.Pi * math.Pow(r, 4) / 2
	if math.Abs(p.Volume-want) > 1e-11 {
		t.Fatal(p.Volume, want)
	}
	native, e := NewPartitionedSplineLensSolid(m.Body)
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WritePartitionedRationalSplineLensSolid(&b, native); e != nil {
		t.Fatal(e)
	}
	target := filepath.Join(t.TempDir(), "target.igs")
	if e = os.WriteFile(target, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	for _, path := range []string{source, target} {
		out := filepath.Join(t.TempDir(), "mass.json")
		oracleCommand(t, out, "iges-solid", path)
		data, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		var q struct {
			Solids, Faces, Edges, Vertices int
			Volume, Area                   float64
		}
		if e = json.Unmarshal(data, &q); e != nil {
			t.Fatal(e)
		}
		if q.Solids != 1 || q.Faces != 2 || q.Edges != 6 || q.Vertices != 6 || math.Abs(q.Volume-want) > 1e-10 || math.Abs(q.Area-p.Area) > 1e-9 {
			t.Fatal(q, p)
		}
	}
}
