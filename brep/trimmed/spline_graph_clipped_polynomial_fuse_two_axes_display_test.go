package trimmed

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestNativeClippedPolynomialFuseTwoAxesClosedDisplay(t *testing.T) {
	checkClippedFuseDisplayLayouts(t, twoAxisClippedFuseStock)
}

func TestNativeClippedPolynomialFuseTwoAxisContainmentDisplay(t *testing.T) {
	ctx := context.Background()
	for axis := range 2 {
		for order := range 2 {
			t.Run(fmt.Sprintf("axis%d/order%d", axis, order), func(t *testing.T) {
				t.Parallel()
				src := clippedPolynomialStock(t, axis, 0, 1)
				src[2] = restrictOriginalClipStock(t, src[2], geom.ParameterRectangle{
					UMin: 2.375, UMax: 4.625, VMin: -.75, VMax: .75,
				})
				fuse, err := FuseClippedPolynomialEnvelope(ctx, src[0], src[1+order], src[2-order], axis, 1e-9,
					PolynomialCubicCrossingTangencyEnvelopeOptions{})
				if err != nil {
					t.Fatal(err)
				}
				checkClippedFuseDisplay(t, fuse)
			})
		}
	}
}

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
