package trimmed

import (
	"context"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestMappedRoofNormalConeUsesOriginalOneSidedDifferentials(t *testing.T) {
	ctx := context.Background()
	p, err := geom.NewSurfacePatch(mappedChainCreasedSupport(t, false, .5, .5), geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// Adjacent triangles share the physical crease edge. Their opposite
	// vertices sample different original continuations of the same position.
	var face CertifiedCurvedQuadraticTrimMesh
	for _, uv := range [][2]float64{{.5, .25}, {.5, .265625}, {.484375, .25}, {.515625, .25}} {
		coord := [2]*big.Rat{ratInset(uv[0]), ratInset(uv[1])}
		j, err := p.EvaluateRational(ctx, coord[0], coord[1], geom.SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		x, _ := j.Point[0].Float64()
		y, _ := j.Point[1].Float64()
		z, _ := j.Point[2].Float64()
		face.ExactUV = append(face.ExactUV, coord)
		face.ExactPoints = append(face.ExactPoints, j.Point)
		face.Mesh.Vertices = append(face.Mesh.Vertices, geom.Vec3{X: x, Y: y, Z: z})
	}
	face.Mesh.Triangles = [][3]int{{0, 1, 2}, {1, 0, 3}}
	proof, err := newMappedRoofNormals(ctx, p, .05, big.NewRat(1, 1<<24))
	if err != nil {
		t.Fatal(err)
	}
	bound, retries, err := proof.certify(ctx, p, face)
	if err != nil || bound == nil || bound.Cmp(proof.limit) > 0 || retries != nil {
		t.Fatal("both original one-sided normals must be covered by their own facet cones", bound, err)
	}
	n := func(tr [3]int) [3]*big.Rat {
		a, b, c := face.ExactPoints[tr[0]], face.ExactPoints[tr[1]], face.ExactPoints[tr[2]]
		return ddCross(ddSub(b, a), ddSub(c, a))
	}
	left, right := face.Mesh.Triangles[0], face.Mesh.Triangles[1]
	uv := [3][2]*big.Rat{face.ExactUV[right[0]], face.ExactUV[right[1]], face.ExactUV[right[2]]}
	if bound, err := proof.bound(ctx, proof.cells[[2]int{1, 0}], n(left), uv, 1); err != nil || bound != nil {
		t.Fatal("using the opposite original differential must fail the stated angle even at a shared crease", bound, err)
	}
}

func TestMappedRoofFiniteAngleProofOnClosedCreasedShell(t *testing.T) {
	ctx := context.Background()
	r, closing, planar, seams := mappedChainClosedFixtureWithKnot(t, true)
	fill := MixedExactMultiSpanMeshOptions{Face: MixedExactCurvedMeshOptions{
		BoundaryModelBudget: .005,
		Interior:            CurvedQuadraticTrimMeshOptions{Deflection: .08},
	}}
	out, err := r.TessellateClosedCertified(ctx, closing, [2]bool{false, true}, planar, seams, MappedChainClosedMeshOptions{
		AlignSupportKnots: true, MaxKnotCellFlips: 10000, NormalAngle: .35,
		Boundary: MappedChainPolylineOptions{Deflection: .005},
		Curved:   [2]MixedExactMultiSpanMeshOptions{fill, fill},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := out.Mesh.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
	if !out.DisplayCollision.Complete || len(out.DisplayCollision.Contacts) != 0 || len(out.Mesh.Triangles) > 6000 {
		t.Fatal("expected bounded closed crease geometry")
	}
	for i, face := range out.Curved {
		norm := face.SquaredNormalTangent
		if norm == nil || norm.Cmp(big.NewRat(133, 1000)) > 0 {
			t.Fatal("missing finite one-sided roof cone", i, norm)
		}
		t.Logf("roof=%d triangles=%d tangent²<=%s", i, len(face.Mesh.Triangles), norm.FloatString(9))
	}
}

func TestMappedRoofNormalTrimNeighborhoodUsesExactUVImage(t *testing.T) {
	a := [3]*big.Rat{new(big.Rat), new(big.Rat), big.NewRat(1, 1)}
	b := [3]*big.Rat{big.NewRat(3, 4), big.NewRat(3, 8), big.NewRat(3, 2)}
	c := [3]*big.Rat{big.NewRat(2, 1), big.NewRat(1, 1), big.NewRat(2, 1)}
	line, err := NewExactQuadraticUV([3][3]*big.Rat{a, b, c})
	if err != nil || !mappedRoofStraightTrimImage([]PCurve{line}) {
		t.Fatal("the positive clock changes only traversal of an exact line", err)
	}
	b[1] = big.NewRat(7, 16)
	bowed, err := NewExactQuadraticUV([3][3]*big.Rat{a, b, c})
	if err != nil || mappedRoofStraightTrimImage([]PCurve{bowed}) {
		t.Fatal("a bowed source trim cannot drop its normal neighborhood", err)
	}
}
