package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/geom"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestClosedIndependentlyCurvedLensHalves(t *testing.T) {
	slowtest.Skip(t)
	source := algebraicCurvedCapFixture(t, big.NewRat(9, 16), 1)
	o := ClosedAlgebraicLensMeshOptions{Boundary: AlgebraicLensBoundaryOptions{Display: geom.ComposedCurvePolylineOptions{Deflection: .001}}, Interior: CurvedQuadraticTrimMeshOptions{Deflection: .01}}
	got, e := source.TessellateClosedHalvesCertified(context.Background(), o)
	if e != nil {
		t.Fatal(e)
	}
	total := 0.
	for _, half := range []CertifiedClosedLensHalf{got.Negative, got.Positive} {
		v, _ := half.SignedDisplayVolume.Float64()
		total += v
		if !half.Collision.Complete || len(half.Collision.Contacts) != 0 {
			t.Fatal("display contact")
		}
		if half.FaceResidualSquared[0].Sign() == 0 || half.FaceResidualSquared[1].Sign() == 0 {
			t.Fatal("both support graphs must curve")
		}
		if e := half.Mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		for id, network := range got.Boundary.Vertices {
			if local, ok := half.NetworkVertices[id]; ok && half.Mesh.Vertices[local] != network {
				t.Fatal("shared boundary changed")
			}
		}
		t.Log("side", half.Side, "triangles", len(half.Mesh.Triangles), "volume", v)
	}
	want := math.Pi / (512 * math.Sqrt2)
	if math.Abs(total-want) > .1*want {
		t.Fatal("common graph bend must preserve lens volume", total, want)
	}
}
