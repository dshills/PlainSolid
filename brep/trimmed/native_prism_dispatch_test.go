package trimmed_test

import (
	"context"
	"math"
	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"plainsolid/intersect"
	"testing"
)

func TestCombineNativePrismsFamilies(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	for _, tc := range []struct {
		name   string
		lo, hi geom.Vec3
		op     intersect.AffineRingOperation
		family trimmed.NativePrismMaterialFamily
		volume float64
	}{
		{"crossing fuse", geom.Vec3{X: 1, Y: .5, Z: 1}, geom.Vec3{X: 3, Y: 1.5, Z: 4}, intersect.AffineRingUnion, trimmed.NativePrismFamilyCrossingFuse, 16},
		{"crossing cut", geom.Vec3{X: 1, Y: .5, Z: 1}, geom.Vec3{X: 3, Y: 1.5, Z: 4}, intersect.AffineRingDifference, trimmed.NativePrismFamilyCrossingCut, 10},
		{"cavity", geom.Vec3{X: .5, Y: .5, Z: 1}, geom.Vec3{X: 1.5, Y: 1.5, Z: 2}, intersect.AffineRingDifference, trimmed.NativePrismFamilyCavityCut, 11},
		{"blind", geom.Vec3{X: .5, Y: .5, Z: 1}, geom.Vec3{X: 1.5, Y: 1.5, Z: 4}, intersect.AffineRingDifference, trimmed.NativePrismFamilyBlindCut, 10},
		{"through", geom.Vec3{X: .5, Y: .5, Z: -1}, geom.Vec3{X: 1.5, Y: 1.5, Z: 4}, intersect.AffineRingDifference, trimmed.NativePrismFamilyThroughCut, 9},
		{"boss", geom.Vec3{X: .5, Y: .5, Z: 1}, geom.Vec3{X: 1.5, Y: 1.5, Z: 4}, intersect.AffineRingUnion, trimmed.NativePrismFamilyBossFuse, 13},
		{"matching fuse", geom.Vec3{X: 1, Y: .5}, geom.Vec3{X: 3, Y: 1.5, Z: 3}, intersect.AffineRingUnion, trimmed.NativePrismFamilyMatching, 15},
		{"matching cut", geom.Vec3{X: 1, Y: .5}, geom.Vec3{X: 3, Y: 1.5, Z: 3}, intersect.AffineRingDifference, trimmed.NativePrismFamilyMatching, 9},
		{"common", geom.Vec3{X: 1, Y: .5, Z: 1}, geom.Vec3{X: 3, Y: 1.5, Z: 4}, intersect.AffineRingIntersection, trimmed.NativePrismFamilyCommon, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := transitionPrism(t, tc.lo, tc.hi)
			q, e := trimmed.CombineNativePrisms(context.Background(), a, b, tc.op, trimmed.NativePrismDispatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if q.Kind() != trimmed.NativePrismMaterialSolid || q.Family() != tc.family || len(q.Components()) != 1 || q.CommonProof() == nil {
				t.Fatal(q.Kind(), q.Family())
			}
			m, e := q.Components()[0].MassProperties()
			if e != nil || math.Abs(m.Volume-tc.volume) > 1e-11 {
				t.Fatal(m, e)
			}
			switch tc.family {
			case trimmed.NativePrismFamilyCrossingFuse:
				if q.CrossingFuseProof() == nil {
					t.Fatal("missing typed Fuse")
				}
			case trimmed.NativePrismFamilyCrossingCut:
				if q.CrossingCutProof() == nil {
					t.Fatal("missing typed Cut")
				}
			case trimmed.NativePrismFamilyCavityCut:
				if q.CavityCutProof() == nil {
					t.Fatal("missing cavity")
				}
			case trimmed.NativePrismFamilyBlindCut:
				if q.BlindCutProof() == nil {
					t.Fatal("missing blind")
				}
			case trimmed.NativePrismFamilyThroughCut:
				if q.ThroughCutProof() == nil {
					t.Fatal("missing through")
				}
			case trimmed.NativePrismFamilyBossFuse:
				if q.BossFuseProof() == nil {
					t.Fatal("missing boss")
				}
			case trimmed.NativePrismFamilyMatching:
				if q.MatchingProof() == nil {
					t.Fatal("missing matching")
				}
			}
			if q.SourceOrder() != [2]int{0, 1} || len(q.ComponentOwners()) != 0 {
				t.Fatal("invented single-owner history")
			}
			copy := q.Components()
			copy[0] = brep.Solid{}
			if len(q.Components()[0].Faces()) == 0 {
				t.Fatal("mutable components")
			}
		})
	}
	b := transitionPrism(t, geom.Vec3{X: .5, Y: .5, Z: 1}, geom.Vec3{X: 1.5, Y: 1.5, Z: 4})
	q, e := trimmed.CombineNativePrisms(context.Background(), b, a, intersect.AffineRingUnion, trimmed.NativePrismDispatchOptions{})
	if e != nil || q.SourceOrder() != [2]int{1, 0} || q.BossFuseProof() == nil {
		t.Fatal(q, e)
	}
	owner, e := q.BossFuseProof().OriginalOwner(0)
	if e != nil || len(owner.Vertices()) != len(a.Vertices()) {
		t.Fatal(e)
	}
	for i, v := range owner.Vertices() {
		if v != a.Vertices()[i] {
			t.Fatal("swapped ancestry")
		}
	}
}
func TestCombineNativePrismsEmptyContactAndOriginal(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	far := transitionPrism(t, geom.Vec3{X: 3, Y: .5}, geom.Vec3{X: 4, Y: 1.5, Z: 3})
	touch := transitionPrism(t, geom.Vec3{X: 1, Y: .5, Z: 3}, geom.Vec3{X: 3, Y: 1.5, Z: 4})
	for _, tc := range []struct {
		b    brep.Solid
		kind trimmed.NativePrismMaterialKind
	}{{far, trimmed.NativePrismMaterialEmpty}, {touch, trimmed.NativePrismMaterialContact}} {
		q, e := trimmed.CombineNativePrisms(context.Background(), a, tc.b, intersect.AffineRingIntersection, trimmed.NativePrismDispatchOptions{})
		if e != nil || q.Kind() != tc.kind || len(q.Components()) != 0 {
			t.Fatal(q, e)
		}
		q, e = trimmed.CombineNativePrisms(context.Background(), a, tc.b, intersect.AffineRingDifference, trimmed.NativePrismDispatchOptions{})
		if e != nil || q.Family() != trimmed.NativePrismFamilyOriginal || len(q.ComponentOwners()) != 1 || q.ComponentOwners()[0] != 0 {
			t.Fatal(q, e)
		}
	}
	q, e := trimmed.CombineNativePrisms(context.Background(), a, far, intersect.AffineRingUnion, trimmed.NativePrismDispatchOptions{})
	if e != nil || len(q.Components()) != 2 || q.ComponentOwners()[1] != 1 {
		t.Fatal(q, e)
	}
	if out, e := trimmed.CombineNativePrisms(context.Background(), a, touch, intersect.AffineRingUnion, trimmed.NativePrismDispatchOptions{}); e != nil || out == nil || out.Family() != trimmed.NativePrismFamilyAbuttingFuse {
		t.Fatal("qualified positive-area cap join", out, e)
	}
	large := transitionPrism(t, geom.Vec3{X: -1, Y: -1, Z: -1}, geom.Vec3{X: 3, Y: 3, Z: 4})
	q, e = trimmed.CombineNativePrisms(context.Background(), a, large, intersect.AffineRingDifference, trimmed.NativePrismDispatchOptions{})
	if e != nil || q.Kind() != trimmed.NativePrismMaterialEmpty || len(q.Components()) != 0 {
		t.Fatal(q, e)
	}
	q, e = trimmed.CombineNativePrisms(context.Background(), a, large, intersect.AffineRingUnion, trimmed.NativePrismDispatchOptions{})
	if e != nil || q.Family() != trimmed.NativePrismFamilyOriginal || q.ComponentOwners()[0] != 1 {
		t.Fatal(q, e)
	}
	if out, e := trimmed.CombineNativePrisms(context.Background(), a, far, intersect.AffineRingUnion, trimmed.NativePrismDispatchOptions{Publish: trimmed.AffineArrangementPrismOptions{MaxComponents: 1}}); e == nil || out != nil {
		t.Fatal("component quota")
	}
}
func TestCombineNativePrismsAtomicStages(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	b := transitionPrism(t, geom.Vec3{X: 1, Y: .5, Z: 1}, geom.Vec3{X: 3, Y: 1.5, Z: 4})
	q, e := trimmed.CombineNativePrisms(context.Background(), a, b, intersect.AffineRingUnion, trimmed.NativePrismDispatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range []trimmed.NativePrismDispatchOptions{{Classify: trimmed.NativePrismBooleanOptions{Native: trimmed.AffineArrangementPrismOptions{MaxOperations: q.ClassificationNativeOperations() - 1}}}, {Construct: trimmed.NativePrismBooleanOptions{Arrangement: intersect.AffinePolygonContactOptions{MaxOperations: 1}}}, {Publish: trimmed.AffineArrangementPrismOptions{MaxOperations: 1}}, {Publish: trimmed.AffineArrangementPrismOptions{MaxVertices: 6}}} {
		if out, e := trimmed.CombineNativePrisms(context.Background(), a, b, intersect.AffineRingUnion, o); e == nil || out != nil {
			t.Fatal("partial dispatch")
		}
	}
	o := trimmed.NativePrismDispatchOptions{Classify: trimmed.NativePrismBooleanOptions{Native: trimmed.AffineArrangementPrismOptions{MaxOperations: q.ClassificationNativeOperations()}}}
	if out, e := trimmed.CombineNativePrisms(context.Background(), a, b, intersect.AffineRingUnion, o); e != nil || out == nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, e := trimmed.CombineNativePrisms(ctx, a, b, intersect.AffineRingUnion, trimmed.NativePrismDispatchOptions{}); e != context.Canceled || out != nil {
		t.Fatal(out, e)
	}
}

func TestCombineNativePrismsUnshearedAxes(t *testing.T) {
	a, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	for _, tc := range []struct {
		lo, hi float64
		op     intersect.AffineRingOperation
		family trimmed.NativePrismMaterialFamily
		volume float64
	}{{1, 4, intersect.AffineRingDifference, trimmed.NativePrismFamilyBlindCut, 10}, {-1, 4, intersect.AffineRingDifference, trimmed.NativePrismFamilyThroughCut, 9}, {1, 2, intersect.AffineRingDifference, trimmed.NativePrismFamilyCavityCut, 11}, {1, 4, intersect.AffineRingUnion, trimmed.NativePrismFamilyBossFuse, 13}, {0, 3, intersect.AffineRingDifference, trimmed.NativePrismFamilyMatching, 9}} {
		b, _ := brep.Box(geom.Vec3{X: .5, Y: .5, Z: tc.lo}, geom.Vec3{X: 1.5, Y: 1.5, Z: tc.hi})
		q, e := trimmed.CombineNativePrisms(context.Background(), a, b, tc.op, trimmed.NativePrismDispatchOptions{})
		if e != nil {
			t.Fatal(tc, e)
		}
		if !q.Complete() || q.Family() != tc.family {
			t.Fatal(tc, q.Family())
		}
		m, e := q.Components()[0].MassProperties()
		if e != nil || math.Abs(m.Volume-tc.volume) > 1e-11 {
			t.Fatal(m, e)
		}
	}
	var zero trimmed.NativePrismMaterial
	if zero.Complete() {
		t.Fatal("forged zero proof")
	}
}

func TestCombineNativePrismsUnshearedCrossing(t *testing.T) {
	a, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	b, _ := brep.Box(geom.Vec3{X: 1, Y: .5, Z: 1}, geom.Vec3{X: 3, Y: 1.5, Z: 4})
	for _, op := range []intersect.AffineRingOperation{intersect.AffineRingUnion, intersect.AffineRingDifference} {
		q, e := trimmed.CombineNativePrisms(context.Background(), a, b, op, trimmed.NativePrismDispatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		want := 16.
		family := trimmed.NativePrismFamilyCrossingFuse
		if op == intersect.AffineRingDifference {
			want = 10
			family = trimmed.NativePrismFamilyCrossingCut
		}
		m, e := q.Components()[0].MassProperties()
		if e != nil || q.Family() != family || math.Abs(m.Volume-want) > 1e-11 {
			t.Fatal(q.Family(), m, e)
		}
	}
}
