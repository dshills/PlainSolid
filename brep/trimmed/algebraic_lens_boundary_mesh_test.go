//go:build research

package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestSharedAlgebraicLensBoundary(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	source := algebraicCapFixture(t, big.NewRat(9, 16))
	before := source.Source.Vertices()
	o := AlgebraicLensBoundaryOptions{Display: geom.ComposedCurvePolylineOptions{Deflection: .001}}
	got, e := source.TessellateSharedBoundaryCertified(ctx, o)
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Edges) != 8 || len(got.TopologyVertices) != 6 {
		t.Fatal("complete arrangement", len(got.Edges), len(got.TopologyVertices))
	}
	counts := map[[2]int]int{}
	orientation := map[[2]int]int{}
	rational, algebraic, irrational := 0, 0, 0
	for _, owner := range got.Owners {
		key := [2]int{owner.Side, owner.Edge}
		counts[key]++
		if owner.Reverse {
			orientation[key]--
		} else {
			orientation[key]++
		}
		path := got.Edges[owner.Edge]
		if len(path) != len(owner.Vertices) {
			t.Fatal("owner sample count")
		}
		for i, id := range owner.Vertices {
			j := i
			if owner.Reverse {
				j = len(path) - 1 - i
			}
			if path[j] != id {
				t.Fatal("post-weld sample replacement")
			}
		}
		if owner.Cap {
			if len(owner.UV) != 0 {
				t.Fatal("cap UV falsely assigned source chart")
			}
			continue
		}
		if len(owner.UV) != len(owner.Vertices) {
			t.Fatal("missing UV provenance")
		}
		for i, uv := range owner.UV {
			p := got.Vertices[owner.Vertices[i]]
			if math.Abs(p.X-uv.Display.X) > 1e-8 || math.Abs(p.Y-uv.Display.Y) > 1e-8 {
				t.Fatal("identity chart owner UV", p, uv.Display)
			}
			for axis, coordinate := range []float64{uv.Display.X, uv.Display.Y} {
				if !finite(coordinate) || uv.Bounds[axis].Lower == nil || uv.Bounds[axis].Upper == nil || uv.Bounds[axis].Lower.Cmp(uv.Bounds[axis].Upper) > 0 {
					t.Fatal("UV enclosure")
				}
			}
			if uv.Root != nil {
				algebraic++
				classification, e := uv.Root.Classify(ctx, geom.ExactCurvePlaneEventOptions{})
				if e != nil {
					t.Fatal(e)
				}
				if classification.Kind == geom.ExactRootKindIrrational {
					irrational++
				}
			} else if uv.Exact[0] != nil && uv.Exact[1] != nil {
				rational++
			} else {
				t.Fatal("missing exact sample identity")
			}
		}
	}
	for key, count := range counts {
		if count != 2 || orientation[key] != 0 {
			t.Fatal("half edge closure", key, count)
		}
	}
	if rational == 0 || algebraic == 0 || irrational == 0 {
		t.Fatal("fixture needs both parameter kinds")
	}
	for ci, path := range got.Cap.CurveVertices {
		global := got.Edges[got.Cap.SourceEdges[ci]]
		for i, local := range path {
			if got.Cap.Mesh.Vertices[local] != got.Vertices[global[i]] {
				t.Fatal("cap network discrepancy")
			}
		}
	}
	for i, v := range source.Source.Vertices() {
		if v != before[i] {
			t.Fatal("source mutated")
		}
	}
	low := o
	low.MaxUVSamples = 1
	if partial, err := source.TessellateSharedBoundaryCertified(ctx, low); err == nil || len(partial.Vertices) != 0 {
		t.Fatal("UV work budget")
	}
	t.Log("shared vertices", len(got.Vertices), "owners", len(got.Owners), "UV samples", got.UVSamples, "rational owner samples", rational, "root-backed owner samples", algebraic, "proved irrational", irrational)
}
