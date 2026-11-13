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

func TestAffinePartitionedIGESLens(t *testing.T) {
	raw, e := os.ReadFile("testdata/affine-partitioned-lens.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAffinePartitionedSplineLensSolid(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.Body.Shear != -1320.0/8192 || m.Body.Tilt != 660.0/8192 || len(m.Pieces) != 6 {
		t.Fatal(m.Body.Shear, len(m.Pieces))
	}
	if len(m.Body.Lower.Data().KnotsV) <= 6 {
		t.Fatal("source chart not refined")
	}
	var rejected bytes.Buffer
	if e = WritePartitionedRationalSplineLensSolid(&rejected, m); e == nil || rejected.Len() != 0 {
		t.Fatal("old writer admitted tilt")
	}
	if _, e = ReadPartitionedRationalSplineLensSolid(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("old reader admitted tilt")
	}
	for _, native := range []bool{false, true} {
		q := m
		if native {
			q, e = NewAffinePartitionedSplineLensSolid(m.Body)
			if e != nil {
				t.Fatal(e)
			}
		}
		var b bytes.Buffer
		if e = WriteAffinePartitionedSplineLensSolid(&b, q); e != nil {
			t.Fatal(e)
		}
		z, e := ReadAffinePartitionedSplineLensSolid(bytes.NewReader(b.Bytes()), Limits{})
		if e != nil {
			t.Fatal(e)
		}
		if z.Body.Shear != m.Body.Shear || z.Body.Tilt != m.Body.Tilt || !reflect.DeepEqual(z.Body.Upper.Data(), m.Body.Upper.Data()) || !reflect.DeepEqual(z.Body.Lower.Data(), m.Body.Lower.Data()) {
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
	for _, kind := range []string{"shear", "owner", "model", "ordering", "tilt", "singular"} {
		t.Run(kind, func(t *testing.T) {
			q, e := ReadAffinePartitionedSplineLensSolid(bytes.NewReader(raw), Limits{})
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
			case "tilt":
				q.Body.Tilt = math.Nextafter(q.Body.Tilt, 1)
			case "singular":
				q.Body.Tilt = q.Body.Radii.Y
				q.Body.Shear = q.Body.Radii.X
			case "ordering":
				q.Curves[0], q.Curves[1] = q.Curves[1], q.Curves[0]
			}
			var b bytes.Buffer
			if e = WriteAffinePartitionedSplineLensSolid(&b, q); e == nil || b.Len() != 0 {
				t.Fatal("invalid source emitted")
			}
		})
	}
}
func TestAffinePartitionedIGESLensOCCT(t *testing.T) {
	if os.Getenv("PLAINSOLID_OCCT_IGES") == "" {
		t.Skip("optional OCCT")
	}
	source := filepath.Join(t.TempDir(), "source.igs")
	oracleCommand(t, source, "write-iges-affine-partitioned-lens")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAffinePartitionedSplineLensSolid(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := m.Body.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	r := 165.0 / 8192
	want := 2500 * math.Pi * math.Pow(r, 4)
	if math.Abs(p.Volume-want) > 1e-11 {
		t.Fatal(p.Volume, want)
	}
	native, e := NewAffinePartitionedSplineLensSolid(m.Body)
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteAffinePartitionedSplineLensSolid(&b, native); e != nil {
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
