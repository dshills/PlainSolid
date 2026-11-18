package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"reflect"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/collision"
	"plainsolid/mesh"
)

func TestNativeClippedPolynomialThreeTransverseCutClosedDisplay(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	native := PolynomialNodeCutOptions{}
	options := parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 80_000}
	for axis := range 2 {
		_, a, _ := nativeThreePolynomialNodeFixture(t, axis, 4, 128)
		for layout := range 3 {
			s := clippedTransverseFuseStock(t, axis, layout, a)
			for order := range 2 {
				if order == 0 && layout != 1 {
					continue
				}
				r, e := CutClippedPolynomialThreeTransverseComponents(ctx, s[0], s[1+order], s[2-order], axis, 1e-9, native)
				if e != nil {
					t.Fatal(axis, layout, order, e)
				}
				display, e := r.Tessellate(ctx, options)
				if e != nil {
					t.Fatal(axis, layout, order, e)
				}
				a, b := threeTransverseClippedComponentVolumes(layout, axis)
				checkThreeTransverseCutDisplay(t, r, display, [2][]string{a, b}[order], options.MaxTriangles)
				var raw bytes.Buffer
				if e := r.WriteJSON(ctx, &raw); e != nil {
					t.Fatal(e)
				}
				loaded, e := ReadClippedPolynomialThreeTransverseCutJSON(ctx, &raw, JSONLimits{}, native)
				if e != nil {
					t.Fatal(e)
				}
				replayed, e := loaded.Tessellate(ctx, options)
				if e != nil || !reflect.DeepEqual(display, replayed) {
					t.Fatal("original source replay preserves each preview mesh", axis, layout, order, e)
				}
			}
		}
	}
}

func checkThreeTransverseCutDisplay(t *testing.T, r *ClippedPolynomialThreeTransverseCut, display []mesh.Mesh, want []string, limit int) {
	t.Helper()
	parts := r.Components()
	if len(display) != len(parts) || len(display) != len(want) || len(r.Contacts()) != 3 {
		t.Fatal("all original components and point contacts survive", len(display), len(parts), len(want))
	}
	triangles := 0
	for i, m := range display {
		triangles += len(m.Triangles)
		native := parts[i].Vertices()
		if len(m.Vertices) < len(native) || !slices.Equal(m.Vertices[:len(native)], native) {
			t.Fatal("native component vertex indices change", i)
		}
		used := make([]bool, len(m.Vertices))
		for _, x := range m.Triangles {
			for _, id := range x {
				used[id] = true
			}
		}
		for id := range native {
			if !used[id] {
				t.Fatal("an original model vertex is unused", i, id)
			}
		}
		if e := m.ValidateSolid(); e != nil || m.SignedVolume() <= 0 {
			t.Fatal("every preview component is independently outward and closed", i, e, m.SignedVolume())
		}
		chambers := r.Chambers(i)
		if chambers[len(chambers)-1].Tile != 0 {
			exact, _ := new(big.Rat).SetString(want[i])
			v, _ := exact.Float64()
			if math.Abs(m.SignedVolume()-v) > .003*v {
				t.Fatal("display of an original stock strip agrees with its independent volume", i, m.SignedVolume(), v)
			}
		}
	}
	if triangles > limit {
		t.Fatal("triangle budget applies to the complete assembly", triangles, limit)
	}
	var declared []collision.AssemblyVertexContact
	for _, c := range r.Contacts() {
		declared = append(declared, collision.AssemblyVertexContact{
			A: collision.AssemblyVertex{Solid: c.Components[0], Vertex: c.Vertices[0]},
			B: collision.AssemblyVertex{Solid: c.Components[1], Vertex: c.Vertices[1]},
		})
	}
	report, e := collision.ValidateMeshVertexContactAssembly(context.Background(), display, declared,
		collision.VertexContactAssemblyOptions{Validation: collision.LocationOptions{MaxTriangles: limit}})
	if e != nil || report.SignedVolume.Sign() <= 0 || len(report.Contacts) != len(declared) {
		t.Fatal("the entire display proves exactly the declared original point contacts", report, e)
	}
	for _, contact := range declared {
		if !slices.Contains(report.Contacts, contact) {
			t.Fatal("original point contact lost its component-local vertex IDs", contact)
		}
	}
}

func TestNativeClippedPolynomialThreeTransverseCutDisplayLimits(t *testing.T) {
	ctx := context.Background()
	_, a, _ := nativeThreePolynomialNodeFixture(t, 0, 4, 128)
	s := clippedTransverseFuseStock(t, 0, 0, a)
	r, e := CutClippedPolynomialThreeTransverseComponents(ctx, s[0], s[2], s[1], 0, 1e-9, PolynomialNodeCutOptions{})
	if e != nil {
		t.Fatal(e)
	}
	options := parametric.TessellationOptions{Deflection: .05, Angle: .15, MaxSubdivisions: 32, MaxTriangles: 80_000}
	parts, e := r.Tessellate(ctx, options)
	if e != nil {
		t.Fatal(e)
	}
	total := 0
	for _, m := range parts {
		total += len(m.Triangles)
	}
	limited := options
	limited.MaxTriangles = total - 1
	if out, e := r.Tessellate(ctx, limited); out != nil || !errors.Is(e, parametric.ErrBudget) {
		t.Fatal("the total display triangle budget is atomic", e)
	}
	analytic := options
	analytic.AnalyticBound = true
	if out, e := r.Tessellate(ctx, analytic); out != nil || !errors.Is(e, parametric.ErrDeflectionUnsupported) {
		t.Fatal("three-contact Cut preview uses sampled deflection", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if out, e := r.Tessellate(cancelled, options); out != nil || !errors.Is(e, context.Canceled) {
		t.Fatal("cancelled display publishes no components", e)
	}
	var incomplete []collision.AssemblyVertexContact
	for _, c := range r.Contacts()[:2] {
		incomplete = append(incomplete, collision.AssemblyVertexContact{
			A: collision.AssemblyVertex{Solid: c.Components[0], Vertex: c.Vertices[0]},
			B: collision.AssemblyVertex{Solid: c.Components[1], Vertex: c.Vertices[1]},
		})
	}
	if _, e := collision.ValidateMeshVertexContactAssembly(ctx, parts, incomplete, collision.VertexContactAssemblyOptions{}); e == nil {
		t.Fatal("the exact assembly must account for the third native point contact")
	}
}
