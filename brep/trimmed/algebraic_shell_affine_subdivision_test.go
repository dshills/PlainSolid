package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestAffineShellAuxiliarySubdivision(t *testing.T) {
	ctx := context.Background()
	a, b := pairedAffineShellFixture(t)
	for order, pair := range [][2]Solid{{a, b}, {b, a}} {
		s, e := AuthorizeAffineSplineShellPairedImprint(ctx, pair[0], pair[1], AffineShellPairedImprintOptions{})
		if e != nil {
			t.Fatal(e)
		}
		total, auxCount := 0, 0
		for owner := 0; owner < 2; owner++ {
			all := []SewFace{}
			for face := 0; face < 6; face++ {
				r, e := s.FragmentFace(ctx, owner, face, AffineImprintFragmentOptions{AuxiliarySubdivision: true})
				if e != nil {
					t.Fatalf("%d/%d: %v", owner, face, e)
				}
				sum := new(big.Rat)
				aux := map[int][]AffineImprintFragmentUse{}
				for _, c := range r.Cycles() {
					sum.Add(sum, c.Area)
					if len(c.Uses) != 3 {
						t.Fatal("nontriangle after complete augmentation")
					}
					for _, u := range c.Uses {
						if u.AuxiliaryEdge >= 0 {
							if u.OriginalEdge >= 0 || u.SectionEdge >= 0 || u.Parameter[0] != nil || u.Parameter[1] != nil {
								t.Fatal("invented source ancestry")
							}
							aux[u.AuxiliaryEdge] = append(aux[u.AuxiliaryEdge], u)
						}
					}
				}
				if sum.Cmp(big.NewRat(1, 1)) != 0 {
					t.Fatal("incomplete source area")
				}
				for _, uses := range aux {
					if len(uses) != 2 || uses[0].Start != uses[1].End || uses[0].End != uses[1].Start {
						t.Fatal("unpaired auxiliary")
					}
				}
				auxCount += len(aux)
				total += len(r.Cycles())
				fs, e := r.NativeFaces(ctx, geom.SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				all = append(all, fs...)
				for _, f := range fs {
					if f.Face.Surface != pair[owner].faces[face].Surface || f.Face.Reversed != pair[owner].faces[face].Reversed {
						t.Fatal("lost original surface")
					}
				}
				if v, e := s.FragmentFace(ctx, owner, face, AffineImprintFragmentOptions{AuxiliarySubdivision: true, MaxOperations: r.ExactOperations() - 1}); v != nil || e == nil {
					t.Fatal("non-atomic arithmetic limit")
				}
			}
			assembled, _, e := Sew(ctx, all, SewOptions{Tolerance: 1e-10})
			if e != nil {
				t.Fatalf("owner %d native whole source: %v", owner, e)
			}
			if len(assembled.vertices)-len(assembled.edges)+len(assembled.faces) != 2 {
				t.Fatal("non-spherical source incidence")
			}
			vol := new(big.Rat)
			for _, f := range all {
				p, q, r := f.Vertices[0], f.Vertices[1], f.Vertices[2]
				x := [3]*big.Rat{ratInset(p.X), ratInset(p.Y), ratInset(p.Z)}
				y := [3]*big.Rat{ratInset(q.X), ratInset(q.Y), ratInset(q.Z)}
				z := [3]*big.Rat{ratInset(r.X), ratInset(r.Y), ratInset(r.Z)}
				det := new(big.Rat)
				for k := 0; k < 3; k++ {
					j, l := (k+1)%3, (k+2)%3
					term := new(big.Rat).Sub(new(big.Rat).Mul(y[j], z[l]), new(big.Rat).Mul(y[l], z[j]))
					det.Add(det, new(big.Rat).Mul(x[k], term))
				}
				if f.Face.Reversed {
					det.Neg(det)
				}
				vol.Add(vol, det)
			}
			vol.Quo(vol, big.NewRat(6, 1))
			want := int64(32)
			if (owner+order)%2 == 1 {
				want = 6
			}
			if vol.Cmp(big.NewRat(want, 1)) != 0 {
				t.Fatalf("source volume %s want %d", vol, want)
			}
		}
		t.Logf("complete original faces=12 native triangles=%d paired auxiliary edges=%d", total, auxCount)
		if auxCount != 35 || total != 60 {
			t.Fatal("no auxiliary proof")
		}
	}
}

func TestAffineShellAuxiliarySubdivisionRejects(t *testing.T) {
	ctx := context.Background()
	a, b := pairedAffineShellFixture(t)
	s, e := AuthorizeAffineSplineShellPairedImprint(ctx, a, b, AffineShellPairedImprintOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range []AffineImprintFragmentOptions{{AuxiliarySubdivision: true, MaxCycles: 1}, {AuxiliarySubdivision: true, MaxVertices: 4}, {AuxiliarySubdivision: true, MaxOperations: 1}} {
		if r, e := s.FragmentFace(ctx, 0, 2, o); r != nil || e == nil {
			t.Fatal("partial publication")
		}
	}
	damaged := *s
	damaged.edges = append(s.Edges(), s.edges[0])
	if r, e := damaged.FragmentFace(ctx, 0, 2, AffineImprintFragmentOptions{AuxiliarySubdivision: true}); r != nil || e == nil {
		t.Fatal("duplicate source atom")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := s.FragmentFace(canceled, 0, 2, AffineImprintFragmentOptions{AuxiliarySubdivision: true}); r != nil || e == nil {
		t.Fatal("cancellation")
	}
}
