package step

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestWrappedFaceModel(t *testing.T) {
	var b bytes.Buffer
	input := WrappedSurfaceModel{LengthUnit: .0254, Surfaces: []WrappedSplineSurface{{Name: "first", Geometry: wrappedCylinder(t)}, {Name: "second", Geometry: wrappedCylinder(t)}}}
	if e := WriteWrappedSplineFaces(&b, input); e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedSplineFaces(bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	if m.LengthUnit != input.LengthUnit || len(m.Faces) != 2 || m.Faces[0].Name != "first" || m.Faces[1].Name != "second" || m.FaceIDs[0] == m.FaceIDs[1] {
		t.Fatalf("%+v", m)
	}
	for _, o := range []WrappedSurfaceOptions{{MaxSurfaces: 1}, {MaxOperations: 1}} {
		if _, e = ReadWrappedSplineFaces(bytes.NewReader(b.Bytes()), Limits{}, o); e == nil {
			t.Fatal("accepted budget")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = ReadWrappedSplineFacesContext(ctx, bytes.NewReader(b.Bytes()), Limits{}, WrappedSurfaceOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	bad := strings.ReplaceAll(b.String(), "OPEN_SHELL(", "CLOSED_SHELL(")
	if _, e = ReadWrappedSplineFaces(strings.NewReader(bad), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("accepted closed shell")
	}
}

func TestReadWrappedFaceModelOCCT(t *testing.T) {
	path := t.TempDir() + "/source.step"
	stepOracle(t, path, "write-step-wrapped-surface")
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	m, e := ReadWrappedSplineFaces(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{ContinuityOrder: 1})
	if e != nil {
		t.Fatal(e)
	}
	if len(m.Faces) != 1 || m.LengthUnit != .001 {
		t.Fatal(m)
	}
	var b bytes.Buffer
	model := WrappedSurfaceModel{LengthUnit: m.LengthUnit}
	for _, f := range m.Faces {
		model.Surfaces = append(model.Surfaces, WrappedSplineSurface{Name: f.Name, Geometry: f.Geometry})
	}
	if e = WriteWrappedSplineFaces(&b, model); e != nil {
		t.Fatal(e)
	}
	output := t.TempDir() + "/result.step"
	if e = os.WriteFile(output, b.Bytes(), 0600); e != nil {
		t.Fatal(e)
	}
	stepHoledArea(t, output)
}
