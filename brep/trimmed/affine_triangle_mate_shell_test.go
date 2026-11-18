//go:build research

package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
	"testing"
)

func triangleShellContact(t *testing.T, fold, swap bool) *geom.SurfaceAffineTriangleMate {
	return triangleShellContactConfigured(t, fold, swap, false)
}
func triangleShellContactConfigured(t *testing.T, fold, swap, refine bool) *geom.SurfaceAffineTriangleMate {
	t.Helper()
	ctx := context.Background()
	var patches [2]*geom.SurfacePatch
	for owner := 0; owner < 2; owner++ {
		var h [][4]*big.Rat
		for i := 0; i < 3; i++ {
			for j := 0; j < 3; j++ {
				u, v := big.NewRat(int64(i), 2), big.NewRat(int64(j), 2)
				q := new(big.Rat)
				if owner == 1 {
					if fold || refine {
						if i == 2 {
							q.Add(q, big.NewRat(1, 1))
						}
						if j == 2 {
							q.Sub(q, big.NewRat(1, 1))
						}
						q.Sub(q, u)
						q.Add(q, v)
						if refine {
							q.Mul(q, big.NewRat(1, 3))
						} else {
							q.Mul(q, big.NewRat(-2, 1))
						}
					} else {
						q.SetInt64(1)
						if i == 2 {
							q.Add(q, big.NewRat(1, 1))
						}
						if j == 2 {
							q.Add(q, big.NewRat(1, 1))
						}
						q.Add(q, new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Mul(u, v)))
						q.Sub(q, new(big.Rat).Mul(big.NewRat(2, 1), new(big.Rat).Add(u, v)))
						q.Quo(q, big.NewRat(4, 1))
					}
				}
				x, y := u, v
				if owner == 1 && swap {
					x, y = y, x
				}
				h = append(h, [4]*big.Rat{new(big.Rat).Add(x, q), new(big.Rat).Sub(y, q), new(big.Rat).Add(u, v), big.NewRat(1, 1)})
			}
		}
		p, e := geom.NewExactHomogeneousBezierSurface(ctx, 2, 2, h, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if refine {
			knot := .5
			if owner == 1 {
				knot = .25
			}
			p, e = p.InsertUKnot(knot, 1, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			p, e = p.InsertVKnot(.5, 1, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
		}
		patches[owner] = p
	}
	pt := func(u, v int64) [2]*big.Rat { return [2]*big.Rat{big.NewRat(u, 1), big.NewRat(v, 1)} }
	triangles := [2][3][2]*big.Rat{{pt(0, 1), pt(1, 0), pt(0, 0)}, {pt(0, 1), pt(1, 0), pt(1, 1)}}
	if swap {
		triangles[1][0], triangles[1][1] = triangles[1][1], triangles[1][0]
	}
	p, e := geom.CertifySurfaceAffineTriangleMate(ctx, patches[0], patches[1], triangles, [3]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(1, 1)}, [2][2]*big.Rat{{big.NewRat(2, 1), big.NewRat(2, 1)}, {big.NewRat(2, 1), big.NewRat(2, 1)}}, [2]*big.Rat{new(big.Rat), new(big.Rat)}, big.NewRat(2, 1), geom.SurfaceAffineCoordinateOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestAffineTriangleMateShell(t *testing.T) {
	ctx := context.Background()
	contact := triangleShellContact(t, false, false)
	s, e := NewAffineTriangleMateShell(ctx, contact, AffineTriangleShellOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(s.Vertices()) != 4 || len(s.Edges()) != 5 || len(s.Faces()) != 2 || len(s.Boundary()) != 4 {
		t.Fatal("native disk topology")
	}
	solid := Solid{vertices: s.Vertices(), edges: s.Edges(), faces: s.Faces(), tolerance: 1e-9}
	r, e := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
	sources, e := s.EdgeSources(0)
	if e != nil || len(sources) != 2 || sources[0].Face != 0 || sources[1].Face != 1 {
		t.Fatal("original mate owners")
	}
	for owner, f := range s.Faces() {
		original, _, _ := contact.Source(owner)
		if f.Surface.(*geom.ExactTensorSurface).ExactPatch() != original {
			t.Fatal("support replaced")
		}
	}
	vertices := s.Vertices()
	vertices[0] = geom.Vec3{}
	if s.Vertices()[0] == (geom.Vec3{}) {
		t.Fatal("mutable vertices")
	}
	for _, o := range []AffineTriangleShellOptions{{MaxWork: s.ExactOperations() - 1}, {Proof: edgecheck.Options{MaxWork: 1}}, {Projection: [2]geom.SurfaceInjectivityOptions{{MaxOperations: 1}, {}}}} {
		if q, e := NewAffineTriangleMateShell(ctx, contact, o); e == nil || q != nil {
			t.Fatal("atomic budget")
		}
	}
	c, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := NewAffineTriangleMateShell(c, contact, AffineTriangleShellOptions{}); e == nil || q != nil {
		t.Fatal("cancel")
	}
}
func TestAffineTriangleMateShellRejectsFoldAndOrientation(t *testing.T) {
	for _, kind := range [][2]bool{{true, false}, {false, true}} {
		contact := triangleShellContact(t, kind[0], kind[1])
		if kind[0] {
			patch, _, _ := contact.Source(1)
			a, e := patch.EvaluateRational(context.Background(), big.NewRat(1, 2), big.NewRat(3, 4), geom.SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			b, e := patch.EvaluateRational(context.Background(), big.NewRat(3, 4), big.NewRat(1, 2), geom.SurfacePatchEvaluationOptions{})
			if e != nil {
				t.Fatal(e)
			}
			for k := 0; k < 3; k++ {
				if a.Point[k].Cmp(b.Point[k]) != 0 {
					t.Fatal("fixture must have actual distinct-UV collision")
				}
			}
		}

		if q, e := NewAffineTriangleMateShell(context.Background(), contact, AffineTriangleShellOptions{Projection: [2]geom.SurfaceInjectivityOptions{{MaxDepth: 2}, {MaxDepth: 2}}}); e == nil || q != nil {
			t.Fatal("unproved embedding/orientation accepted", kind)
		}
	}
}

func TestAffineTriangleMateShellMultiknot(t *testing.T) {
	ctx := context.Background()
	contact := triangleShellContactConfigured(t, false, false, true)
	s, e := NewAffineTriangleMateShell(ctx, contact, AffineTriangleShellOptions{})
	if e != nil {
		t.Fatal(e)
	}
	solid := Solid{vertices: s.Vertices(), edges: s.Edges(), faces: s.Faces(), tolerance: 1e-9}
	r, e := solid.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !r.Complete {
		t.Fatal(r, e)
	}
	for owner, f := range s.Faces() {
		p, tri, _ := contact.Source(owner)
		if f.Surface.(*geom.ExactTensorSurface).ExactPatch() != p {
			t.Fatal("changed original patch")
		}
		pieces, e := p.AffineParameterCurvePieces(ctx, tri[0], tri[1], geom.SurfaceAffineCurveOptions{})
		if e != nil || !pieces.Complete || len(pieces.Pieces) < 2 {
			t.Fatal("knot-crossing mate", pieces, e)
		}
		// A horizontal knot-line composition must preserve exact original units too.
		knotPieces, e := p.AffineParameterCurvePieces(ctx, [2]*big.Rat{new(big.Rat), big.NewRat(1, 2)}, [2]*big.Rat{big.NewRat(1, 1), big.NewRat(1, 2)}, geom.SurfaceAffineCurveOptions{})
		if e != nil || !knotPieces.Complete || !knotPieces.Continuous {
			t.Fatal("edge on original knot", e)
		}
	}
	p, _, _ := contact.Source(1)
	point, e := p.EvaluateRational(ctx, big.NewRat(1, 4), big.NewRat(1, 2), geom.SurfacePatchEvaluationOptions{})
	if e != nil || point.Point[0].Cmp(big.NewRat(13, 48)) != 0 {
		t.Fatal("nonbinary original model", point, e)
	}
}
