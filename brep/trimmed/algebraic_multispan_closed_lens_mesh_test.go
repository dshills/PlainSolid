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

func TestClosedMultiSpanCurvedLensHalves(t *testing.T) {
	slowtest.Skip(t)
	for _, crease := range []bool{false, true} {
		source := algebraicCapSpansFixture(t, big.NewRat(9, 16), 1, true, crease)
		o := MultiSpanClosedLensMeshOptions{Closed: ClosedAlgebraicLensMeshOptions{Boundary: AlgebraicLensBoundaryOptions{Display: geom.ComposedCurvePolylineOptions{Deflection: .001}}, Interior: CurvedQuadraticTrimMeshOptions{Deflection: .01}}}
		got, e := source.TessellateMultiSpanClosedHalvesCertified(context.Background(), o)
		if e != nil {
			t.Fatal("crease", crease, e)
		}
		if got.SpanCellChecks < 16 || got.SpanPieceChecks == 0 {
			t.Fatal("no knot proof")
		}
		total := 0.
		for _, half := range []CertifiedClosedLensHalf{got.Negative, got.Positive} {
			if e := half.Mesh.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
			if !half.Collision.Complete || len(half.Collision.Contacts) != 0 {
				t.Fatal("contacts")
			}
			v, _ := half.SignedDisplayVolume.Float64()
			total += v
			for id, index := range half.NetworkVertices {
				if half.Mesh.Vertices[index] != got.Boundary.Vertices[id] {
					t.Fatal("boundary moved")
				}
			}
		}
		want := math.Pi / (512 * math.Sqrt2)
		if math.Abs(total-want) > .1*want {
			t.Fatal("crease/graph bend changes gap volume", total, want)
		}
		low := o
		low.MaxSpanCells = 1
		if partial, err := source.TessellateMultiSpanClosedHalvesCertified(context.Background(), low); err == nil || len(partial.Negative.Mesh.Triangles) != 0 {
			t.Fatal("span budget")
		}
		t.Log("crease", crease, "triangles", len(got.Negative.Mesh.Triangles), len(got.Positive.Mesh.Triangles), "cells", got.SpanCellChecks, "pieces", got.SpanPieceChecks, "volume", total)
	}
}
