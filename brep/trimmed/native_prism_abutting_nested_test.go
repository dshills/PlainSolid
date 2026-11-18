//go:build research

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

func TestNativeAbuttingEqualNestedPrisms(t *testing.T) {
	ctx := context.Background()
	for _, equal := range []bool{true, false} {
		a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 2})
		lo, hi := geom.Vec3{X: 1, Y: 1, Z: 2}, geom.Vec3{X: 3, Y: 3, Z: 5}
		volume := 44.
		z := 74. / 44
		shoulder := 12.
		if equal {
			lo = geom.Vec3{Z: 2}
			hi = geom.Vec3{X: 4, Y: 4, Z: 5}
			volume = 80
			z = 2.5
			shoulder = 0
		}
		b := transitionPrism(t, lo, hi)
		for _, swap := range []bool{false, true} {
			owners := [2]brep.Solid{a, b}
			if swap {
				owners = [2]brep.Solid{b, a}
			}
			p, e := trimmed.ImprintAbuttingNativePrismFuse(ctx, owners[0], owners[1], trimmed.NativePrismBooleanOptions{})
			if e != nil {
				t.Fatal(equal, swap, e)
			}
			q, e := p.BuildNative(ctx, trimmed.AffineArrangementPrismOptions{})
			if e != nil {
				t.Fatal(equal, swap, e)
			}
			m, e := q.Solid().MassProperties()
			if e != nil || math.Abs(m.Volume-volume) > 1e-10 || math.Abs(m.Centroid.Z-z) > 1e-10 {
				t.Fatal(equal, swap, m, e)
			}
			if len(q.Solid().Vertices())-len(q.Solid().Edges())+len(q.Solid().Faces()) != 2 {
				t.Fatal("Euler")
			}
			area := new(big.Rat)
			for i := 0; i < 2; i++ {
				for _, c := range p.ShoulderProfile(i).Cycles() {
					area.Add(area, c.SignedAreaA)
				}
			}
			if area.Cmp(new(big.Rat).SetFloat64(shoulder)) != 0 {
				t.Fatal("shoulders", area)
			}
			for _, u := range q.WallUses() {
				if u.HeightParameter[0].Sign() != 0 || u.HeightParameter[1].Cmp(big.NewRat(1, 1)) != 0 {
					t.Fatal("wall clipping")
				}
			}
			r, e := trimmed.CombineNativePrisms(ctx, owners[0], owners[1], intersect.AffineRingUnion, trimmed.NativePrismDispatchOptions{})
			if e != nil || r.Family() != trimmed.NativePrismFamilyAbuttingFuse {
				t.Fatal("dispatch", e)
			}
		}
	}
}

