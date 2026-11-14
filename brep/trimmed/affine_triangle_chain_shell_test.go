package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func affineTriangleFanSupport(t *testing.T, owner int, shift int64) *geom.SurfacePatch {
	t.Helper()
	var controls [][4]*big.Rat
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			u, v := big.NewRat(int64(i), 2), big.NewRat(int64(j), 2)
			z := big.NewRat(shift, 64)
			if owner == 0 {
				if i == 2 {
					z.Add(z, big.NewRat(1, 1))
				}
				if j == 2 {
					z.Add(z, big.NewRat(1, 1))
				}
				z.Sub(z, new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(u, v)))
			}
			if owner == 2 {
				if i == 2 {
					z.Add(z, big.NewRat(1, 1))
				}
				z.Sub(z, u)
				z.Add(z, big.NewRat(1, 4))
			}
			controls = append(controls, [4]*big.Rat{u, v, z, big.NewRat(1, 1)})
		}
	}
	p, e := geom.NewExactHomogeneousBezierSurface(context.Background(), 2, 2, controls, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e = p.InsertUKnot([]float64{.25, .5, .75}[owner], 1, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func triangleChainFixture(t *testing.T) (*geom.SurfaceAffineTriangleMate, *geom.SurfaceAffineTriangleMate, *geom.SurfaceAffineTriangleVertexContact) {
	ctx := context.Background()
	a, b, c := affineTriangleFanSupport(t, 0, 0), affineTriangleFanSupport(t, 1, 0), affineTriangleFanSupport(t, 2, 0)
	pt := func(u, v int64) [2]*big.Rat { return [2]*big.Rat{big.NewRat(u, 2), big.NewRat(v, 2)} }
	ta := [3][2]*big.Rat{pt(1, 1), pt(0, 0), pt(0, 1)}
	tb := [3][2]*big.Rat{pt(1, 1), pt(0, 0), pt(1, 0)}
	tc := [3][2]*big.Rat{pt(1, 1), pt(1, 0), pt(2, 0)}
	q := geom.SurfaceAffineTriangleSeparator{Normal: [3]*big.Rat{big.NewRat(1, 1), big.NewRat(-1, 2), new(big.Rat)}, UV: [2][2]*big.Rat{{big.NewRat(1, 1), big.NewRat(-1, 2)}, {big.NewRat(1, 1), big.NewRat(-1, 2)}}, Offset: [2]*big.Rat{new(big.Rat), new(big.Rat)}, Level: big.NewRat(1, 4)}
	ab, e := geom.CertifySurfaceAffineTriangleMate(ctx, a, b, [2][3][2]*big.Rat{ta, tb}, [3]*big.Rat{big.NewRat(1, 1), big.NewRat(-1, 1), new(big.Rat)}, [2][2]*big.Rat{{big.NewRat(1, 1), big.NewRat(-1, 1)}, {big.NewRat(1, 1), big.NewRat(-1, 1)}}, q.Offset, new(big.Rat), geom.SurfaceAffineCoordinateOptions{})
	if e != nil {
		t.Fatal("AB continuous mate", e)
	}
	tb2 := [3][2]*big.Rat{tb[0], tb[2], tb[1]}
	bc, e := geom.CertifySurfaceAffineTriangleMate(ctx, b, c, [2][3][2]*big.Rat{tb2, tc}, [3]*big.Rat{big.NewRat(1, 1), new(big.Rat), new(big.Rat)}, [2][2]*big.Rat{{big.NewRat(1, 1), new(big.Rat)}, {big.NewRat(1, 1), new(big.Rat)}}, q.Offset, big.NewRat(1, 2), geom.SurfaceAffineCoordinateOptions{})
	if e != nil {
		t.Fatal("BC continuous mate", e)
	}
	originalB, _, _ := ab.Source(1)
	otherB, _, _ := bc.Source(0)
	if originalB != otherB {
		t.Fatal("original middle binding")
	}
	pair := [2][3][2]*big.Rat{ta, tc}
	contact, e := geom.CertifySurfaceAffineTriangleVertexContact(ctx, a, c, pair, [2]int{0, 0}, q, geom.SurfaceAffineCoordinateOptions{})
	if e != nil {
		t.Fatal(e)
	}

	return ab, bc, contact
}

func TestAffineTriangleChainShell(t *testing.T) {
	ctx := context.Background()
	ab, bc, outer := triangleChainFixture(t)
	s, e := NewAffineTriangleChainShell(ctx, ab, bc, outer, AffineTriangleShellOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Vertices()) != 5 || len(s.Edges()) != 7 || len(s.Faces()) != 3 || len(s.Boundary()) != 5 {
		t.Fatal("indexed disk")
	}
	owners, e := s.VertexSources(0)
	if e != nil || len(owners) != 3 {
		t.Fatal("three-owner original vertex fan")
	}
	solid := Solid{vertices: s.Vertices(), edges: s.Edges(), faces: s.Faces(), tolerance: 1e-9}
	r, e := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
	for _, opts := range []AffineTriangleShellOptions{{MaxWork: s.ExactOperations() - 1}, {Proof: edgecheck.Options{MaxWork: 1}}, {Proof: edgecheck.Options{MaxIntervals: 1}}, {Projection: [2]geom.SurfaceInjectivityOptions{{ProjectionAxes: [2]int{0, 2}, MaxDepth: 2}, {}}}} {
		if got, e := NewAffineTriangleChainShell(ctx, ab, bc, outer, opts); e == nil || got != nil {
			t.Fatal("budget or folded projection")
		}
	}
	other, _, _ := triangleChainFixture(t)
	if got, e := NewAffineTriangleChainShell(ctx, other, bc, outer, AffineTriangleShellOptions{}); e == nil || got != nil {
		t.Fatal("unbound original support")
	}
	s.graph.faces[0].Outer[0].Reversed = !s.graph.faces[0].Outer[0].Reversed
	if _, e := validateTriangleChainIncidence(ctx, s.graph); e == nil {
		t.Fatal("reversed incidence")
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := NewAffineTriangleChainShell(c, ab, bc, outer, AffineTriangleShellOptions{}); e == nil || got != nil {
		t.Fatal("cancel")
	}
}
