package geom

import (
	"math"
	"math/big"
	"testing"
)

func TestAxialBlindBoreContact(t *testing.T) {
	box := Box{Vec3{0, 0, 0}, Vec3{20, 10, 10}}
	for _, tc := range []struct {
		x    float64
		want BlindBoreContactKind
	}{{1, BlindBoreSideTangency}, {math.Nextafter(1, 2), BlindBoreStrictInterior}, {math.Nextafter(1, 0), BlindBoreCrossesSide}} {
		got, e := ClassifyAxialBlindBoreContact(box, Vec3{tc.x, 5, 10}, 1, 3, 2, -1)
		if e != nil || got.Kind != tc.want {
			t.Fatalf("%g: %+v %v", tc.x, got, e)
		}
		if got.SignedTravel.Cmp(big.NewRat(-3, 1)) != 0 || got.Floor[2].Cmp(big.NewRat(7, 1)) != 0 {
			t.Fatal(got)
		}
		if tc.want == BlindBoreSideTangency {
			if !got.ManifoldExcluded || len(got.Generators) != 1 || got.Generators[0].Side != 0 || got.Generators[0].Start[0].Sign() != 0 || got.Generators[0].End[2].Cmp(big.NewRat(7, 1)) != 0 {
				t.Fatal(got)
			}
		}
	}
	for axis := 0; axis < 3; axis++ {
		for _, sign := range []int{-1, 1} {
			p := [3]float64{1, 1, 1}
			p[axis] = 0
			if sign < 0 {
				p[axis] = 10
			}
			got, e := ClassifyAxialBlindBoreContact(Box{Vec3{}, Vec3{10, 10, 10}}, Vec3{p[0], p[1], p[2]}, 1, .1, axis, sign)
			if e != nil || got.Kind != BlindBoreMultipleTangency || len(got.Generators) != 2 || !got.ManifoldExcluded {
				t.Fatalf("axis%d sign%d %+v %v", axis, sign, got, e)
			}
			expected := new(big.Rat).SetFloat64(.1)
			if sign < 0 {
				expected.Neg(expected)
			}
			expected.Add(expected, new(big.Rat).SetFloat64(p[axis]))
			if got.Floor[axis].Cmp(expected) != 0 {
				t.Fatal("rounded floor")
			}
			got.Generators[0].End[axis].SetInt64(99)
			if got.Floor[axis].Cmp(expected) != 0 {
				t.Fatal("aliased result")
			}
		}
	}
	got, e := ClassifyAxialBlindBoreContact(Box{Vec3{}, Vec3{2, 2, 10}}, Vec3{1, 1, 0}, 1, 3, 2, 1)
	if e != nil || len(got.Generators) != 4 || got.Kind != BlindBoreMultipleTangency {
		t.Fatal(got, e)
	}
}

func TestAxialBlindBoreContactRejects(t *testing.T) {
	b := Box{Vec3{}, Vec3{20, 10, 10}}
	for _, depth := range []float64{0, -1, 10, 11, math.Inf(1), math.NaN()} {
		if _, e := ClassifyAxialBlindBoreContact(b, Vec3{1, 5, 10}, 1, depth, 2, -1); e == nil {
			t.Fatalf("depth %g", depth)
		}
	}
	for _, p := range []Vec3{{1, 5, 9}, {-1, 5, 10}, {1, 11, 10}} {
		if _, e := ClassifyAxialBlindBoreContact(b, p, 1, 3, 2, -1); e == nil {
			t.Fatal(p)
		}
	}
	if _, e := ClassifyAxialBlindBoreContact(b, Vec3{1, 5, 10}, 1, 3, 2, 1); e == nil {
		t.Fatal("outward accepted")
	}
}
