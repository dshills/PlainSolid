package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestAffineShellFaceFragments(t *testing.T) {
	ctx := context.Background()
	a, b := pairedAffineShellFixture(t)
	s, err := AuthorizeAffineSplineShellPairedImprint(ctx, a, b, AffineShellPairedImprintOptions{})
	if err != nil {
		t.Fatal(err)
	}

	total, native := 0, 0
	for owner := 0; owner < 2; owner++ {
		for face := 0; face < 6; face++ {
			r, e := s.FragmentFace(ctx, owner, face, AffineImprintFragmentOptions{})
			if owner == 0 && face == 2 {
				if e != geom.ErrDomain || r != nil {
					t.Fatal("pinched exterior accepted")
				}
				continue
			}
			if e != nil {
				t.Fatalf("%d/%d: %v", owner, face, e)
			}
			cs := r.Cycles()
			sum := new(big.Rat)
			for _, c := range cs {
				sum.Add(sum, c.Area)
				if c.Area.Sign() <= 0 {
					t.Fatal("nonpositive")
				}
			}
			if sum.Cmp(big.NewRat(1, 1)) != 0 || r.ExactArea().Cmp(sum) != 0 {
				t.Fatal("incomplete area")
			}
			total += len(cs)
			fs, e := r.NativeFaces(ctx, geom.SurfacePatchEvaluationOptions{})
			if e == nil {
				native += len(fs)
				for _, f := range fs {
					if f.Face.Surface != s.sources[owner].faces[face].Surface {
						t.Fatal("replaced support")
					}
					for i, u := range f.Face.Outer {
						p, err := f.Face.Surface.Eval(u.UVStart.X, u.UVStart.Y)
						if err != nil || p != f.Vertices[i] {
							t.Fatal("endpoint mismatch")
						}
						for _, v := range []float64{.25, .5, .75} {
							q, err := f.Edges[i].Curve.Eval(v)
							uv := geom.Vec2{X: u.UVStart.X + v*(u.UVEnd.X-u.UVStart.X), Y: u.UVStart.Y + v*(u.UVEnd.Y-u.UVStart.Y)}
							want, e := f.Face.Surface.Eval(uv.X, uv.Y)
							if err != nil || e != nil || q != want {
								t.Fatal("continuous source use")
							}
						}
					}
				}
			}
			if e != nil && e != geom.ErrDomain {
				t.Fatal(e)
			}
			if _, e = r.NativeFaces(ctx, geom.SurfacePatchEvaluationOptions{MaxOperations: 1}); e == nil {
				t.Fatal("evaluation budget")
			}
			if _, e = s.FragmentFace(ctx, owner, face, AffineImprintFragmentOptions{MaxOperations: r.ExactOperations() - 1}); e == nil {
				t.Fatal("budget")
			}
			cs[0].Area.SetInt64(99)
			if r.Cycles()[0].Area.Cmp(big.NewRat(99, 1)) == 0 {
				t.Fatal("mutable")
			}
		}
	}
	t.Logf("cycles=%d native=%d", total, native)
	if total != 23 || native != 23 {
		t.Fatal("no actual fragmentation/native output")
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := s.FragmentFace(cancelCtx, 0, 0, AffineImprintFragmentOptions{}); e == nil || r != nil {
		t.Fatal("cancellation")
	}
}

func TestAffineShellFragmentAncestryAndLimits(t *testing.T) {
	ctx := context.Background()
	a, b := pairedAffineShellFixture(t)
	for _, pair := range [][2]Solid{{a, b}, {b, a}} {
		s, e := AuthorizeAffineSplineShellPairedImprint(ctx, pair[0], pair[1], AffineShellPairedImprintOptions{})
		if e != nil {
			t.Fatal(e)
		}
		count := 0
		for owner := 0; owner < 2; owner++ {
			for face := 0; face < 6; face++ {
				r, e := s.FragmentFace(ctx, owner, face, AffineImprintFragmentOptions{})
				if e == geom.ErrDomain {
					continue
				}
				if e != nil {
					t.Fatal(e)
				}
				count++
				oo, ff, rev := r.Original()
				if oo != owner || ff != face || rev != pair[owner].faces[face].Reversed {
					t.Fatal("lost owner")
				}
				vs := r.Vertices()
				for _, c := range r.Cycles() {
					for _, u := range c.Uses {
						if u.OriginalEdge < 0 {
							if u.SectionEdge < 0 {
								t.Fatal("missing ancestry")
							}
							continue
						}
						edge := pair[owner].edges[u.OriginalEdge]
						for k, id := range []int{u.Start, u.End} {
							v, _ := u.Parameter[k].Float64()
							p, e := edge.Curve.Eval(edge.Range.Min + v*(edge.Range.Max-edge.Range.Min))
							x, _ := vs[id][0].Float64()
							y, _ := vs[id][1].Float64()
							q, er := pair[owner].faces[face].Surface.Eval(x, y)
							if e != nil || er != nil || p != q {
								t.Fatal("wrong original edge range")
							}
						}
					}
				}
				if len(r.Cycles()) > 1 {
					if v, e := s.FragmentFace(ctx, owner, face, AffineImprintFragmentOptions{MaxCycles: 1}); e == nil || v != nil {
						t.Fatal("cycle quota")
					}
				}
			}
		}
		if count != 11 {
			t.Fatal(count)
		}
	}
}
