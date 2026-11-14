package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func tensorTriangleFanFixture(t *testing.T, changedClosing bool) [3]*geom.SurfaceAffineTriangleMate {
	t.Helper()
	ctx := context.Background()
	var patches [3]*geom.SurfacePatch
	for owner := 0; owner < 3; owner++ {
		var h [][4]*big.Rat
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				u, v := big.NewRat(int64(i), 2), big.NewRat(int64(j), 2)
				z := new(big.Rat)
				if i == 2 {
					z.SetInt64(1)
				}
				switch owner {
				case 0:
					if j == 2 {
						z.Sub(z, big.NewRat(1, 1))
					}
					z.Sub(z, u)
					z.Add(z, v)
				case 1:
					z.Add(z, new(big.Rat).Mul(u, v))
					z.Sub(z, new(big.Rat).Mul(big.NewRat(3, 2), u))
					z.Sub(z, new(big.Rat).Mul(big.NewRat(1, 2), v))
					z.Add(z, big.NewRat(1, 2))
				case 2:
					z.Sub(z, new(big.Rat).Mul(u, v))
					z.Sub(z, new(big.Rat).Mul(big.NewRat(1, 2), u))
					z.Add(z, new(big.Rat).Mul(big.NewRat(1, 2), v))
				}
				z.Quo(z, big.NewRat(4, 1))
				h = append(h, [4]*big.Rat{new(big.Rat).Add(u, z), v, z, big.NewRat(1, 1)})
			}
		}
		p, e := geom.NewExactHomogeneousBezierSurface(ctx, 2, 2, h, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		p, e = p.InsertUKnot([]float64{.25, .5, .75}[owner], 1, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		patches[owner] = p
	}
	pt := func(u, v int64) [2]*big.Rat { return [2]*big.Rat{big.NewRat(u, 2), big.NewRat(v, 2)} }
	center := pt(1, 1)
	rim := [3][2]*big.Rat{pt(0, 0), pt(2, 0), pt(1, 2)}
	var mates [3]*geom.SurfaceAffineTriangleMate
	coeff := [3][2]*big.Rat{{big.NewRat(1, 1), big.NewRat(-1, 1)}, {big.NewRat(1, 1), big.NewRat(1, 1)}, {big.NewRat(1, 1), new(big.Rat)}}
	levels := [3]*big.Rat{new(big.Rat), big.NewRat(1, 1), big.NewRat(1, 2)}
	for i := range mates {
		next := (i + 1) % 3
		nextRim := (i + 1) % 3
		ta := [3][2]*big.Rat{center, rim[nextRim], rim[i]}
		tb := [3][2]*big.Rat{center, rim[nextRim], rim[(i+2)%3]}
		if changedClosing && i == 2 {
			tb[2] = [2]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 4)}
		}
		q := coeff[nextRim]
		normal := [3]*big.Rat{q[0], q[1], new(big.Rat).Neg(q[0])}
		p, e := geom.CertifySurfaceAffineTriangleMate(ctx, patches[i], patches[next], [2][3][2]*big.Rat{ta, tb}, normal, [2][2]*big.Rat{q, q}, [2]*big.Rat{new(big.Rat), new(big.Rat)}, levels[nextRim], geom.SurfaceAffineCoordinateOptions{})
		if e != nil {
			t.Fatal(e)
		}
		mates[i] = p
	}
	return mates
}
func TestAffineTriangleFanShell(t *testing.T) {
	ctx := context.Background()
	mates := tensorTriangleFanFixture(t, false)
	s, e := NewAffineTriangleFanShell(ctx, mates, AffineTriangleShellOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Vertices()) != 4 || len(s.Edges()) != 6 || len(s.Faces()) != 3 || len(s.Boundary()) != 3 {
		t.Fatal("fan disk counts")
	}
	owners, e := s.VertexSources(s.CenterVertex())
	if e != nil || len(owners) != 3 {
		t.Fatal("central occurrences")
	}
	for _, u := range s.Boundary() {
		if u.Start == s.CenterVertex() || u.End == s.CenterVertex() {
			t.Fatal("center on exterior")
		}
	}
	solid := Solid{vertices: s.Vertices(), edges: s.Edges(), faces: s.Faces(), tolerance: 1e-9}
	r, e := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
	for _, o := range []AffineTriangleShellOptions{{MaxWork: s.ExactOperations() - 1}, {Proof: edgecheck.Options{MaxIntervals: 1}}, {Projection: [2]geom.SurfaceInjectivityOptions{{ProjectionAxes: [2]int{1, 2}, MaxDepth: 1}, {}}}} {
		if p, e := NewAffineTriangleFanShell(ctx, mates, o); e == nil || p != nil {
			t.Fatal("work/partition/projection bounds")
		}
	}
	bad := mates
	bad[2] = mates[0]
	if p, e := NewAffineTriangleFanShell(ctx, bad, AffineTriangleShellOptions{}); e == nil || p != nil {
		t.Fatal("missing closing pair")
	}
	changed := tensorTriangleFanFixture(t, true)
	if p, e := NewAffineTriangleFanShell(ctx, changed, AffineTriangleShellOptions{}); e == nil || p != nil {
		t.Fatal("individually valid mates disagree on original triangle")
	}
	s.graph.faces[0].Outer[0].Reversed = !s.graph.faces[0].Outer[0].Reversed
	if _, e := validateWrappedFanIncidence(ctx, s.graph, s.CenterVertex()); e == nil {
		t.Fatal("reversed cycle")
	}
}
