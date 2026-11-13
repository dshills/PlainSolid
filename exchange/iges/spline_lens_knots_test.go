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

func TestIGESRefinedLens(t *testing.T) {
	raw, e := os.ReadFile("testdata/refined-spline-lens.igs")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplineLensSolid(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if m.Lower.Data().CountU == 3 || m.Lower.Data().CountV == 3 {
		t.Fatal("refinement absent")
	}
	for i := 0; i < 4; i++ {
		if m.SourceKnotCells[0][i] == m.SourceKnotCells[1][i] || !m.SourceKnotCells[1][i].Valid() {
			t.Fatal("source knot ancestry lost")
		}
	}
	var b bytes.Buffer
	if e = WriteSplineLensSolid(&b, m); e != nil {
		t.Fatal(e)
	}
	q, e := ReadSplineLensSolid(bytes.NewReader(b.Bytes()), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(q.Lower.Data(), m.Lower.Data()) || q.SourceKnotCells != m.SourceKnotCells {
		t.Fatal("source basis changed")
	}
	// A new knot through the interior of a quarter cannot be hidden by a
	// mathematically equivalent whole-edge model curve in this single-cell API.
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
	b.Reset()
	if e = WriteSplineLensSolid(&b, m); e == nil || b.Len() != 0 {
		t.Fatal("unqualified quarter knot crossing emitted")
	}
}
func TestIGESRefinedLensOCCT(t *testing.T) {
	if os.Getenv("PLAINSOLID_OCCT_IGES") == "" {
		t.Skip("optional OCCT")
	}
	source := filepath.Join(t.TempDir(), "source.igs")
	oracleCommand(t, source, "write-iges-refined-lens")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadSplineLensSolid(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteSplineLensSolid(&b, m); e != nil {
		t.Fatal(e)
	}
	target := filepath.Join(t.TempDir(), "target.igs")
	if e = os.WriteFile(target, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	p, e := m.Solid.MassProperties(context.Background(), parametric.IntegrationOptions{RelativeTolerance: 1e-10})
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
			Volume, Area                   float64
		}
		if e = json.Unmarshal(data, &r); e != nil || r.Solids != 1 || r.Faces != 2 || r.Edges != 4 || r.Vertices != 4 || math.Abs(r.Volume-p.Volume) > 1e-10 || math.Abs(r.Area-p.Area) > 1e-9 {
			t.Fatal(r, p, e)
		}
	}
}
