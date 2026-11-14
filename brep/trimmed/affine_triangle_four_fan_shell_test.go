package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func tensorTriangleFourFanFixture(t *testing.T) ([4]*geom.SurfaceAffineTriangleMate, [2]*geom.SurfaceAffineTriangleVertexContact) {
	t.Helper()
	ctx := context.Background()
	var patches [4]*geom.SurfacePatch
	for owner := range patches {
		var h [][4]*big.Rat
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				u, v := big.NewRat(int64(i), 2), big.NewRat(int64(j), 2)
				z := new(big.Rat)
				if i == 2 {
					z.SetInt64(1)
				}
				if j == 2 {
					z.Sub(z, big.NewRat(1, 1))
				}
				z.Sub(z, u)
				z.Add(z, v)
				z.Quo(z, big.NewRat(int64(8*(owner+1)), 1))
				h = append(h, [4]*big.Rat{new(big.Rat).Add(u, z), new(big.Rat).Add(v, new(big.Rat).Quo(z, big.NewRat(8, 1))), z, big.NewRat(1, 1)})
			}
		}
		p, e := geom.NewExactHomogeneousBezierSurface(ctx, 2, 2, h, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		p, e = p.InsertUKnot([]float64{.125, .25, .5, .75}[owner], 1, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		patches[owner] = p
	}
	pt := func(u, v int64) [2]*big.Rat { return [2]*big.Rat{big.NewRat(u, 2), big.NewRat(v, 2)} }
	center := pt(1, 1)
	rim := [4][2]*big.Rat{pt(0, 0), pt(2, 0), pt(2, 2), pt(0, 2)}
	normal := func(a, b *big.Rat) [3]*big.Rat {
		return [3]*big.Rat{a, b, new(big.Rat).Neg(new(big.Rat).Add(a, new(big.Rat).Quo(b, big.NewRat(8, 1))))}
	}
	var triangles [4][3][2]*big.Rat
	for i := range triangles {
		triangles[i] = [3][2]*big.Rat{center, rim[(i+1)%4], rim[i]}
	}
	var mates [4]*geom.SurfaceAffineTriangleMate
	for i := range mates {
		next := (i + 1) % 4
		coeff := [2]*big.Rat{big.NewRat(1, 1), big.NewRat(-1, 1)}
		level := new(big.Rat)
		if next%2 == 1 {
			coeff[1] = big.NewRat(1, 1)
			level = big.NewRat(1, 1)
		}
		other := [3][2]*big.Rat{center, rim[next], rim[(i+2)%4]}
		p, e := geom.CertifySurfaceAffineTriangleMate(ctx, patches[i], patches[next], [2][3][2]*big.Rat{triangles[i], other}, normal(coeff[0], coeff[1]), [2][2]*big.Rat{coeff, coeff}, [2]*big.Rat{new(big.Rat), new(big.Rat)}, level, geom.SurfaceAffineCoordinateOptions{})
		if e != nil {
			t.Fatal(e)
		}
		mates[i] = p
	}
	var opposite [2]*geom.SurfaceAffineTriangleVertexContact
	for i := range opposite {
		coeff := [2]*big.Rat{new(big.Rat), big.NewRat(1, 1)}
		if i == 1 {
			coeff = [2]*big.Rat{big.NewRat(1, 1), new(big.Rat)}
		}
		q := geom.SurfaceAffineTriangleSeparator{Normal: normal(coeff[0], coeff[1]), UV: [2][2]*big.Rat{coeff, coeff}, Offset: [2]*big.Rat{new(big.Rat), new(big.Rat)}, Level: big.NewRat(1, 2)}
		p, e := geom.CertifySurfaceAffineTriangleVertexContact(ctx, patches[i], patches[i+2], [2][3][2]*big.Rat{triangles[i], triangles[i+2]}, [2]int{0, 0}, q, geom.SurfaceAffineCoordinateOptions{})
		if e != nil {
			t.Fatal(e)
		}
		opposite[i] = p
	}
	return mates, opposite
}
func TestAffineTriangleFourFanShell(t *testing.T) {
	ctx := context.Background()
	mates, opposite := tensorTriangleFourFanFixture(t)
	s, e := NewAffineTriangleFourFanShell(ctx, mates, opposite, AffineTriangleShellOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Vertices()) != 5 || len(s.Edges()) != 8 || len(s.Faces()) != 4 || len(s.Boundary()) != 4 {
		t.Fatal("native four fan topology")
	}
	owners, e := s.VertexSources(s.CenterVertex())
	if e != nil || len(owners) != 4 {
		t.Fatal("four original occurrences")
	}
	solid := Solid{vertices: s.Vertices(), edges: s.Edges(), faces: s.Faces(), tolerance: 1e-9}
	r, e := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
	for i := 0; i < 2; i++ {
		p, e := s.Opposite(i)
		if e != nil || p != opposite[i] || p.Point()[2].Sign() != 0 {
			t.Fatal("opposite source proof")
		}
	}
	_, stale := tensorTriangleFourFanFixture(t)
	if p, e := NewAffineTriangleFourFanShell(ctx, mates, stale, AffineTriangleShellOptions{}); e == nil || p != nil {
		t.Fatal("stale independently valid opposite proofs")
	}
	missing := opposite
	missing[1] = nil
	if p, e := NewAffineTriangleFourFanShell(ctx, mates, missing, AffineTriangleShellOptions{}); e == nil || p != nil {
		t.Fatal("missing opposite pair")
	}
	// Same source patches and central point, but a smaller valid opposite
	// triangle: its certificate must not stand in for the complete owner domain.
	a, ta, _ := opposite[0].Source(0)
	b, tb, _ := opposite[0].Source(1)
	for i := 1; i < 3; i++ {
		for j := 0; j < 2; j++ {
			ta[i][j].Add(ta[i][j], ta[0][j])
			ta[i][j].Quo(ta[i][j], big.NewRat(2, 1))
		}
	}
	changed, e := geom.CertifySurfaceAffineTriangleVertexContact(ctx, a, b, [2][3][2]*big.Rat{ta, tb}, [2]int{0, 0}, opposite[0].Separator(), geom.SurfaceAffineCoordinateOptions{})
	if e != nil {
		t.Fatal(e)
	}
	bad := opposite
	bad[0] = changed
	if p, e := NewAffineTriangleFourFanShell(ctx, mates, bad, AffineTriangleShellOptions{}); e == nil || p != nil {
		t.Fatal("partial opposite-domain proof")
	}
	for _, opts := range []AffineTriangleShellOptions{{MaxWork: s.ExactOperations() - 1}, {Proof: edgecheck.Options{MaxIntervals: 1}}} {
		if p, e := NewAffineTriangleFourFanShell(ctx, mates, opposite, opts); e == nil || p != nil {
			t.Fatal("atomic budget")
		}
	}
	s.fan.graph.faces[0].Outer[0].Reversed = !s.fan.graph.faces[0].Outer[0].Reversed
	if _, e := validateWrappedFanIncidence(ctx, s.fan.graph, s.CenterVertex()); e == nil {
		t.Fatal("reversed cycle")
	}
}
