package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"plainsolid/intersect"
	"strings"
	"testing"
)

// Independent integration in B's raw (s,t,w) coordinates. The Jacobian is 3/4;
// t has retained length 2-|s/2-w/8|, w in [-2,min(0,-s)]. Splitting at s=-1/2,0
// gives integrals 7/4,23/12,39/16, independently of the boundary triangles.
func affineMaterialFixtureCommonVolume() *big.Rat {
	v := new(big.Rat).Add(big.NewRat(7, 4), big.NewRat(23, 12))
	v.Add(v, big.NewRat(39, 16))
	return v.Mul(v, big.NewRat(3, 4))
}
func affineMaterialFixtureInside(p [3]*big.Rat, second bool) bool {
	x, y, z := new(big.Rat).Set(p[0]), new(big.Rat).Set(p[1]), new(big.Rat).Set(p[2])
	if second {
		x.Quo(new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(4, 1), p[0]), p[2]), big.NewRat(3, 1))
		z.Mul(new(big.Rat).Sub(p[2], p[0]), big.NewRat(4, 3))
		y.Add(p[1], new(big.Rat).Quo(new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(2, 1), p[2]), new(big.Rat).Mul(big.NewRat(5, 1), p[0])), big.NewRat(6, 1)))
	}
	bounds := [3][2]int64{{-2, 2}, {-1, 1}, {-4, 0}}
	if second {
		bounds = [3][2]int64{{-1, 1}, {-1, 1}, {-2, 0}}
	}
	for k, v := range [3]*big.Rat{x, y, z} {
		if v.Cmp(big.NewRat(bounds[k][0], 1)) <= 0 || v.Cmp(big.NewRat(bounds[k][1], 1)) >= 0 {
			return false
		}
	}
	return true
}
func TestAffineShellConvexMaterial(t *testing.T) {
	ctx := context.Background()
	a, b := pairedAffineShellFixture(t)
	for order, pair := range [][2]Solid{{a, b}, {b, a}} {
		s, e := AuthorizeAffineSplineShellPairedImprint(ctx, pair[0], pair[1], AffineShellPairedImprintOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for _, op := range []intersect.AffineRingOperation{intersect.AffineRingIntersection, intersect.AffineRingUnion} {
			r, e := s.CombineConvex(ctx, op, AffineShellMaterialOptions{})
			if e != nil {
				t.Fatal(e)
			}
			want := affineMaterialFixtureCommonVolume()
			nf := 20
			if op == intersect.AffineRingUnion {
				want.Sub(big.NewRat(38, 1), want)
				nf = 40
			}
			if r.ExactVolume().Cmp(want) != 0 || len(r.Solid().Faces()) != nf || len(r.FaceSources()) != nf || len(r.Fragments()) != 60 {
				t.Fatalf("op %v volume %s faces %d", op, r.ExactVolume(), len(r.Solid().Faces()))
			}
			native := r.Solid()
			if len(native.vertices)-len(native.edges)+len(native.faces) != 2 {
				t.Fatal("unexpected topology")
			}
			if e := affineMaterialVertexLinks(ctx, native); e != nil {
				t.Fatal(e)
			}
			for _, f := range r.Fragments() {
				proof := r.SourceFaceFragments(f.Owner, f.Face)
				fs, e := proof.NativeFaces(ctx, geom.SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				center := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
				for _, p := range fs[f.Fragment].Vertices {
					for k, v := range affineMaterialXYZ(p) {
						center[k].Add(center[k], new(big.Rat).Quo(v, big.NewRat(3, 1)))
					}
				}
				inside := affineMaterialFixtureInside(center, (1-f.Owner+order)%2 == 1)
				if inside != f.InsideOther {
					t.Fatal("independent occupancy disagreement")
				}
				keep := inside
				if op == intersect.AffineRingUnion {
					keep = !inside
				}
				if f.Retained != keep || f.Reversed {
					t.Fatal("wrong signed selection")
				}
			}
			for i, f := range r.FaceSources() {
				if native.faces[i].Surface != pair[f.Owner].faces[f.Face].Surface || native.faces[i].Reversed != pair[f.Owner].faces[f.Face].Reversed {
					t.Fatal("source owner changed")
				}
			}
			ledger := r.Fragments()
			ledger[0].Face = -1
			if r.Fragments()[0].Face < 0 {
				t.Fatal("mutable ledger")
			}
			volume := r.ExactVolume()
			volume.SetInt64(0)
			if r.ExactVolume().Sign() <= 0 {
				t.Fatal("mutable volume")
			}
			if v, e := s.CombineConvex(ctx, op, AffineShellMaterialOptions{MaxOperations: r.ExactOperations() - 1}); v != nil || e == nil {
				t.Fatal("partial budget output")
			}
			if _, e := s.CombineConvex(ctx, op, AffineShellMaterialOptions{MaxOperations: r.ExactOperations()}); e != nil {
				t.Fatal("exact budget", e)
			}
		}
		// At the actual simultaneous edge/edge contact, Difference has a disconnected
		// local material fan. Neither operand order may invent a manifold pairing.
		if r, e := s.CombineConvex(ctx, intersect.AffineRingDifference, AffineShellMaterialOptions{}); r != nil || e == nil || !strings.Contains(e.Error(), "disconnected fan") {
			t.Fatalf("singular Difference accepted: %v", e)
		}
	}
}
func TestAffineShellConvexMaterialLimits(t *testing.T) {
	ctx := context.Background()
	a, b := pairedAffineShellFixture(t)
	s, e := AuthorizeAffineSplineShellPairedImprint(ctx, a, b, AffineShellPairedImprintOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range []AffineShellMaterialOptions{{MaxOperations: 1}, {MaxFaces: 1}, {NativeEvaluation: geom.SurfacePatchEvaluationOptions{MaxOperations: 1}}, {MaxOperations: -1}} {
		if r, e := s.CombineConvex(ctx, intersect.AffineRingIntersection, o); r != nil || e == nil {
			t.Fatal("non-atomic limit")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := s.CombineConvex(canceled, intersect.AffineRingIntersection, AffineShellMaterialOptions{}); r != nil || e == nil {
		t.Fatal("cancellation")
	}
	if r, e := s.CombineConvex(ctx, intersect.AffineRingOperation(255), AffineShellMaterialOptions{}); r != nil || e == nil {
		t.Fatal("unknown operation")
	}
}

func affineMaterialOnePairedFixture(t *testing.T) (Solid, Solid) {
	a, _ := pairedAffineShellFixture(t)
	surfaces := []*geom.BSplineSurface{}
	rev := []bool{}
	for axis := 0; axis < 3; axis++ {
		u, v := (axis+1)%3, (axis+2)%3
		for side := 0; side < 2; side++ {
			poles := []geom.Vec3{}
			for i := 0; i < 2; i++ {
				for j := 0; j < 2; j++ {
					q := [3]float64{}
					q[axis] = float64(side)
					q[u] = float64(i)
					q[v] = float64(j)
					x, y, z := 2*q[0]-1, q[1], 2*q[2]-2
					poles = append(poles, geom.Vec3{X: x + z/4, Y: y + x/2 - z/8, Z: z + x})
				}
			}
			p, e := geom.NewBSplineSurface(1, 1, 2, 2, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
			if e != nil {
				t.Fatal(e)
			}
			surfaces = append(surfaces, p)
			rev = append(rev, side == 0)
		}
	}
	b, _, e := SewSplineRectangles(context.Background(), surfaces, make([]geom.ParameterRectangle, 6), rev, SewOptions{Tolerance: 1e-10})
	if e != nil {
		t.Fatal(e)
	}
	return a, b
}
func TestAffineShellConvexDifferencePositive(t *testing.T) {
	ctx := context.Background()
	a, b := affineMaterialOnePairedFixture(t)
	// Independent raw-coordinate integration now uses t in [0,1]: retained
	// lengths 1 (negative offset), 1-offset (positive). The same three s bands
	// integrate to 1,23/24,15/16 before multiplying the Jacobian 3/4.
	common := new(big.Rat).Add(big.NewRat(1, 1), big.NewRat(23, 24))
	common.Add(common, big.NewRat(15, 16))
	common.Mul(common, big.NewRat(3, 4))
	for order, pair := range [][2]Solid{{a, b}, {b, a}} {
		s, e := AuthorizeAffineSplineShellPairedImprint(ctx, pair[0], pair[1], AffineShellPairedImprintOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if len(s.PairedNodes()) != 1 {
			t.Fatal("missing paired source event")
		}
		r, e := s.CombineConvex(ctx, intersect.AffineRingDifference, AffineShellMaterialOptions{})
		if e != nil {
			t.Fatal(e)
		}
		stock := int64(32)
		if order == 1 {
			stock = 3
		}
		want := new(big.Rat).Sub(big.NewRat(stock, 1), common)
		if r.ExactVolume().Cmp(want) != 0 {
			t.Fatalf("volume %s want %s", r.ExactVolume(), want)
		}
		reversed := 0
		solid := r.Solid()
		for i, f := range r.FaceSources() {
			if f.Reversed != (f.Owner == 1) || solid.faces[i].Surface != pair[f.Owner].faces[f.Face].Surface || solid.faces[i].Reversed != (pair[f.Owner].faces[f.Face].Reversed != f.Reversed) {
				t.Fatal("wrong cavity source orientation")
			}
			if f.Reversed {
				reversed++
			}
			if f.InsideOther != (f.Owner == 1) {
				t.Fatal("wrong difference occupancy")
			}
		}
		if reversed == 0 {
			t.Fatal("no tool faces retained")
		}
		if len(solid.vertices)-len(solid.edges)+len(solid.faces) != 2 {
			t.Fatal("wrong material topology")
		}
	}
}
