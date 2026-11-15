package trimmed

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/mesh"
)

func TestNativeClippedPolynomialTransverseCutClosedDisplay(t *testing.T) {
	ctx := context.Background()
	options := parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 80_000}
	for axis := range 2 {
		_, roof, _ := nativeWideTwoPolynomialNodeFixture(t, axis)
		for layout := range 2 {
			src := clippedTransverseStock(t, axis, layout, roof)
			for order := range 2 {
				if layout == 0 && order == 0 {
					continue // The native proof refuses the pinch in this order.
				}
				t.Run(fmt.Sprintf("axis%d/layout%d/order%d", axis, layout, order), func(t *testing.T) {
					r, err := CutClippedPolynomialComponents(ctx, src[0], src[1+order], src[2-order], axis, 1e-9, PolynomialNodeCutOptions{})
					if err != nil {
						t.Fatal(err)
					}
					got, err := r.Tessellate(ctx, options)
					if err != nil {
						t.Fatal(err)
					}
					volumes, _ := r.ExactComponentVolumes()
					if len(got) != len(volumes) || len(r.Contacts()) != 2 {
						t.Fatal("display lost a positive component or exact contact", len(got), len(volumes), r.Contacts())
					}
					for i, m := range got {
						original := r.Components()[i].Vertices()
						if len(m.Vertices) < len(original) || !slices.Equal(m.Vertices[:len(original)], original) {
							t.Fatal("display changed the native component vertices", i)
						}
						if err := m.ValidateSolid(); err != nil || m.SignedVolume() <= 0 {
							t.Fatalf("component %d is not outward closed: %g (%v)", i, m.SignedVolume(), err)
						}
						if len(r.Chambers(i)) > 1 {
							want, _ := volumes[i].Float64()
							if math.Abs(m.SignedVolume()-want) > .003*want {
								t.Fatalf("strip-bearing component %d: display volume %.12g, exact %.12g", i, m.SignedVolume(), want)
							}
						}
					}
					if axis == 1 && layout == 1 && order == 1 {
						var raw bytes.Buffer
						if err := r.WriteJSON(ctx, &raw); err != nil {
							t.Fatal(err)
						}
						loaded, err := ReadClippedPolynomialCutJSON(ctx, &raw, JSONLimits{}, PolynomialNodeCutOptions{})
						if err != nil {
							t.Fatal(err)
						}
						reloaded, err := loaded.Tessellate(ctx, options)
						if err != nil || !reflect.DeepEqual(reloaded, got) {
							t.Fatal("source replay changed the transverse component meshes", err)
						}
					}
				})
			}
		}
	}
}

// A finer source-volume match is optional when the baseline approximation
// already meets the geometric constraints and exact component contact proof.
func TestNativeClippedPolynomialTransverseCutDisplayTightBudget(t *testing.T) {
	ctx := context.Background()
	_, roof, _ := nativeWideTwoPolynomialNodeFixture(t, 0)
	src := clippedTransverseStock(t, 0, 1, roof)
	r, e := CutClippedPolynomialComponents(ctx, src[0], src[1], src[2], 0, 1e-9, PolynomialNodeCutOptions{})
	if e != nil {
		t.Fatal(e)
	}
	options := parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 80_000}
	var geometric []mesh.Mesh
	triangles := 0
	for _, part := range r.Components() {
		part = part.withTessellationTrimMemo(&tessellationTrimMemo{limit: tessellationTrimMemoLimit})
		m, e := part.tessellateClippedPolynomial(ctx, 0, options)
		if e != nil {
			t.Fatal(e)
		}
		geometric = append(geometric, m)
		triangles += len(m.Triangles)
	}
	refined, e := r.Tessellate(ctx, options)
	if e != nil {
		t.Fatal(e)
	}
	if len(refined[0].Triangles) <= len(geometric[0].Triangles) || !reflect.DeepEqual(refined[1:], geometric[1:]) {
		t.Fatal("refinement stays on the underresolved strip-bearing component")
	}
	for _, budget := range []int{triangles, triangles + 1} {
		options.MaxTriangles = budget
		m, e := r.Tessellate(ctx, options)
		if e != nil || !reflect.DeepEqual(m, geometric) {
			t.Fatal("hard triangle limits preserve the qualified geometric approximation", budget, e)
		}
	}
}
