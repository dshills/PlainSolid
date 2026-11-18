package trimmed

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/geom"
)

func TestNativeClippedPolynomialThreeTransverseCutVolumeDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	options := parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 80_000}
	const tolerance = .02
	for axis := range 2 {
		_, a, _ := nativeThreePolynomialNodeFixture(t, axis, 4, 128)
		for layout := range 3 {
			s := clippedTransverseFuseStock(t, axis, layout, a)
			for order := range 2 {
				if order == 0 && layout != 1 {
					continue
				}
				t.Run(fmt.Sprintf("axis%d/layout%d/order%d", axis, layout, order), func(t *testing.T) {
					r, e := CutClippedPolynomialThreeTransverseComponents(ctx, s[0], s[1+order], s[2-order], axis, 1e-9, PolynomialNodeCutOptions{})
					if e != nil {
						t.Fatal(e)
					}
					display, e := r.TessellateWithVolumeTolerance(ctx, options, tolerance)
					if e != nil {
						t.Fatal(e)
					}
					forward, reverse := threeTransverseClippedComponentVolumes(layout, axis)
					want := [2][]string{forward, reverse}[order]
					if len(display) != len(want) {
						t.Fatal("volume-qualified display loses original components", len(display), len(want))
					}
					total := 0
					for i, m := range display {
						vertices := r.Components()[i].Vertices()
						if len(m.Vertices) < len(vertices) || !slices.Equal(m.Vertices[:len(vertices)], vertices) {
							t.Fatal("volume refinement changes original vertex indices", i)
						}
						exact, _ := new(big.Rat).SetString(want[i])
						volume, _ := exact.Float64()
						if err := math.Abs(m.SignedVolume()-volume) / volume; err > tolerance {
							t.Fatal("component fails independent source volume", i, m.SignedVolume(), volume, err)
						}
						total += len(m.Triangles)
					}
					if total > options.MaxTriangles {
						t.Fatal("assembly exceeds the triangle budget", total)
					}
					t.Logf("%d volume-qualified meshes, %d triangles", len(display), total)
					if layout == 1 && order == 0 {
						// Both exterior shells share the three original point contacts.
						var declarations []collision.AssemblyVertexContact
						for _, c := range r.Contacts() {
							declarations = append(declarations, collision.AssemblyVertexContact{
								A: collision.AssemblyVertex{Solid: c.Components[0], Vertex: c.Vertices[0]},
								B: collision.AssemblyVertex{Solid: c.Components[1], Vertex: c.Vertices[1]},
							})
						}
						report, err := collision.ValidateMeshVertexContactAssembly(ctx, display, declarations, collision.VertexContactAssemblyOptions{})
						if err != nil || len(report.Contacts) != 3 {
							t.Fatal("refinement loses exact component contacts", report, err)
						}
					}
					if axis == 0 && layout == 0 {
						// Read the exact planar volume using the independent locator.
						// This component is approximately 7.7e-10 cubic model units.
						thin := display[1]
						loc, e := collision.NewMeshPointLocator(ctx, thin, nil, collision.LocationOptions{})
						if e != nil {
							t.Fatal(e)
						}
						exact, _ := new(big.Rat).SetString(want[1])
						meshVolume := loc.Shells()[0].SignedVolume
						difference := new(big.Rat).Abs(new(big.Rat).Sub(meshVolume, exact))
						limit := new(big.Rat).Mul(exact, new(big.Rat).SetFloat64(tolerance))
						if difference.Cmp(limit) > 0 {
							t.Fatal("exact mesh construction fails the rational relative bound", difference, limit)
						}
						var recipe bytes.Buffer
						if e := r.WriteJSON(ctx, &recipe); e != nil {
							t.Fatal(e)
						}
						reloaded, e := ReadClippedPolynomialThreeTransverseCutJSON(ctx, &recipe, JSONLimits{}, PolynomialNodeCutOptions{})
						if e != nil {
							t.Fatal(e)
						}
						out, e := reloaded.TessellateWithVolumeTolerance(ctx, options, tolerance)
						if e != nil || !reflect.DeepEqual(out, display) {
							t.Fatal("exact volume constraint changes across original source replay", e)
						}
					}
				})
			}
		}
	}
}

func TestNativeClippedPolynomialThreeTransverseCutVolumeDisplayLimits(t *testing.T) {
	ctx := context.Background()
	_, a, _ := nativeThreePolynomialNodeFixture(t, 0, 4, 128)
	s := clippedTransverseFuseStock(t, 0, 0, a)
	r, e := CutClippedPolynomialThreeTransverseComponents(ctx, s[0], s[2], s[1], 0, 1e-9, PolynomialNodeCutOptions{})
	if e != nil {
		t.Fatal(e)
	}
	o := parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 80_000}
	for _, relative := range []float64{0, -.02, math.NaN(), math.Inf(1)} {
		if m, e := r.TessellateWithVolumeTolerance(ctx, o, relative); m != nil || !errors.Is(e, geom.ErrInvalid) {
			t.Fatal("invalid relative accuracy publishes a mesh", relative, e)
		}
	}
	limited := o
	limited.MaxSubdivisions = 4
	if _, e := r.Tessellate(ctx, limited); e != nil {
		t.Fatal("coarse geometric display should qualify", e)
	}
	if m, e := r.TessellateWithVolumeTolerance(ctx, limited, .02); m != nil || !errors.Is(e, parametric.ErrBudget) {
		t.Fatal("insufficient subdivision budget publishes a mesh", e)
	}
	limited = o
	limited.MaxTriangles = 250
	if _, e := r.Tessellate(ctx, limited); e != nil {
		t.Fatal("coarse geometric display fits the triangle limit", e)
	}
	if m, e := r.TessellateWithVolumeTolerance(ctx, limited, .02); m != nil || !errors.Is(e, parametric.ErrBudget) {
		t.Fatal("insufficient assembly triangle budget publishes a mesh", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if m, e := r.TessellateWithVolumeTolerance(cancelled, o, .02); m != nil || !errors.Is(e, context.Canceled) {
		t.Fatal("cancelled volume refinement publishes a mesh", e)
	}
}
