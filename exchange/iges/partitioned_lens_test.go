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

func TestPartitionedIGESLens(t *testing.T) {
	raw, e := os.ReadFile("testdata/partitioned-lens.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadPartitionedRationalSplineLensSolid(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Pieces) != 6 || len(m.Curves) != 6 {
		t.Fatal("piece count")
	}
	var b bytes.Buffer
	if e = WritePartitionedRationalSplineLensSolid(&b, m); e != nil {
		t.Fatal(e)
	}
	q, e := ReadPartitionedRationalSplineLensSolid(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(m.Curves, q.Curves) || !reflect.DeepEqual(m.Body.Lower.Data(), q.Body.Lower.Data()) {
		t.Fatal("source bases lost")
	}
	for i, p := range m.Pieces {
		z := q.Pieces[i]
		if p.Quarter != z.Quarter || p.Start.Cmp(z.Start) != 0 || p.End.Cmp(z.End) != 0 || p.Cells != z.Cells {
			t.Fatal("partition lineage lost")
		}
	}
	native, e := NewPartitionedSplineLensSolid(m.Body)
	if e != nil {
		t.Fatal(e)
	}
	if len(native.Pieces) != 6 {
		t.Fatal("native piece count")
	}
	b.Reset()
	if e = WriteSplineLensSolid(&b, m.Body); e == nil || b.Len() != 0 {
		t.Fatal("ordinary API discarded partition")
	}
	if _, e = ReadRationalSplineLensSolid(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("four-edge reader broadened")
	}
	for _, kind := range []string{"missing", "cut", "cell", "model", "owner"} {
		t.Run(kind, func(t *testing.T) {
			m, e := ReadPartitionedRationalSplineLensSolid(bytes.NewReader(raw), Limits{})
			if e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "missing":
				m.Curves = m.Curves[:5]
			case "cut":
				m.Pieces[0].End.SetFrac64(2, 3)
			case "cell":
				m.Pieces[0].Cells[0].UMax = .75
			case "model":
				m.Curves[0][0].Poles[1].Z = math.Nextafter(2, 3)
			case "owner":
				m.Curves[0][2] = m.Curves[1][2]
			}
			var b bytes.Buffer
			if e = WritePartitionedRationalSplineLensSolid(&b, m); e == nil || b.Len() != 0 {
				t.Fatal("invalid partition emitted")
			}
		})
	}
}
func TestPartitionedIGESLensOCCT(t *testing.T) {
	if os.Getenv("PLAINSOLID_OCCT_IGES") == "" {
		t.Skip("optional OCCT")
	}
	source := filepath.Join(t.TempDir(), "source.igs")
	oracleCommand(t, source, "write-iges-partitioned-lens")
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
		var r struct {
			Solids, Faces, Edges, Vertices int
			Volume, Area                   float64
		}
		if e = json.Unmarshal(data, &r); e != nil || r.Solids != 1 || r.Faces != 2 || r.Edges != 6 || r.Vertices != 6 || math.Abs(r.Volume-p.Volume) > 1e-10 || math.Abs(r.Area-p.Area) > 1e-9 {
			t.Fatal(r, p, e)
		}
	}
}
func TestConcurrentPartitionedIGESLens(t *testing.T) {
	raw, e := os.ReadFile("testdata/concurrent-partitioned-lens.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadPartitionedRationalSplineLensSolid(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Pieces) != 7 {
		t.Fatal("concurrent cut count")
	}
	var b bytes.Buffer
	if e = WritePartitionedRationalSplineLensSolid(&b, m); e != nil {
		t.Fatal(e)
	}
	q, e := ReadPartitionedRationalSplineLensSolid(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(m.Curves, q.Curves) || !reflect.DeepEqual(m.Body.Upper.Data(), q.Body.Upper.Data()) || !reflect.DeepEqual(m.Body.Lower.Data(), q.Body.Lower.Data()) {
		t.Fatal("source bases changed")
	}
	if os.Getenv("PLAINSOLID_OCCT_IGES") != "" {
		path := filepath.Join(t.TempDir(), "native.igs")
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
			Solids, Faces, Edges, Vertices int
			Volume                         float64
		}
		radius := 263340. / 4194304
		if e = json.Unmarshal(data, &r); e != nil || r.Solids != 1 || r.Faces != 2 || r.Edges != 7 || r.Vertices != 7 || math.Abs(r.Volume-math.Pi*math.Pow(radius, 4)/2) > 1e-10 {
			t.Fatal(r, e)
		}
	}
}
func FuzzPartitionedIGESLens(f *testing.F) {
	for _, name := range []string{"partitioned-lens", "concurrent-partitioned-lens", "sheared-partitioned-lens"} {
		b, e := os.ReadFile("testdata/" + name + ".igs")
		if e != nil {
			f.Fatal(e)
		}
		f.Add(b)
	}
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		_, _ = ReadPartitionedRationalSplineLensSolid(bytes.NewReader(b), Limits{MaxBytes: 128 << 10, MaxRecords: 2000, MaxEntities: 128, MaxValues: 20000, MaxControlPoints: 2048})
	})
}
func TestPartitionedIGESLensUnresolved(t *testing.T) {
	if _, e := readIGESLens(topologySource{}, Limits{}, true, true); e == nil {
		t.Fatal("empty graph accepted")
	}
	raw, e := os.ReadFile("testdata/spline-lens.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplineLensSolid(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(m.Lower, curvedOvalOptions())
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertUKnot(.625, 1, curvedOvalOptions())
	if e != nil {
		t.Fatal(e)
	}
	m.Lower, e = p.ToBSplineSurface()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = NewPartitionedSplineLensSolid(m); e == nil {
		t.Fatal("irrational quarter crossing rounded")
	}
	raw, e = os.ReadFile("testdata/partitioned-lens.igs")
	if e != nil {
		t.Fatal(e)
	}
	for _, l := range []Limits{{MaxBytes: 100}, {MaxEntities: 3}, {MaxControlPoints: 2}} {
		if _, e = ReadPartitionedRationalSplineLensSolid(bytes.NewReader(raw), l); e == nil {
			t.Fatal("limit ignored")
		}
	}
}
