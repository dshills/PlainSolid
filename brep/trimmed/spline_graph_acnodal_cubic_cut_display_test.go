//go:build research

package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeAcnodalCubicCutVolumeQualifiedDisplay(t *testing.T) {
	ctx := context.Background()
	source := acnodalCubicStockFixture(t, true)
	o := AcnodalCubicCutOptions{}
	r, err := CutAcnodalCubicSplineGraphs(ctx, source[0], source[2], source[1], 1, 1e-9, o)
	if err != nil {
		t.Fatal(err)
	}
	meshing := parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxTriangles: 1024, MaxSubdivisions: 64}
	const volume = 4703.0 / 1614807040
	old, err := r.Boundary().Tessellate(ctx, meshing)
	if err != nil || math.Abs(old.SignedVolume()-volume) <= .1*volume {
		t.Fatal("the coarse control must expose the missing material", old.SignedVolume(), err)
	}
	m, err := r.Tessellate(ctx, meshing)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.ValidateSolid(); err != nil || math.Abs(m.SignedVolume()-volume) > .02*volume {
		t.Fatal("unqualified preview closed or exceeded source volume budget", m.SignedVolume(), err)
	}
	for _, original := range r.Boundary().Vertices() {
		if !slices.Contains(m.Vertices, original) {
			t.Fatal("refinement dropped an exact original trim vertex", original)
		}
	}
	t.Logf("%d triangles, %.6f%% error against the independent source volume", len(m.Triangles), 100*math.Abs(m.SignedVolume()-volume)/volume)
	var receipt bytes.Buffer
	if err := r.WriteJSON(ctx, &receipt); err != nil {
		t.Fatal(err)
	}
	copy, err := ReadAcnodalCubicCutJSON(ctx, &receipt, JSONLimits{}, o)
	if err != nil {
		t.Fatal(err)
	}
	again, err := copy.Tessellate(ctx, meshing)
	if err != nil || !slices.Equal(again.Vertices, m.Vertices) || !slices.Equal(again.Triangles, m.Triangles) {
		t.Fatal("native replay changed certified preview", err)
	}

	limited := meshing
	limited.MaxTriangles = len(old.Triangles)
	empty, err := r.Tessellate(ctx, limited)
	if !errors.Is(err, parametric.ErrBudget) || len(empty.Vertices) != 0 || len(empty.Triangles) != 0 {
		t.Fatal("exhausted preview published partial geometry", err)
	}
	empty, err = r.TessellateWithVolumeTolerance(ctx, meshing, math.NaN())
	if !errors.Is(err, geom.ErrInvalid) || len(empty.Triangles) != 0 {
		t.Fatal("invalid volume bound was accepted", err)
	}
}
