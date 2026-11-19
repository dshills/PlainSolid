//go:build research

package trimmed

import (
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/csg/poly"
	"plainsolid/geom"
)

func TestAffineSplineShellExactObliqueSplit(t *testing.T) {
	ctx := context.Background()
	s, _ := makeOpenSplineBox(t, [3]float64{}, [3]float64{1, 1, 1}, 0)
	o := AffineSplineShellBooleanOptions{}
	a, b, h, e := SplitAffineSplineShellPlane(ctx, s, geom.Vec3{Z: 1}, geom.Vec3{X: 3, Z: 3}, o)
	if e != nil {
		t.Fatal(e)
	}
	for i, piece := range []struct {
		s        Solid
		h        AffineSplinePlaneSplitSide
		centroid geom.Vec3
	}{{a, h.Negative, geom.Vec3{X: 1.0 / 3, Y: .5, Z: 1.0 / 3}},
		{b, h.Positive, geom.Vec3{X: 2.0 / 3, Y: .5, Z: 2.0 / 3}}} {
		if piece.s.Topology().EulerCharacteristic != 2 || len(piece.h.CapFaces) == 0 {
			t.Fatal(i, piece.s.Topology(), piece.h.CapFaces)
		}
		p, e := piece.s.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(p.Volume-.5) > 1e-12 || p.Centroid.Distance(piece.centroid) > 1e-12 {
			t.Fatal(i, p, e)
		}
		for _, j := range piece.h.CapFaces {
			f := piece.s.faces[j]
			for _, u := range f.Outer {
				v := piece.s.vertices[piece.s.edges[u.Edge].Start]
				if v.X+v.Z != 1 {
					t.Fatal("cap not on represented cutter", v)
				}
			}
		}
		retained := 0
		for original, fragments := range piece.h.Faces {
			for _, j := range fragments {
				if piece.s.faces[j].Surface != s.faces[original].Surface {
					t.Fatal("original support replaced", original, j)
				}
				retained++
			}
		}
		if retained+len(piece.h.CapFaces) != len(piece.s.faces) || len(piece.h.PlanarEdges) != len(piece.s.edges) {
			t.Fatal("incomplete ancestry", i, retained)
		}
		for j, ancestry := range piece.h.PlanarEdges {
			for _, overlap := range ancestry.Overlaps {
				found := false
				for _, edge := range piece.h.Edges[overlap.Edge] {
					found = found || edge == j
				}
				if !found {
					t.Fatal("missing exact edge overlap", overlap, j)
				}
			}
		}
		if _, e := PlanarBoundaryFromAffineSplines(ctx, piece.s, o); e != nil {
			t.Fatal("split output could not be reused as affine operand", e)
		}
	}
	again, empty, h, e := SplitAffineSplineShellPlane(ctx, a, geom.Vec3{Z: 1}, geom.Vec3{X: 1, Z: 1}, o)
	if e != nil || len(again.faces) != len(a.faces) || len(empty.faces) != 0 || len(h.Negative.CapFaces) != 0 {
		t.Fatal("tangent plane should retain original input", e)
	}
	rejoined, ancestry, e := CombineSplineShells(ctx, a, b, parametric.Union, SplineShellCombineOptions{})
	if e != nil || ancestry.Method != "affine-planar-arrangement" {
		t.Fatal("dispatch did not reuse nonrectangular pieces", ancestry, e)
	}
	props, e := rejoined.MassProperties(ctx, parametric.IntegrationOptions{})
	if e != nil || math.Abs(props.Volume-1) > 1e-12 {
		t.Fatal(props, e)
	}
	// A cut through x=1/3 cannot be reconstructed as exact float geometry.
	a, b, _, e = SplitAffineSplineShellPlane(ctx, s, geom.Vec3{Z: 1}, geom.Vec3{X: 3, Z: 1}, o)
	if !errors.Is(e, poly.ErrPrecision) || len(a.faces) != 0 || len(b.faces) != 0 {
		t.Fatal("nonrepresentable split returned geometry", e)
	}
}
