package trimmed_test

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func TestCombineObliqueNativePrisms(t *testing.T) {
	baseA, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: .75, Y: .75, Z: 2})
	baseB, _ := brep.Box(geom.Vec3{X: .25, Y: .25}, geom.Vec3{X: 1, Y: 1, Z: 2})
	// Determinant1: oblique cap z=x+y/2 and an oblique generator(1/2,1/4,13/8).
	pose := geom.Affine{{1, 0, .5, 3}, {0, 1, .25, -2}, {1, .5, 1.625, 4}}
	a, e := baseA.Transformed(pose)
	if e != nil {
		t.Fatal(e)
	}
	b, e := baseB.Transformed(pose)
	if e != nil {
		t.Fatal(e)
	}
	if q, e := trimmed.CombineMatchingNativePrisms(context.Background(), a, b, intersect.AffineRingUnion, trimmed.NativePrismBooleanOptions{}); e == nil || q != nil {
		t.Fatal("cardinal API unexpectedly widened")
	}
	for _, tc := range []struct {
		op     intersect.AffineRingOperation
		volume *big.Rat
	}{{intersect.AffineRingIntersection, big.NewRat(1, 2)}, {intersect.AffineRingUnion, big.NewRat(7, 4)}, {intersect.AffineRingDifference, big.NewRat(5, 8)}} {
		q, e := trimmed.CombineAffineNativePrisms(context.Background(), a, b, tc.op, trimmed.AffineNativePrismOptions{})
		if e != nil {
			t.Fatal(tc.op, e)
		}
		if q.ExactVolume().Cmp(tc.volume) != 0 || len(q.Components()) != 1 {
			t.Fatal(q.ExactVolume(), tc.volume)
		}
		m, e := q.Components()[0].MassProperties()
		want, _ := tc.volume.Float64()
		if e != nil || math.Abs(m.Volume-want) > 1e-10 {
			t.Fatal(m, e)
		}
		caps := q.OriginalCaps()
		for i := 0; i < 2; i++ {
			owner, e := q.OriginalOwner(i)
			if e != nil || len(owner.Faces()) != 6 || caps[i][0] < 0 || caps[i][1] < 0 {
				t.Fatal(owner, caps, e)
			}
		}
		if len(q.WallUses()) == 0 {
			t.Fatal("missing original incidence")
		}
		for _, u := range q.WallUses() {
			owner, _ := q.OriginalOwner(u.Operand)
			edge := owner.Edges()[u.UpperEdge]
			if edge.Start == edge.End || u.Wall < 0 {
				t.Fatal(u)
			}
		}
		if tc.op == intersect.AffineRingIntersection {
			count, work := q.ProofWork()
			if count < 1 || work < 1 {
				t.Fatal(count, work)
			}
			for _, cap := range []int{work, work - 1} {
				r, e := trimmed.CombineAffineNativePrisms(context.Background(), a, b, tc.op, trimmed.AffineNativePrismOptions{MaxTransformOperations: cap})
				if cap == work {
					if e != nil || r == nil {
						t.Fatal(e)
					}
				} else if e == nil || r != nil {
					t.Fatal("partial transformed result")
				}
			}
			matrix := q.CanonicalToOriginal()
			matrix[0][0].SetInt64(99)
			if q.CanonicalToOriginal()[0][0].Cmp(big.NewRat(99, 1)) == 0 {
				t.Fatal("mutable exact frame")
			}
		}
	}
}
func TestCombineObliqueNativePrismHolesAndLimits(t *testing.T) {
	polygon := func(lo, hi float64) []geom.Vec3 {
		return []geom.Vec3{{X: lo, Y: lo}, {X: hi, Y: lo}, {X: hi, Y: hi}, {X: lo, Y: hi}}
	}
	a, e := brep.PrismWithHoles(polygon(0, .75), [][]geom.Vec3{polygon(1./16, 1./8)}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	b, e := brep.PrismWithHoles(polygon(.25, 1), [][]geom.Vec3{polygon(.5, .625)}, geom.Vec3{Z: 2})
	if e != nil {
		t.Fatal(e)
	}
	pose := geom.Affine{{1, 0, .5, 3}, {0, 1, .25, -2}, {1, .5, 1.625, 4}}
	a, e = a.Transformed(pose)
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.Transformed(pose)
	if e != nil {
		t.Fatal(e)
	}
	q, e := trimmed.CombineAffineNativePrisms(context.Background(), a, b, intersect.AffineRingDifference, trimmed.AffineNativePrismOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Components()) != 2 || q.ExactVolume().Cmp(big.NewRat(83, 128)) != 0 {
		t.Fatal(q.ExactVolume())
	}
	volume := 0.
	for _, s := range q.Components() {
		if e := s.Validate(); e != nil {
			t.Fatal(e)
		}
		m, e := s.MassProperties()
		if e != nil {
			t.Fatal(e)
		}
		volume += m.Volume
	}
	if math.Abs(volume-83./128) > 1e-10 {
		t.Fatal(volume)
	}
	for _, o := range []trimmed.AffineNativePrismOptions{{MaxTransformOperations: 1}, {MaxCandidates: -1}, {MaxCoefficientBits: 63}, {Native: trimmed.NativePrismBooleanOptions{Native: trimmed.AffineArrangementPrismOptions{MaxComponents: 1}}}} {
		if r, e := trimmed.CombineAffineNativePrisms(context.Background(), a, b, intersect.AffineRingDifference, o); e == nil || r != nil {
			t.Fatal("accepted limits", o)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if r, e := trimmed.CombineAffineNativePrisms(ctx, a, b, intersect.AffineRingUnion, trimmed.AffineNativePrismOptions{}); e != context.Canceled || r != nil {
		t.Fatal(r, e)
	}
}

func TestCombineAffinePrismDeterminantAndNonPrism(t *testing.T) {
	a, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: .75, Y: .75, Z: 2})
	b, _ := brep.Box(geom.Vec3{X: .25, Y: .25}, geom.Vec3{X: 1, Y: 1, Z: 2})
	// Independent world scaling/reflection: absolute determinant2.
	pose := geom.Affine{{-2, 0, -1, 3}, {0, 1, .25, -2}, {1, .5, 1.625, 4}}
	pa, e := a.Transformed(pose)
	if e != nil {
		t.Fatal(e)
	}
	pb, e := b.Transformed(pose)
	if e != nil {
		t.Fatal(e)
	}
	for _, owners := range [][2]brep.Solid{{pa, pb}, {pb, pa}} {
		q, e := trimmed.CombineAffineNativePrisms(context.Background(), owners[0], owners[1], intersect.AffineRingIntersection, trimmed.AffineNativePrismOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if q.ExactVolume().Cmp(big.NewRat(1, 1)) != 0 {
			t.Fatal(q.ExactVolume())
		}
		mass, e := q.Components()[0].MassProperties()
		if e != nil || math.Abs(mass.Volume-1) > 1e-10 {
			t.Fatal(mass, e)
		}
	}
	polygon := []geom.Vec3{{}, {X: 1}, {X: 1, Y: 1}, {Y: 1}}
	tapered, e := brep.DraftPrism(polygon, nil, geom.Vec3{X: .5, Y: .5}, geom.Vec3{Z: 2}, .5)
	if e != nil {
		t.Fatal(e)
	}
	if q, e := trimmed.CombineAffineNativePrisms(context.Background(), tapered, tapered, intersect.AffineRingUnion, trimmed.AffineNativePrismOptions{}); e == nil || q != nil {
		t.Fatal("cap pairing inferred for non-prism")
	}
}

func TestCombineAffinePrismBoundedDirectionSearch(t *testing.T) {
	a, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: .75, Y: .75, Z: 2})
	b, _ := brep.Box(geom.Vec3{X: .25, Y: .25}, geom.Vec3{X: 1, Y: 1, Z: 2})
	faces := a.Faces()
	faces[0], faces[2] = faces[2], faces[0]
	var e error
	a, e = brep.NewSolid(a.Vertices(), a.Edges(), faces)
	if e != nil {
		t.Fatal(e)
	}
	q, e := trimmed.CombineAffineNativePrisms(context.Background(), a, b, intersect.AffineRingUnion, trimmed.AffineNativePrismOptions{})
	if e != nil {
		t.Fatal(e)
	}
	attempts, _ := q.ProofWork()
	if attempts < 2 {
		t.Fatal("fixture did not exercise competing prism directions", attempts)
	}
	if r, e := trimmed.CombineAffineNativePrisms(context.Background(), a, b, intersect.AffineRingUnion, trimmed.AffineNativePrismOptions{MaxCandidates: 1}); e == nil || r != nil {
		t.Fatal("exhausted candidate search published result")
	}
}
