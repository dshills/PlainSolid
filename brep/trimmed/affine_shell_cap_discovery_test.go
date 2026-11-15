package trimmed

import (
	"context"
	"math/big"
	"plainsolid/brep/parametric"
	"testing"
)

func TestDiscoverAffineShellCap(t *testing.T) {
	ctx := context.Background()
	for _, diamond := range []bool{false, true} {
		a, b := affineCapAbuttingFixtureShape(t, .5, false, diamond)
		for order, pair := range [][2]Solid{{a, b}, {b, a}} {
			r, e := DiscoverAffineShellCapContact(ctx, pair[0], pair[1], AffineShellCapDiscoveryOptions{})
			if e != nil {
				t.Fatal(e)
			}
			want := [2]int{5, 4}
			if order == 1 {
				want = [2]int{4, 5}
			}
			if r.CompleteFacePairs() != 36 || r.Contact().OriginalFaces() != want {
				t.Fatal("incomplete/or wrong original cap discovery")
			}
			if _, e = DiscoverAffineShellCapContact(ctx, pair[0], pair[1], AffineShellCapDiscoveryOptions{MaxOperations: r.ExactOperations()}); e != nil {
				t.Fatal("exact quota", e)
			}
			if q, e := DiscoverAffineShellCapContact(ctx, pair[0], pair[1], AffineShellCapDiscoveryOptions{MaxOperations: r.ExactOperations() - 1}); q != nil || e == nil {
				t.Fatal("short quota")
			}
			for _, op := range []parametric.BooleanOp{parametric.Union, parametric.Intersection, parametric.Difference} {
				solid, h, e := CombineSplineShells(ctx, pair[0], pair[1], op, SplineShellCombineOptions{AffineCap: &AffineShellCapNativeOptions{}})
				if e != nil {
					t.Fatal(e)
				}
				if h.Method != "exact-affine-cap-abutment" || h.AffineCap == nil || h.AffineCap.CompleteFacePairs() != 36 {
					t.Fatal("lost normal authorization")
				}
				if op == parametric.Intersection {
					if len(solid.faces) != 0 || h.AffineCap.Contact().ExactCommonVolume().Sign() != 0 {
						t.Fatal("nonempty common")
					}
					continue
				}
				if op == parametric.Difference {
					if len(solid.faces) != len(pair[0].faces) {
						t.Fatal("changed zero-volume cut")
					}
					for i, f := range solid.faces {
						if f.Surface != pair[0].faces[i].Surface || len(h.FacesA[i]) != 1 || h.FacesA[i][0] != i {
							t.Fatal("lost stock source")
						}
					}
					continue
				}
				wantVolume := big.NewRat(25, 2)
				if diamond {
					wantVolume = big.NewRat(39, 4)
				}
				if h.AffineCapUnion == nil || h.AffineCapUnion.ExactVolume().Cmp(wantVolume) != 0 {
					t.Fatal("wrong union mass")
				}
				faces, edges := map[int]bool{}, map[int]bool{}
				for _, table := range [][][]int{h.FacesA, h.FacesB} {
					for _, ids := range table {
						for _, id := range ids {
							if faces[id] {
								t.Fatal("duplicate source owner")
							}
							faces[id] = true
						}
					}
				}
				for _, table := range [][][]int{h.EdgesA, h.EdgesB} {
					for _, ids := range table {
						for _, id := range ids {
							edges[id] = true
						}
					}
				}
				for _, ids := range [][]int{h.SectionEdges, h.GeneratedEdges} {
					for _, id := range ids {
						edges[id] = true
					}
				}
				if len(faces) != len(solid.faces) || len(edges) != len(solid.edges) {
					t.Fatal("incomplete native history")
				}
			}
		}
	}
}
func TestDiscoverAffineShellCapRejects(t *testing.T) {
	ctx := context.Background()
	a, b := affineCapAbuttingFixtureShape(t, .5, false, true)
	for _, o := range []AffineShellCapDiscoveryOptions{{MaxFacePairs: 35}, {MaxOperations: 1}, {Contact: AffineShellCapContactOptions{MaxOperations: 1}}} {
		if q, e := DiscoverAffineShellCapContact(ctx, a, b, o); q != nil || e == nil {
			t.Fatal("partial search publication")
		}
	}
	// Each coincident original face is individually legitimate, but the pair has
	// multiple area interfaces and cannot be selected by first-success ordering.
	if q, e := DiscoverAffineShellCapContact(ctx, a, a, AffineShellCapDiscoveryOptions{}); q != nil || e == nil {
		t.Fatal("ambiguous extra cap pairs")
	}
	x, y := affineCapAbuttingFixture(t, .5, true)
	if q, e := DiscoverAffineShellCapContact(ctx, x, y, AffineShellCapDiscoveryOptions{}); q != nil || e == nil {
		t.Fatal("same-side occupancy")
	}
	x, y = affineCapAbuttingFixture(t, 1, false)
	if q, e := DiscoverAffineShellCapContact(ctx, x, y, AffineShellCapDiscoveryOptions{}); q != nil || e == nil {
		t.Fatal("line contact")
	}
	for _, o := range []SplineShellCombineOptions{{AffineCap: &AffineShellCapNativeOptions{}, AffinePaired: &ConvexAffineSplineShellOptions{}}, {AffineCap: &AffineShellCapNativeOptions{}, Algebraic: &SplineShellAlgebraicNativeOptions{}}, {AffineCap: &AffineShellCapNativeOptions{Fuse: AffineShellCapFuseOptions{MaxFaces: 1}}}, {AffineCap: &AffineShellCapNativeOptions{}, DisableAffineDiscovery: true}} {
		solid, h, e := CombineSplineShells(ctx, a, b, parametric.Union, o)
		if e == nil || len(solid.faces) != 0 || h.AffineCap != nil || h.AffineCapUnion != nil || h.Method != "" {
			t.Fatal("partial/conflicting normal output")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if q, e := DiscoverAffineShellCapContact(canceled, a, b, AffineShellCapDiscoveryOptions{}); q != nil || e == nil {
		t.Fatal("cancellation")
	}
}
