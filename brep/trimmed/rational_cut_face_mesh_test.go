//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func rationalCutFaceFixture(t *testing.T) (*geom.SurfacePatch, []SplineUV) {
	p, curve := rationalCutTrimFixture(t)
	outer := []SplineUV{curve}
	corners := []geom.Vec3{{X: 1, Y: .125}, {X: 1, Y: .75}, {Y: .75}, {Y: .125}}
	for i := 0; i+1 < len(corners); i++ {
		c, e := geom.NewBSpline(1, []geom.Vec3{corners[i], corners[i+1]}, []float64{0, 0, 1, 1}, nil)
		if e != nil {
			t.Fatal(e)
		}
		outer = append(outer, SplineUV{Curve: c, Range: UVInterval{Max: 1}})
	}
	return p, outer
}
func TestRationalCutCurvedFaceFill(t *testing.T) {
	p, outer := rationalCutFaceFixture(t)
	cuts := make([][]*big.Rat, len(outer))
	cuts[0] = []*big.Rat{big.NewRat(1, 3)}
	o := RationalCutFaceMeshOptions{Face: MultiSpanCurvedTrimMeshOptions{Mesh: CurvedQuadraticTrimMeshOptions{Deflection: .15}}}
	got, e := TessellateRationalCutFaceCertified(context.Background(), p, outer, cuts, o)
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, q := range got.Trims[0].NormalizedParameters {
		if q.Cmp(big.NewRat(1, 3)) == 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("cut omitted")
	}
	if got.FillSquaredDistance.Cmp(new(big.Rat).Mul(ratInset(.15), ratInset(.15))) > 0 {
		t.Fatal("distance")
	}
	area := new(big.Rat)
	for _, tri := range got.Mesh.Triangles {
		a, b, c := got.ExactUV[tri[0]], got.ExactUV[tri[1]], got.ExactUV[tri[2]]
		det := new(big.Rat).Sub(new(big.Rat).Mul(new(big.Rat).Sub(b[0], a[0]), new(big.Rat).Sub(c[1], a[1])), new(big.Rat).Mul(new(big.Rat).Sub(b[1], a[1]), new(big.Rat).Sub(c[0], a[0])))
		if det.Sign() <= 0 {
			t.Fatal("orientation")
		}
		area.Add(area, new(big.Rat).Quo(det, big.NewRat(2, 1)))
	}
	if area.Cmp(got.Boundary.Region.PolygonUVArea) != 0 {
		t.Fatal("exact fill coverage")
	}
	t.Logf("triangles=%d pieces=%d cells=%d", len(got.Mesh.Triangles), len(got.Trims[0].SquaredErrors), got.SpanCellChecks)
}
