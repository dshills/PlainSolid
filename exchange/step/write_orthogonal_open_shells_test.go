package step

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"plainsolid/geom"
	"reflect"
	"testing"
)

func openShellFixture(t testing.TB) OrthogonalSplineOpenShellModel {
	t.Helper()
	raw, e := os.ReadFile("testdata/orthogonal-open-shell.step")
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadOrthogonalSplineOpenShells(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return m
}
func TestWriteOrthogonalOpenShell(t *testing.T) {
	m := openShellFixture(t)
	m.Shells[0].Name = "joined patches"
	m.LengthUnit = .0254
	var b bytes.Buffer
	if e := WriteOrthogonalSplineOpenShells(&b, m); e != nil {
		t.Fatal(e)
	}
	got, e := ReadOrthogonalSplineOpenShells(&b, Limits{})
	if e != nil {
		t.Fatal(e)
	}
	if got.LengthUnit != m.LengthUnit || got.Shells[0].Name != m.Shells[0].Name || len(got.Shells[0].Uses) != 10 {
		t.Fatal(got)
	}
	for i, face := range got.Shells[0].Faces {
		if !reflect.DeepEqual(face.Boundary, m.Shells[0].Faces[i].Boundary) || !reflect.DeepEqual(face.Geometry.Data(), m.Shells[0].Faces[i].Geometry.Data()) {
			t.Fatal("geometry changed")
		}
	}
}
func TestWriteOrthogonalOpenShellReject(t *testing.T) {
	for _, kind := range []string{"sense", "vertex", "edge", "face-index", "face-id", "curve", "empty"} {
		t.Run(kind, func(t *testing.T) {
			m := openShellFixture(t)
			s := &m.Shells[0]
			switch kind {
			case "sense":
				s.Uses[0].Forward = !s.Uses[0].Forward
			case "vertex":
				s.Uses[0].StartVertexID = 99999
			case "edge":
				s.Uses[0].EdgeID = s.Uses[1].EdgeID
			case "face-index":
				s.Uses[0].Face = -1
			case "face-id":
				s.FaceIDs[0] = s.FaceIDs[1]
			case "empty":
				m.Shells = nil
			case "curve":
				g, e := geom.NewBSplineSurface(1, 2, 2, 3, []geom.Vec3{{X: 4}, {X: 4, Y: 1, Z: .5}, {X: 4, Y: 2, Z: .5}, {X: 6}, {X: 6, Y: 1, Z: .375}, {X: 6, Y: 2, Z: .75}}, []float64{4, 4, 6, 6}, []float64{0, 0, 0, 2, 2, 2}, nil)
				if e != nil {
					t.Fatal(e)
				}
				s.Faces[1].Geometry = g
				// Each face remains exactly representable and locally valid. Only
				// the shared curve interior disagrees; endpoints still coincide.
				var local bytes.Buffer
				if e := WriteOrthogonalSplineFaces(&local, OrthogonalSplineFaceModel{LengthUnit: m.LengthUnit, Faces: []OrthogonalSplineFace{s.Faces[1]}}); e != nil {
					t.Fatal(e)
				}

			}
			var b bytes.Buffer
			if e := WriteOrthogonalSplineOpenShells(&b, m); e == nil || b.Len() != 0 {
				t.Fatal("invalid graph emitted", e)
			}
		})
	}
}

type openShellShortWriter struct{}

func (openShellShortWriter) Write(b []byte) (int, error) { return len(b) - 1, nil }
func TestWriteOrthogonalOpenShellIO(t *testing.T) {
	m := openShellFixture(t)
	if e := WriteOrthogonalSplineOpenShells(openShellShortWriter{}, m); e != io.ErrShortWrite {
		t.Fatal(e)
	}
}
func openShellArea(t *testing.T, path string) float64 {
	t.Helper()
	out := filepath.Join(t.TempDir(), "area.json")
	stepOracle(t, out, "step-surface", path)
	raw, e := os.ReadFile(out)
	if e != nil {
		t.Fatal(e)
	}
	var m struct {
		Faces int
		Area  float64
	}
	if e = json.Unmarshal(raw, &m); e != nil || m.Faces != 2 || m.Area <= 0 {
		t.Fatal(m, e)
	}
	return m.Area
}
func TestOCCTWriteOrthogonalOpenShell(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source.step")
	stepOracle(t, source, "write-step-orthogonal-open-shell")
	raw, e := os.ReadFile(source)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadOrthogonalSplineOpenShells(bytes.NewReader(raw), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = WriteOrthogonalSplineOpenShells(&b, m); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "native.step")
	if e = os.WriteFile(path, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	a, c := openShellArea(t, source), openShellArea(t, path)
	if math.Abs(a-c) > 1e-8 {
		t.Fatal(a, c)
	}
}
