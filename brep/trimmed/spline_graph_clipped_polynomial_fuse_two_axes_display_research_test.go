//go:build research

package trimmed

import (
	"bytes"
	"context"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
)

func TestNativeClippedPolynomialFuseTwoAxesDisplayReload(t *testing.T) {
	ctx := context.Background()
	src := twoAxisClippedFuseStock(t, 1, 1, 1)
	fuseOpts := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	r, err := FuseClippedPolynomialEnvelope(ctx, src[0], src[2], src[1], 1, 1e-9, fuseOpts)
	if err != nil {
		t.Fatal(err)
	}
	var raw bytes.Buffer
	if err := r.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ReadClippedPolynomialFuseJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, fuseOpts)
	if err != nil {
		t.Fatal(err)
	}
	opts := parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 80_000}
	original, err := r.Boundary().Tessellate(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := reloaded.Boundary().Tessellate(ctx, opts)
	if err != nil || !reflect.DeepEqual(original, replayed) {
		t.Fatalf("two-axis source replay changed the display: %v", err)
	}
}
