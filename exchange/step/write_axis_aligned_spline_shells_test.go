package step

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func axisAlignedShellFixture(t testing.TB) AxisAlignedSplineShellModel {
	t.Helper()
	raw, e := os.ReadFile("testdata/axis-aligned-spline-shell.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadAxisAlignedSplineShells(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func TestWriteAxisAlignedSplineShells(t *testing.T) {
	m := axisAlignedShellFixture(t)
	m.Bodies[0].Name = "L-shaped source"
	m.LengthUnit = .0254
	var b bytes.Buffer
	if e := WriteAxisAlignedSplineShells(&b, m); e != nil {
		t.Fatal(e)
	}
	raw := append([]byte(nil), b.Bytes()...)
	round, e := ReadAxisAlignedSplineShells(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if round.LengthUnit != m.LengthUnit || round.Bodies[0].Name != m.Bodies[0].Name || len(round.Bodies[0].Uses) != 36 {
		t.Fatal(round)
	}
	for i, f := range round.Bodies[0].Faces {
		if !reflect.DeepEqual(f.Boundary, m.Bodies[0].Faces[i].Boundary) || !reflect.DeepEqual(f.Geometry.Data(), m.Bodies[0].Faces[i].Geometry.Data()) {
			t.Fatal("source support changed")
		}
	}
	if !bytes.Contains(raw, []byte("CLOSED_SHELL")) || !bytes.Contains(raw, []byte("MANIFOLD_SOLID_BREP")) || bytes.Contains(raw, []byte("OPEN_SHELL")) {
		t.Fatal("wrong topology encoding")
	}
	if _, e = ReadOrthogonalSplineOpenShells(bytes.NewReader(raw), Limits{}); e == nil {
		t.Fatal("closed output accepted as open")
	}
}
func TestWriteAxisAlignedSplineShellsReject(t *testing.T) {
	for _, kind := range []string{"missing-use", "sense", "wrong-vertex", "duplicate-face", "empty"} {
		t.Run(kind, func(t *testing.T) {
			m := axisAlignedShellFixture(t)
			b := &m.Bodies[0]
			switch kind {
			case "missing-use":
				b.Uses = b.Uses[1:]
			case "sense":
				b.Uses[0].Forward = !b.Uses[0].Forward
			case "wrong-vertex":
				b.Uses[0].StartVertexID = 999999
			case "duplicate-face":
				b.FaceIDs[0] = b.FaceIDs[1]
			case "empty":
				m.Bodies = nil
			}
			var out bytes.Buffer
			if e := WriteAxisAlignedSplineShells(&out, m); e == nil || out.Len() != 0 {
				t.Fatal("invalid source emitted", e)
			}
		})
	}
	if e := WriteAxisAlignedSplineShells(openShellShortWriter{}, axisAlignedShellFixture(t)); e != io.ErrShortWrite {
		t.Fatal(e)
	}
}
func TestOCCTWriteAxisAlignedSplineShells(t *testing.T) {
	m := axisAlignedShellFixture(t)
	var b bytes.Buffer
	if e := WriteAxisAlignedSplineShells(&b, m); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "native.step")
	if e := os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	out := filepath.Join(t.TempDir(), "mass.json")
	stepOracle(t, out, "step-mass-precise", path)
	raw, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var measure struct{ Volume, Area float64 }
	if e = json.Unmarshal(raw, &measure); e != nil || math.Abs(measure.Volume-36) > 1e-9 || math.Abs(measure.Area-72) > 1e-9 {
		t.Fatal(measure, e)
	}
}
