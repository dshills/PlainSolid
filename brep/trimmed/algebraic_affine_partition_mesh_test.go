package trimmed

import (
	"context"
	"math"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func affineAlgebraicPartitionFixture(t *testing.T, placement *geom.Affine) (AlgebraicSubdivision, [3]*big.Rat) {
	t.Helper()
	s := solidEllipticalLensFixture(t)
	if placement != nil {
		var err error
		s, err = s.Transformed(*placement)
		if err != nil {
			t.Fatal(err)
		}
	}
	return algebraicPartitionForSolid(t, s)
}
func algebraicPartitionForSolid(t *testing.T, s Solid) (AlgebraicSubdivision, [3]*big.Rat) {
	t.Helper()
	ctx := context.Background()
	plane := [4]*big.Rat{big.NewRat(1, 1), nil, nil, big.NewRat(-5, 8)}
	var recipes []AlgebraicSeamRecipe
	for ei, edge := range s.edges {
		model := edge.Curve.(surfaceTrimCurve)
		h, e := quadraticTrimSourceControls(model.curve.(SplineUV))
		if e != nil {
			t.Fatal(e)
		}
		q, e := NewExactQuadraticUV(h)
		if e != nil {
			t.Fatal(e)
		}
		patch, e := exactFaceSupportPatch(Face{Surface: model.surface}, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		hits, e := q.IntersectModelRawPlane(ctx, patch, plane, geom.SurfaceCurveAgreementOptions{}, geom.ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for _, hit := range hits.Hits {
			r, _, e := s.PrepareAlgebraicSeamRecipe(ctx, ei, plane, hit.Event, AlgebraicSeamOptions{})
			if e != nil {
				t.Fatal(e)
			}
			recipes = append(recipes, r)
		}
	}
	if len(recipes) != 2 {
		t.Fatal("two contacts", len(recipes))
	}
	sub, e := s.SubdivideAlgebraicSeams(ctx, recipes, AlgebraicSeamOptions{})
	if e != nil {
		t.Fatal(e)
	}
	line := [3]*big.Rat{big.NewRat(1, 1), new(big.Rat), big.NewRat(-5, 8)}
	return sub, line
}
func TestAffineAlgebraicPartitionBoundary(t *testing.T) {
	ctx := context.Background()
	sub, line := affineAlgebraicPartitionFixture(t, nil)
	o := AlgebraicBoundaryMeshOptions{Display: geom.ComposedCurvePolylineOptions{Deflection: .01}}
	got, e := sub.TessellateAffinePartitionBoundaryCertified(ctx, 1, line, o)
	if e != nil {
		t.Fatal(e)
	}
	if got.CutSquaredError.Sign() < 0 || got.CutSquaredError.Cmp(big.NewRat(1, 10000)) > 0 {
		t.Fatal("cut error")
	}
	directions := [2]int{}
	for fi, ring := range got.Fragments {
		for j, a := range ring {
			b := ring[(j+1)%len(ring)]
			if a == got.Cut[0] && b == got.Cut[1] {
				directions[fi] = 1
			}
			if a == got.Cut[1] && b == got.Cut[0] {
				directions[fi] = -1
			}
		}
	}
	if directions[0]*directions[1] != -1 {
		t.Fatal("opposite cut ownership")
	}
	// Lower graph is exactly z=2; x=5/8 cuts ellipse at y=.5±sqrt(3)/16.
	for _, id := range got.Cut {
		p := got.Boundary.Vertices[id]
		if math.Abs(p.X-.625) > 1e-8 || p.Z != 2 || math.Abs(math.Abs(p.Y-.5)-math.Sqrt(3)/16) > 1e-8 {
			t.Fatal("independent plane cut", p)
		}
	}
	if _, err := sub.TessellateAffinePartitionBoundaryCertified(ctx, 0, line, o); err == nil {
		t.Fatal("curved support accepted as affine")
	}
	filled, e := sub.TessellateAffinePartitionFillCertified(ctx, 1, line, AffinePartitionFillOptions{Boundary: o})
	if e != nil {
		t.Fatal(e)
	}
	if len(filled.Mesh.Triangles) != len(filled.Mesh.Vertices)-2 {
		t.Fatal("disk Euler triangle count")
	}
	if new(big.Rat).Add(filled.FragmentAreas[0], filled.FragmentAreas[1]).Cmp(filled.ProjectedArea) != 0 {
		t.Fatal("partition area")
	}
	// Direct 3D triangle area must equal the independently accumulated XY area
	// on this z=2 fixture; the polygon approximates the full ellipse pi/32.
	area := filled.Mesh.Area()
	exactArea, _ := filled.ProjectedArea.Float64()
	if math.Abs(area-exactArea) > 1e-14 || math.Abs(area-math.Pi/32) > .005 {
		t.Fatal("independent area", area, exactArea)
	}
	contacts, err := collision.SelfIntersections(filled.Mesh, collision.Options{MaxPairs: 100000})
	if err != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
		t.Fatal("independent display contacts", contacts, err)
	}
	for _, tri := range filled.Mesh.Triangles {
		a, b, c := filled.Mesh.Vertices[tri[0]], filled.Mesh.Vertices[tri[1]], filled.Mesh.Vertices[tri[2]]
		if b.Sub(a).Cross(c.Sub(a)).Z >= 0 {
			t.Fatal("physical lower face orientation")
		}
	}
	low := AffinePartitionFillOptions{Boundary: o, MaxPredicates: 1}
	if partial, err := sub.TessellateAffinePartitionFillCertified(ctx, 1, line, low); err == nil || len(partial.Mesh.Triangles) != 0 {
		t.Fatal("fill budget")
	}
	t.Log("fill triangles", len(filled.Mesh.Triangles), "area", area, "predicates", filled.Predicates)

	t.Log("cut squared error", got.CutSquaredError.RatString(), "fragment sizes", len(got.Fragments[0]), len(got.Fragments[1]))
}
