//go:build research

package step

import (
	"bytes"
	"context"
	"io"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"
)

func TestWrappedRationalStationAssemblyRoundTrip(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	raw := weightedAssemblyFixture(t)
	m, e := ReadWrappedRationalStationAssembly(ctx, bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = ReadWrappedRationalStationLoft(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("standalone admitted assembly")
	}
	if _, e = ReadWrappedMaterialAssembly(bytes.NewReader(raw), Limits{}, WrappedSurfaceOptions{}); e == nil {
		t.Fatal("old mixed route admitted weighted family")
	}
	if m.Product.DefinitionID != 31 || m.SourceBodyID != 37 || !reflect.DeepEqual(m.SourceWallFaceIDs, []int64{39, 174}) {
		t.Fatal(m)
	}
	ref := rationalStationSTEPFixture(t, 2)
	for i := range m.Body.Walls {
		a, _ := m.Body.Walls[i].Geometry.ExportRecipe()
		b, _ := ref.Walls[i].Geometry.ExportRecipe()
		if !reflect.DeepEqual(a.Surface.Data(), b.Surface.Data()) {
			t.Fatal("original support changed")
		}
	}
	var source, native, out bytes.Buffer
	if e = WriteWrappedRationalStationAssemblySource(ctx, &source, m.Assembly.Source, Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
	re, e := ReadWrappedRationalStationAssembly(ctx, &source, Limits{}, WrappedSurfaceOptions{})
	if e != nil || !reflect.DeepEqual(m.SourceWallFaceIDs, re.SourceWallFaceIDs) || !reflect.DeepEqual(m.Occurrences, re.Occurrences) {
		t.Fatal(e)
	}
	if e = trimmed.WriteJSON(&native, m.Body.Solid); e != nil {
		t.Fatal(e)
	}
	solid, e := trimmed.ReadJSONContext(ctx, &native, trimmed.JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	body, e := WrappedRationalStationLoftFromSolid(ctx, solid, m.Body.Name, m.Body.LengthUnit, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	body.CapNames = m.Body.CapNames
	m.Body = body
	if e = WriteWrappedRationalStationAssembly(ctx, &out, weightedAssemblyModel(m), Limits{}, WrappedSurfaceOptions{}); e != nil {
		t.Fatal(e)
	}
	n, e := ReadWrappedRationalStationAssembly(ctx, bytes.NewReader(out.Bytes()), Limits{}, WrappedSurfaceOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i := range m.Body.Walls {
		a, _ := m.Body.Walls[i].Geometry.ExportRecipe()
		b, _ := n.Body.Walls[i].Geometry.ExportRecipe()
		if !reflect.DeepEqual(a.Surface.Data(), b.Surface.Data()) {
			t.Fatal("export flattened or replaced support")
		}
	}
	if len(n.Assembly.Products) != 2 || len(n.Occurrences) != 2 || n.Occurrences[0].ChildID != n.Occurrences[1].ChildID {
		t.Fatal("lost shared asset")
	}
	// Caches are not writer authority; source supports still reconstruct the body.
	bad := weightedAssemblyModel(m)
	bad.Products[1].Body = &WrappedRationalStationLoftBody{LengthUnit: m.Body.LengthUnit}
	var buf bytes.Buffer
	if e = WriteWrappedRationalStationAssembly(ctx, &buf, bad, Limits{}, WrappedSurfaceOptions{}); e == nil || buf.Len() != 0 {
		t.Fatal("partial invalid write")
	}
	if e = WriteWrappedRationalStationAssembly(ctx, occurrenceShortWriter{}, weightedAssemblyModel(m), Limits{}, WrappedSurfaceOptions{}); e != io.ErrShortWrite {
		t.Fatal(e)
	}
}
