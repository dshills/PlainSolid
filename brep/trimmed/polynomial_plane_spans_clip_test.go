package trimmed

import (
	"context"
	"testing"

	"plainsolid/geom"
)

func TestPolynomialPlaneWholeKnotSeamRequired(t *testing.T) {
	ctx := context.Background()
	s := creasedPolynomialClipStock(t)
	proof, e := CertifyPiecewisePolynomialPlaneFaceSection(ctx, s, 4, 3./16, 0, PolynomialPlaneSpanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a, e := proof.patch.Trim(proof.domains[0])
	if e != nil {
		t.Fatal(e)
	}
	b, e := proof.patch.Trim(proof.domains[1])
	if e != nil {
		t.Fatal(e)
	}
	// Scaling the right residual about the cutting plane preserves the exact
	// branch and its junction, but makes the REST of the source seam discontinuous.
	b, e = b.Transformed(geom.Affine{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 2, -3. / 16}})
	if e != nil {
		t.Fatal(e)
	}
	q, _, e := polynomialPlaneTrimAxis(b, ratInset(3./16), 0)
	if e != nil {
		t.Fatal(e)
	}
	right, _, e := polynomialPlaneEndpoints(q)
	if e != nil {
		t.Fatal(e)
	}
	left, _, e := polynomialPlaneEndpoints(proof.pieces[0])
	if e != nil {
		t.Fatal(e)
	}
	for k := range right[0] {
		if right[0][k].Cmp(left[1][k]) != 0 {
			t.Fatal("branch junction must match")
		}
	}
	if e = quadraticPlaneSeam(ctx, a, b, 0, .5, []float64{0, 1}, geom.SurfacePatchEvaluationOptions{}); e == nil {
		t.Fatal("accepted discontinuous source through matching branch point")
	}
}
