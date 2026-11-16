package trimmed

import (
	"bytes"
	"context"
	"math/big"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
)

func TestNativeClippedPolynomialThreeTransverseCommonClosedDisplay(t *testing.T) {
	ctx := context.Background()
	o := PolynomialCubicCrossingTangencyEnvelopeOptions{}
	options := parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 80_000}
	volumes := [3]string{"26955626929285653/2251799813685248", "6580961624997/549755813888", "20216720162646549/2251799813685248"}
	for axis := range 2 {
		_, a, _ := nativeThreePolynomialNodeFixture(t, axis, 4, 128)
		for layout := range 3 {
			s := clippedTransverseFuseStock(t, axis, layout, a)
			exact, _ := new(big.Rat).SetString(volumes[layout])
			want, _ := exact.Float64()
			for order := range 2 {
				r, e := CommonClippedPolynomialThreeTransverseEnvelope(ctx, s[0], s[1+order], s[2-order], axis, 1e-9, o)
				if e != nil {
					t.Fatal(axis, layout, order, e)
				}
				checkClippedShellDisplay(t, r.Boundary(), want)
				if axis == 1 && layout == 2 && order == 1 {
					m, e := r.Boundary().Tessellate(ctx, options)
					if e != nil {
						t.Fatal(e)
					}
					var raw bytes.Buffer
					if e := r.WriteJSON(ctx, &raw); e != nil {
						t.Fatal(e)
					}
					loaded, e := ReadClippedPolynomialThreeTransverseCommonJSON(ctx, &raw, JSONLimits{}, o)
					if e != nil {
						t.Fatal(e)
					}
					replayed, e := loaded.Boundary().Tessellate(ctx, options)
					if e != nil || !reflect.DeepEqual(m, replayed) {
						t.Fatal("the original source recipe reproduces the default Common mesh", e)
					}
				}
			}
		}
	}
}
