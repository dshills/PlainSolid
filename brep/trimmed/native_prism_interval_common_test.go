//go:build research

package trimmed_test

import (
	"context"
	"math"
	"math/big"
	"plainsolid/brep"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

func intervalCommonPrism(t *testing.T, lo, hi, z0, z1 float64) brep.Solid {
	t.Helper()
	s, e := brep.Box(geom.Vec3{X: lo, Y: lo, Z: z0}, geom.Vec3{X: hi, Y: hi, Z: z1})
	if e != nil {
		t.Fatal(e)
	}
	s, e = s.Transformed(geom.Affine{{1, .5, 0, 2}, {0, 1, 0, -1}, {0, 0, 1, 0}})
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestNativePrismUnequalIntervalCommon(t *testing.T) {
	a := intervalCommonPrism(t, 0, .75, 0, 3)
	b := intervalCommonPrism(t, .25, 1, 1, 2)
	q, e := trimmed.CommonNativePrismIntervals(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	kind, _ := q.Kind()
	if kind != trimmed.NativePrismCommonMaterial || q.Axis() != 2 || q.Material().ExactVolume().Cmp(big.NewRat(1, 4)) != 0 {
		t.Fatal(kind, q.Axis(), q.Material().ExactVolume())
	}
	caps := q.ControllingCaps()
	if len(caps[0]) != 1 || len(caps[1]) != 1 || caps[0][0].Operand != 1 || caps[1][0].Operand != 1 || caps[0][0].Face != 0 || caps[1][0].Face != 1 {
		t.Fatal(caps)
	}
	seen := [2]bool{}
	for _, u := range q.WallUses() {
		seen[u.Use.Operand] = true
		want := [2]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1)}
		if u.Use.Operand == 0 {
			want = [2]*big.Rat{big.NewRat(1, 3), big.NewRat(2, 3)}
		}
		for i := 0; i < 2; i++ {
			if u.HeightParameter[i].Cmp(want[i]) != 0 {
				t.Fatal("lost exact clipped original height", u)
			}
		}
	}
	if !seen[0] || !seen[1] {
		t.Fatal(seen)
	}
	mass, e := q.Material().Components()[0].MassProperties()
	if e != nil || math.Abs(mass.Volume-.25) > 1e-11 {
		t.Fatal(mass, e)
	}
	for _, cap := range []int{q.NativeOperations(), q.NativeOperations() - 1} {
		out, e := trimmed.CommonNativePrismIntervals(context.Background(), a, b, trimmed.NativePrismBooleanOptions{Native: trimmed.AffineArrangementPrismOptions{MaxOperations: cap}})
		if cap == q.NativeOperations() {
			if e != nil || out == nil {
				t.Fatal(e)
			}
		} else if e == nil || out != nil {
			t.Fatal("partial common")
		}
	}
	// Opposite controllers: lower from B, upper from A.
	a = intervalCommonPrism(t, 0, .75, 0, 2)
	b = intervalCommonPrism(t, .25, 1, 1, 4)
	q, e = trimmed.CommonNativePrismIntervals(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	caps = q.ControllingCaps()
	if caps[0][0].Operand != 1 || caps[1][0].Operand != 0 {
		t.Fatal(caps)
	}
	// A tie retains both original cap owners.
	a = intervalCommonPrism(t, 0, .75, 1, 3)
	b = intervalCommonPrism(t, .25, 1, 1, 2)
	q, e = trimmed.CommonNativePrismIntervals(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	caps = q.ControllingCaps()
	if len(caps[0]) != 2 || len(caps[1]) != 1 {
		t.Fatal(caps)
	}
}
func TestNativePrismIntervalContactAndEmpty(t *testing.T) {
	a := intervalCommonPrism(t, 0, .75, 0, 1)
	for _, tc := range []struct {
		lo, hi float64
		kind   trimmed.NativePrismCommonKind
	}{{1, 2, trimmed.NativePrismCommonContact}, {1.25, 2, trimmed.NativePrismCommonEmpty}} {
		b := intervalCommonPrism(t, .25, 1, tc.lo, tc.hi)
		q, e := trimmed.CommonNativePrismIntervals(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
		if e != nil {
			t.Fatal(e)
		}
		kind, _ := q.Kind()
		if kind != tc.kind || q.Material() != nil || len(q.ProjectedProfile().Area().Cycles()) != 1 {
			t.Fatal(kind, q.Material())
		}
		domain := q.AxialInterval()
		domain[0].SetInt64(99)
		if q.AxialInterval()[0].Cmp(big.NewRat(99, 1)) == 0 {
			t.Fatal("mutable interval")
		}
	}
	// Profile point contact swept over positive height remains non-solid.
	b := intervalCommonPrism(t, .75, 1, 0, 1)
	q, e := trimmed.CommonNativePrismIntervals(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	kind, _ := q.Kind()
	if kind != trimmed.NativePrismCommonContact || q.Material() != nil {
		t.Fatal(kind)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if q, e := trimmed.CommonNativePrismIntervals(ctx, a, b, trimmed.NativePrismBooleanOptions{}); e != context.Canceled || q != nil {
		t.Fatal(q, e)
	}
}

func TestNativePrismIntervalDisconnectedProfile(t *testing.T) {
	uv := [][2]float64{{0, 0}, {1, 0}, {1, 1}, {.75, 1}, {.75, .25}, {.25, .25}, {.25, 1}, {0, 1}}
	points := []geom.Vec3{}
	for _, p := range uv {
		points = append(points, geom.Vec3{X: p[0], Y: p[1]})
	}
	a, e := brep.Prism(points, geom.Vec3{Z: 3})
	if e != nil {
		t.Fatal(e)
	}
	b, e := brep.Prism([]geom.Vec3{{X: .125, Y: .5, Z: 1}, {X: .875, Y: .5, Z: 1}, {X: .875, Y: .75, Z: 1}, {X: .125, Y: .75, Z: 1}}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	pose := geom.Affine{{1, .5, 0, 2}, {0, 1, 0, -1}, {0, 0, 1, 0}}
	a, e = a.Transformed(pose)
	if e != nil {
		t.Fatal(e)
	}
	b, e = b.Transformed(pose)
	if e != nil {
		t.Fatal(e)
	}
	q, e := trimmed.CommonNativePrismIntervals(context.Background(), a, b, trimmed.NativePrismBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if q.Material().ExactVolume().Cmp(big.NewRat(1, 16)) != 0 || len(q.Material().Components()) != 2 {
		t.Fatal(q.Material().ExactVolume())
	}
	volume := 0.
	for _, s := range q.Material().Components() {
		if e := s.Validate(); e != nil {
			t.Fatal(e)
		}
		m, e := s.MassProperties()
		if e != nil {
			t.Fatal(e)
		}
		volume += m.Volume
	}
	if math.Abs(volume-1./16) > 1e-11 {
		t.Fatal(volume)
	}
	if r, e := trimmed.CommonNativePrismIntervals(context.Background(), a, b, trimmed.NativePrismBooleanOptions{Native: trimmed.AffineArrangementPrismOptions{MaxComponents: 1}}); e == nil || r != nil {
		t.Fatal("partial disconnected common")
	}
}
