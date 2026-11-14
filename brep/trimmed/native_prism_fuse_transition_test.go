package trimmed_test

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

func TestNativeCrossingPrismFuseTransition(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	b := transitionPrism(t, geom.Vec3{X: 1, Y: .5, Z: 1}, geom.Vec3{X: 3, Y: 1.5, Z: 4})
	for _, swapped := range []bool{false, true} {
		owners := [2]brep.Solid{a, b}
		if swapped {
			owners[0], owners[1] = b, a
		}
		q, e := trimmed.ImprintCrossingNativePrismFuse(context.Background(), owners[0], owners[1], trimmed.NativePrismBooleanOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if q.Axis() != 2 || q.ExactMaterialVolume().Cmp(big.NewRat(16, 1)) != 0 {
			t.Fatal(q.Axis(), q.ExactMaterialVolume())
		}
		for i, z := range []int64{0, 1, 3, 4} {
			if q.Levels()[i].Cmp(big.NewRat(z, 1)) != 0 {
				t.Fatal(q.Levels())
			}
		}
		sum := new(big.Rat)
		for _, c := range q.UnionProfile().Cycles() {
			sum.Add(sum, c.SignedAreaA)
		}
		if sum.Cmp(big.NewRat(5, 1)) != 0 {
			t.Fatal(sum)
		}
		for owner := 0; owner < 2; owner++ {
			isA := (owner == 0) != swapped
			want, level, normal := int64(1), int64(1), -1
			if isA {
				want, level, normal = 3, 3, 1
			}
			area := new(big.Rat)
			for _, c := range q.Shoulder(owner).Cycles() {
				area.Add(area, c.SignedAreaA)
			}
			if area.Cmp(big.NewRat(want, 1)) != 0 || q.ShoulderLevel(owner).Cmp(big.NewRat(level, 1)) != 0 || q.ShoulderNormalSign(owner) != normal {
				t.Fatal("wrong shoulder", owner)
			}
		}
		if len(q.BoundaryPairs()) != len(q.UnionProfile().Edges()) || len(q.OverlapWallUses()) != len(q.BoundaryPairs()) {
			t.Fatal("incomplete exposed union")
		}
		for i, p := range q.BoundaryPairs() {
			u := q.UnionProfile().Edges()[p.UnionEdge]
			v := q.Shoulder(p.Shoulder).Edges()[p.ShoulderEdge]
			if p.UnionEdge != i || u.Owner != p.Shoulder || v.Owner != 0 || u.Edge != v.Edge || u.Parameter[0].Cmp(v.Parameter[0]) != 0 || u.Parameter[1].Cmp(v.Parameter[1]) != 0 {
				t.Fatal(p)
			}
			use := q.OverlapWallUses()[i]
			if use.Use.Operand != p.Shoulder || use.Use.Face != -1 {
				t.Fatal(use)
			}
			for _, h := range use.HeightParameter {
				if h.Sign() < 0 || h.Cmp(big.NewRat(1, 1)) > 0 {
					t.Fatal("outside source height")
				}
			}
		}
		nw, aw := q.Operations()
		for _, native := range []bool{true, false} {
			for _, delta := range []int{0, -1} {
				o := trimmed.NativePrismBooleanOptions{}
				if native {
					o.Native.MaxOperations = nw + delta
				} else {
					o.Arrangement.MaxOperations = aw + delta
				}
				out, e := trimmed.ImprintCrossingNativePrismFuse(context.Background(), owners[0], owners[1], o)
				if delta == 0 {
					if e != nil || out == nil {
						t.Fatal("exact limit", e)
					}
				} else if e == nil || out != nil {
					t.Fatal("partial fuse")
				}
			}
		}
		q.Levels()[0].SetInt64(99)
		q.ExactMaterialVolume().SetInt64(0)
		if q.Levels()[0].Sign() != 0 || q.ExactMaterialVolume().Sign() == 0 {
			t.Fatal("mutable")
		}
	}
}
func TestNativeCrossingPrismFuseTransitionReject(t *testing.T) {
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	for _, ends := range [][2]geom.Vec3{
		{{X: 1, Y: .5, Z: 1}, {X: 3, Y: 1.5, Z: 2}},
		{{X: 1, Y: .5, Z: 0}, {X: 3, Y: 1.5, Z: 4}},
		{{X: 1, Y: .5, Z: 3}, {X: 3, Y: 1.5, Z: 4}},
		{{X: .5, Y: .5, Z: 1}, {X: 1.5, Y: 1.5, Z: 4}},
		{{X: 2, Y: .5, Z: 1}, {X: 3, Y: 1.5, Z: 4}},
	} {
		b := transitionPrism(t, ends[0], ends[1])
		if out, e := trimmed.ImprintCrossingNativePrismFuse(context.Background(), a, b, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
			t.Fatal("unsupported fuse", ends)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, e := trimmed.ImprintCrossingNativePrismFuse(ctx, a, a, trimmed.NativePrismBooleanOptions{}); e != context.Canceled || out != nil {
		t.Fatal(out, e)
	}
}