func TestNativeAbuttingEqualNestedPrismHoles(t *testing.T) {
	makeRing := func(lo, hi, hlo, hhi, z, dz float64) brep.Solid {
		pts := func(a, b float64) []geom.Vec3 {
			return []geom.Vec3{{X: a, Y: a, Z: z}, {X: b, Y: a, Z: z}, {X: b, Y: b, Z: z}, {X: a, Y: b, Z: z}}
		}
		s, e := brep.PrismWithHoles(pts(lo, hi), [][]geom.Vec3{pts(hlo, hhi)}, geom.Vec3{Z: dz})
		if e != nil {
			t.Fatal(e)
		}
		s, e = s.Transformed(geom.Affine{{1, .25, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}})
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	a := makeRing(0, 6, 2, 4, 0, 2)
	for _, equal := range []bool{false, true} {
		b := makeRing(1, 5, 1.5, 4.5, 2, 3)
		want := 85.
		if equal {
			b = makeRing(0, 6, 2, 4, 2, 3)
			want = 160
		}
		for _, swap := range []bool{false, true} {
			owners := [2]brep.Solid{a, b}
			if swap {
				owners = [2]brep.Solid{b, a}
			}
			p, e := trimmed.ImprintAbuttingNativePrismFuse(context.Background(), owners[0], owners[1], trimmed.NativePrismBooleanOptions{})
			if e != nil {
				t.Fatal(equal, swap, e)
			}
			q, e := p.BuildNative(context.Background(), trimmed.AffineArrangementPrismOptions{})
			if e != nil {
				t.Fatal(equal, swap, e)
			}
			m, e := q.Solid().MassProperties()
			if e != nil || math.Abs(m.Volume-want) > 1e-9 {
				t.Fatal(m, e)
			}
			if len(q.Solid().Vertices())-len(q.Solid().Edges())+len(q.Solid().Faces()) != 0 {
				t.Fatal("retained through hole")
			}
			holes := [2]map[int]bool{{}, {}}
			for owner, s := range owners {
				for _, f := range s.Faces() {
					for _, h := range f.Holes {
						for _, u := range h {
							holes[owner][u.Edge] = true
						}
					}
				}
			}
			seen := [2]map[int]bool{{}, {}}
			for _, u := range q.WallUses() {
				if holes[u.Use.Operand][u.Use.LowerEdge] {
					seen[u.Use.Operand][u.Use.Wall] = true
				}
			}
			if len(seen[0]) != 4 || len(seen[1]) != 4 {
				t.Fatal("lost original hole walls", seen)
			}
		}
	}
}

func TestNativeAbuttingGeneralPrismLimitsAndContact(t *testing.T) {
	ctx := context.Background()
	a := transitionPrism(t, geom.Vec3{}, geom.Vec3{X: 4, Y: 4, Z: 2})
	b := transitionPrism(t, geom.Vec3{X: 1, Y: 1, Z: 2}, geom.Vec3{X: 3, Y: 3, Z: 5})
	p, e := trimmed.ImprintAbuttingNativePrismFuse(ctx, a, b, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, aw := p.Operations()
	for _, limits := range [][2]int{{w, aw}, {w - 1, aw}, {w, aw - 1}} {
		o := trimmed.NativePrismBooleanOptions{}
		o.Native.MaxOperations = limits[0]
		o.Arrangement.MaxOperations = limits[1]
		out, e := trimmed.ImprintAbuttingNativePrismFuse(ctx, a, b, o)
		if limits[0] == w && limits[1] == aw {
			if e != nil {
				t.Fatal(e)
			}
		} else if e == nil || out != nil {
			t.Fatal("partial proof published")
		}
	}
	if out, e := trimmed.ImprintAbuttingCrossingNativePrismFuse(ctx, a, b, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
		t.Fatal("strict API widened")
	}
	// A shared side is not strict containment: preserve the explicit unsupported
	// contact domain instead of selecting a topology from representative uses.
	b = transitionPrism(t, geom.Vec3{Y: 1, Z: 2}, geom.Vec3{X: 3, Y: 3, Z: 5})
	if out, e := trimmed.ImprintAbuttingNativePrismFuse(ctx, a, b, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
		t.Fatal("partial shared perimeter accepted")
	}
	b = transitionPrism(t, geom.Vec3{X: 4, Y: 1, Z: 2}, geom.Vec3{X: 5, Y: 3, Z: 5})
	if out, e := trimmed.ImprintAbuttingNativePrismFuse(ctx, a, b, trimmed.NativePrismBooleanOptions{}); e == nil || out != nil {
		t.Fatal("line contact accepted")
	}
}

func TestNativeAbuttingEqualPrismUnshearedDispatch(t *testing.T) {
	a, _ := brep.Box(geom.Vec3{}, geom.Vec3{X: 2, Y: 2, Z: 3})
	b, _ := brep.Box(geom.Vec3{Z: 3}, geom.Vec3{X: 2, Y: 2, Z: 5})
	q, e := trimmed.CombineNativePrisms(context.Background(), a, b, intersect.AffineRingUnion, trimmed.NativePrismDispatchOptions{})
	if e != nil || q.Family() != trimmed.NativePrismFamilyAbuttingFuse {
		t.Fatal(q, e)
	}
}
