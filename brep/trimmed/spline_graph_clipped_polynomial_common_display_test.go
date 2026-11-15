package trimmed

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
)

func TestNativeClippedPolynomialCommonClosedDisplay(t *testing.T) {
	ctx := context.Background()
	// Physical overlap in the offset layouts has the same independent
	// 3/4-square integral, whether its projected axes or roof order swap.
	offset, _ := new(big.Rat).SetString("370046848335928793529/41216943789694779392")
	offsetVolume, _ := offset.Float64()
	for layout := range 4 {
		for axis := range 2 {
			for profile := range 2 {
				for order := range 2 {
					t.Run(fmt.Sprintf("layout%d/axis%d/profile%d/order%d", layout, axis, profile, order), func(t *testing.T) {
						t.Parallel()
						src := clippedPolynomialStock(t, axis, profile, layout%2)
						_, want := clippedCommonMeasures(axis, profile)
						if layout >= 2 {
							src = twoAxisClippedFuseStock(t, axis, profile, layout-2)
							want = offsetVolume
						}
						r, err := CommonClippedPolynomialEnvelope(ctx, src[0], src[1+order], src[2-order], axis, 1e-9,
							PolynomialCubicCrossingTangencyEnvelopeOptions{})
						if err != nil {
							t.Fatal(err)
						}
						checkClippedShellDisplay(t, r.Boundary(), want)
					})
				}
			}
		}
	}
}

func TestNativeClippedPolynomialCommonDisplayLimitsAndReload(t *testing.T) {
	ctx := context.Background()
	src := twoAxisClippedFuseStock(t, 1, 1, 1)
	sourceOpts := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	r, err := CommonClippedPolynomialEnvelope(ctx, src[0], src[2], src[1], 1, 1e-9, sourceOpts)
	if err != nil {
		t.Fatal(err)
	}
	boundary := r.Boundary()
	opts := parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 80_000}
	m, err := boundary.Tessellate(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, limit := range []parametric.TessellationOptions{
		{Deflection: opts.Deflection, Angle: opts.Angle, MaxSubdivisions: 32, MaxTriangles: len(m.Triangles) - 1},
		{Deflection: 1e-8, Angle: opts.Angle, MaxSubdivisions: 1, MaxTriangles: 80_000},
	} {
		if partial, err := boundary.Tessellate(ctx, limit); !errors.Is(err, parametric.ErrBudget) || len(partial.Vertices)+len(partial.Triangles) != 0 {
			t.Fatalf("Common exhausted the display budget: %+v, %v", limit, err)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := boundary.Tessellate(cancelled, opts); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled Common display: %v", err)
	}
	analytic := opts
	analytic.AnalyticBound = true
	if _, err := boundary.Tessellate(ctx, analytic); !errors.Is(err, parametric.ErrDeflectionUnsupported) {
		t.Fatalf("Common display requires sampled deflection: %v", err)
	}
	var record bytes.Buffer
	if err := r.WriteJSON(ctx, &record); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ReadClippedPolynomialCommonJSON(ctx, bytes.NewReader(record.Bytes()), JSONLimits{}, sourceOpts)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := reloaded.Boundary().Tessellate(ctx, opts)
	if err != nil || !reflect.DeepEqual(m, replayed) {
		t.Fatalf("reloading the original-source Common recipe changed the display: %v", err)
	}
}
