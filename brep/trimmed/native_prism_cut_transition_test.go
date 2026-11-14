package trimmed_test

import (
	"context"
	"math/big"
	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

func transitionPrism(t *testing.T, lo, hi geom.Vec3) brep.Solid {
	t.Helper()
	s, e := brep.Box(lo, hi)
	if e != nil {
		t.Fatal(e)
	}
	s, e = s.Transformed(geom.Affine{{1, .25, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestNativeCrossingPrismCutTransition(t *testing.T) {
	stock := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	for _, span := range [][2]float64{{1, 4}, {-1, 2}} {
		tool := transitionPrism(t, geom.Vec3{X: 1, Y: .5, Z: span[0]}, geom.Vec3{X: 3, Y: 1.5, Z: span[1]})
		q, e := trimmed.ImprintCrossingNativePrismCut(context.Background(), stock, tool, trimmed.NativePrismBooleanOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if q.Axis() != 2 || q.ExactMaterialVolume().Cmp(big.NewRat(10, 1)) != 0 || len(q.Interfaces()) != 3 {
			t.Fatal(q.Axis(), q.ExactMaterialVolume(), q.Interfaces())
		}
		sign, floor, entry := 1, int64(1), int64(3)
		if span[0] < 0 {
			sign, floor, entry = -1, 2, 0
		}
		if q.FloorNormalSign() != sign || q.Levels()[0].Cmp(big.NewRat(floor, 1)) != 0 || q.Levels()[1].Cmp(big.NewRat(entry, 1)) != 0 {
			t.Fatal(q.Levels())
		}
		cap := q.CapOwners()
		if cap[0].Operand != 1 || cap[1].Operand != 0 || cap[0].Face != map[int]int{1: 0, -1: 1}[sign] || cap[1].Face != map[int]int{1: 1, -1: 0}[sign] {
			t.Fatal(cap)
		}
		for region, area := range []int64{3, 1} {
			arr := q.Region(region)
			sum := new(big.Rat)
			for _, c := range arr.Cycles() {
				sum.Add(sum, c.SignedAreaA)
			}
			if sum.Cmp(big.NewRat(area, 1)) != 0 {
				t.Fatal(sum)
			}
			for _, v := range arr.Vertices() {
				if v.Point[2].Cmp(big.NewRat(floor, 1)) != 0 {
					t.Fatal("wrong original floor")
				}
			}
			uses := q.BoundaryUses(region)
			if len(uses) != len(arr.Edges()) {
				t.Fatal("missing edge owner")
			}
			for _, u := range uses {
				if u.Use.Face != -1 || u.Use.Component != -1 {
					t.Fatal("false native face")
				}
				for _, h := range u.HeightParameter {
					if h.Sign() < 0 || h.Cmp(big.NewRat(1, 1)) > 0 {
						t.Fatal("outside original wall", u)
					}
				}
			}
		}
		rv, re := q.Region(0).Vertices(), q.Region(0).Edges()
		fv, fe := q.Region(1).Vertices(), q.Region(1).Edges()
		for _, pair := range q.Interfaces() {
			a, b := re[pair.RetainedEdge], fe[pair.FloorEdge]
			if a.Owner != 1 || b.Owner != 1 || a.Edge != b.Edge {
				t.Fatal(pair)
			}
			for k := 0; k < 3; k++ {
				if rv[a.Start].Point[k].Cmp(fv[b.End].Point[k]) != 0 || rv[a.End].Point[k].Cmp(fv[b.Start].Point[k]) != 0 {
					t.Fatal("unmatched interface")
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
				out, e := trimmed.ImprintCrossingNativePrismCut(context.Background(), stock, tool, o)
				if delta == 0 {
					if e != nil || out == nil {
						t.Fatal("exact budget", e)
					}
				} else if e == nil || out != nil {
					t.Fatal("partial transition")
				}
			}
		}
		q.Levels()[0].SetInt64(999)
		q.ExactMaterialVolume().SetInt64(0)
		if q.Levels()[0].Cmp(big.NewRat(floor, 1)) != 0 || q.ExactMaterialVolume().Sign() == 0 {
			t.Fatal("mutable")
		}
	}
}
func TestNativeCrossingPrismCutTransitionReject(t *testing.T) {
	stock := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	for _, ends := range [][2]geom.Vec3{
		{{X: .5, Y: .5, Z: 1}, {X: 1.5, Y: 1.5, Z: 4}},
		{{X: 2, Y: .5, Z: 1}, {X: 3, Y: 1.5, Z: 4}},
		{{X: 1, Y: 0, Z: 1}, {X: 3, Y: 1, Z: 4}},
		{{X: 1, Y: .5, Z: -1}, {X: 3, Y: 1.5, Z: 4}},
	} {
		tool := transitionPrism(t, ends[0], ends[1])
		out, e := trimmed.ImprintCrossingNativePrismCut(context.Background(), stock, tool, trimmed.NativePrismBooleanOptions{})
		if e == nil || out != nil {
			t.Fatal("unsupported transition", ends)
		}
	}
	tool := transitionPrism(t, geom.Vec3{X: 1, Y: .5, Z: 1}, geom.Vec3{X: 3, Y: 1.5, Z: 4})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, e := trimmed.ImprintCrossingNativePrismCut(ctx, stock, tool, trimmed.NativePrismBooleanOptions{})
	if e != context.Canceled || out != nil {
		t.Fatal(out, e)
	}
}
