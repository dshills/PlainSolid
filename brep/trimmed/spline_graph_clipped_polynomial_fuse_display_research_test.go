//go:build research

package trimmed

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
)

func TestNativeClippedPolynomialFuseDisplayLimitsAndReload(t *testing.T) {
	ctx := context.Background()
	src := clippedPolynomialStock(t, 0, 0, 0)
	fuseOpts := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	r, err := FuseClippedPolynomialEnvelope(ctx, src[0], src[1], src[2], 0, 1e-9, fuseOpts)
	if err != nil {
		t.Fatal(err)
	}
	boundary := r.Boundary()
	opts := parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 80_000}
	m, err := boundary.Tessellate(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	strictAngle := opts
	strictAngle.Angle = .075
	fine, err := boundary.Tessellate(ctx, strictAngle)
	if err != nil || len(fine.Triangles) <= len(m.Triangles) {
		t.Fatalf("a tighter normal angle must refine the curved roofs: %v, %d vs %d", err, len(fine.Triangles), len(m.Triangles))
	}

	for _, limit := range []parametric.TessellationOptions{
		{Deflection: opts.Deflection, Angle: opts.Angle, MaxSubdivisions: 2, MaxTriangles: opts.MaxTriangles},
		{Deflection: opts.Deflection, Angle: opts.Angle, MaxSubdivisions: opts.MaxSubdivisions, MaxTriangles: len(m.Triangles) - 1},
	} {
		if partial, err := boundary.Tessellate(ctx, limit); !errors.Is(err, parametric.ErrBudget) || len(partial.Vertices)+len(partial.Triangles) != 0 {
			t.Fatalf("display exceeded the caller's limits: %+v, %v", limit, err)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := boundary.Tessellate(cancelled, opts); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled display: %v", err)
	}
	analytic := opts
	analytic.AnalyticBound = true
	if _, err := boundary.Tessellate(ctx, analytic); !errors.Is(err, parametric.ErrDeflectionUnsupported) {
		t.Fatalf("sampled display must require ordinary deflection mode: %v", err)
	}

	var saved bytes.Buffer
	if err := r.WriteJSON(ctx, &saved); err != nil {
		t.Fatal(err)
	}
	loaded, err := ReadClippedPolynomialFuseJSON(ctx, bytes.NewReader(saved.Bytes()), JSONLimits{}, fuseOpts)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := loaded.Boundary().Tessellate(ctx, opts)
	if err != nil || !reflect.DeepEqual(m, replayed) {
		t.Fatalf("reloading the original-source receipt changed the display: %v", err)
	}
}
