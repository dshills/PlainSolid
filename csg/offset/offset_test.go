package offset

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/csg/prism"
	"plainsolid/geom"
	"plainsolid/solid"
	"testing"
)

func rectangle(x0, y0, x1, y1 float64) prism.Profile {
	return prism.Profile{Outer: []geom.Vec2{{X: x0, Y: y0}, {X: x1, Y: y0}, {X: x1, Y: y1}, {X: x0, Y: y1}}}
}
func compute(t *testing.T, p []prism.Profile, r, area float64) Result {
	t.Helper()
	out, e := Compute(context.Background(), p, 2, 5, r, Options{})
	if e != nil {
		t.Fatal(e)
	}
	exact, _ := out.ExactArea.Float64()
	if math.Abs(exact-area) > 1e-10 {
		t.Fatal("exact area", exact, area)
	}
	if area == 0 {
		if len(out.Cells) != 0 || len(out.Solid.Faces()) != 0 {
			t.Fatal("nonempty zero-area result")
		}
		return out
	}
	props, e := out.Solid.MassProperties()
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(props.Volume-3*area) > 1e-9 {
		t.Fatal("volume", props.Volume, 3*area)
	}
	sum := new(big.Rat)
	for _, c := range out.Cells {
		sum.Add(sum, mul(sub(c.Max[0], c.Min[0]), sub(c.Max[1], c.Min[1])))
	}
	if sum.Cmp(out.ExactArea) != 0 {
		t.Fatal("cell cover area mismatch")
	}
	return out
}
func TestRectangleDilationErosion(t *testing.T) {
	p := []prism.Profile{rectangle(0, 0, 2, 4)}
	compute(t, p, 0, 8)
	compute(t, p, .5, 15)
	compute(t, p, -.5, 3)
	compute(t, p, -1, 0)
	compute(t, p, -3, 0)
}
func TestNotchClosure(t *testing.T) {
	p := prism.Profile{Outer: []geom.Vec2{{}, {X: 6}, {X: 6, Y: 6}, {X: 4, Y: 6}, {X: 4, Y: 2}, {X: 2, Y: 2}, {X: 2, Y: 6}, {Y: 6}}}
	compute(t, []prism.Profile{p}, 0, 28)
	compute(t, []prism.Profile{p}, .5, 45)
	r := compute(t, []prism.Profile{p}, 1, 64)
	if r.Solid.Topology().Shells != 1 {
		t.Fatal("closure changed component count")
	}
}
func TestHoleClosure(t *testing.T) {
	p := rectangle(0, 0, 8, 8)
	p.Holes = [][]geom.Vec2{rectangle(2, 2, 6, 6).Outer}
	compute(t, []prism.Profile{p}, 0, 48)
	r := compute(t, []prism.Profile{p}, 1, 96)
	if r.Solid.Topology().EulerCharacteristic != 0 {
		t.Fatal("hole lost early")
	}
	r = compute(t, []prism.Profile{p}, 2, 144)
	if r.Solid.Topology().EulerCharacteristic != 2 {
		t.Fatal("hole not closed")
	}
	compute(t, []prism.Profile{p}, -.5, 24)
	compute(t, []prism.Profile{p}, -1, 0)
}
func TestErosionSplitsBridge(t *testing.T) {
	p := []prism.Profile{rectangle(0, 0, 3, 3), rectangle(3, 1, 5, 2), rectangle(5, 0, 8, 3)}
	compute(t, p, 0, 20)
	compute(t, p, -.25, 13.75)
	r := compute(t, p, -.5, 8)
	if r.Solid.Topology().Shells != 2 {
		t.Fatal("expected two eroded components", r.Solid.Topology())
	}
}
func TestPrecisionAndBudgets(t *testing.T) {
	p := []prism.Profile{rectangle(0, 0, 2, 4)}
	r, e := Compute(context.Background(), p, 0, 1, .1, Options{})
	if e != nil {
		t.Fatal(e)
	}
	d := num(.1)
	want := mul(add(num(2), mul(num(2), d)), add(num(4), mul(num(2), d)))
	if r.ExactArea.Cmp(want) != 0 || r.MaxDisplacement <= 0 {
		t.Fatal("lost exact footprint", r.MaxDisplacement)
	}
	if _, e = Compute(context.Background(), p, 0, 1, .1, Options{MaxCoordinateError: 1e-30}); !errors.Is(e, ErrPrecision) {
		t.Fatal(e)
	}
	if _, e = Compute(context.Background(), p, 0, 1, math.SmallestNonzeroFloat64, Options{}); !errors.Is(e, ErrPrecision) {
		t.Fatal(e)
	}
	for _, o := range []Options{{MaxWork: 1}, {MaxCells: 1}, {MaxSegments: 3}} {
		if _, e = Compute(context.Background(), p, 0, 1, 1, o); !errors.Is(e, ErrLimit) {
			t.Fatal(e)
		}
	}
	for _, o := range []Options{{MaxWork: -1}, {MaxCells: -1}, {MaxSegments: 2049}, {MaxCoordinateError: -1}} {
		if _, e = Compute(context.Background(), p, 0, 1, 1, o); !errors.Is(e, ErrInvalid) {
			t.Fatal(e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = Compute(ctx, p, 0, 1, 1, Options{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	r, e = Compute(context.Background(), nil, 0, 1, 1, Options{})
	if e != nil || r.ExactArea.Sign() != 0 {
		t.Fatal(e)
	}
}
func TestInvalidAndNonmanifold(t *testing.T) {
	for _, p := range []prism.Profile{{Outer: []geom.Vec2{{}, {X: 1, Y: 1}, {X: 2}, {Y: 2}}}, {Outer: []geom.Vec2{{}, {X: 2}, {X: 2, Y: 2}, {}, {Y: 2}}}} {
		if _, e := Compute(context.Background(), []prism.Profile{p}, 0, 1, .5, Options{}); !errors.Is(e, ErrInvalid) {
			t.Fatal(e)
		}
	}
	p := rectangle(0, 0, 2, 2)
	p.Holes = [][]geom.Vec2{rectangle(1, 1, 3, 3).Outer}
	if _, e := Compute(context.Background(), []prism.Profile{p}, 0, 1, 1, Options{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	if _, e := Compute(context.Background(), []prism.Profile{rectangle(0, 0, 1, 1), rectangle(2, 2, 3, 3)}, 0, 1, .5, Options{}); e == nil {
		t.Fatal("point-contact result accepted")
	}
}

func TestFromSolidAndVaryingHeight(t *testing.T) {
	p := rectangle(0, 0, 8, 8)
	p.Holes = [][]geom.Vec2{rectangle(2, 2, 6, 6).Outer}
	original := compute(t, []prism.Profile{p}, 0, 48)
	r, e := FromSolid(context.Background(), original.Solid, 1, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if r.ExactArea.Cmp(big.NewRat(96, 1)) != 0 {
		t.Fatal(r.ExactArea)
	}
}
func TestCrossMorphologyAndHeightRejection(t *testing.T) {
	p := []prism.Profile{rectangle(0, 2, 7, 5), rectangle(2, 0, 5, 7)}
	compute(t, p, 1, 65)
	compute(t, p, -1, 9)
	cells, e := solid.New(geom.Box{Min: geom.Vec3{}, Max: geom.Vec3{X: 2, Y: 2, Z: 1}}, geom.Box{Min: geom.Vec3{X: 2}, Max: geom.Vec3{X: 4, Y: 2, Z: 2}})
	if e != nil {
		t.Fatal(e)
	}
	s, e := cells.BRep(solid.Options{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = FromSolid(context.Background(), s, 1, Options{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
}
func FuzzRectangleOffset(f *testing.F) {
	f.Add(uint8(2), uint8(4), int8(2))
	f.Add(uint8(1), uint8(1), int8(-4))
	f.Fuzz(func(t *testing.T, x, y uint8, step int8) {
		w, h := float64(x%12+1)/2, float64(y%12+1)/2
		r := float64(step%16) / 4
		area := 0.
		if w+2*r > 0 && h+2*r > 0 {
			area = (w + 2*r) * (h + 2*r)
		}
		compute(t, []prism.Profile{rectangle(-1, -2, w-1, h-2)}, r, area)
	})
}

func TestFromSolidIgnoresVerticalSubdivision(t *testing.T) {
	cells, e := solid.New(geom.Box{Min: geom.Vec3{}, Max: geom.Vec3{X: 2, Y: 2, Z: 1}}, geom.Box{Min: geom.Vec3{Z: 1}, Max: geom.Vec3{X: 2, Y: 2, Z: 2}})
	if e != nil {
		t.Fatal(e)
	}
	s, e := cells.BRep(solid.Options{})
	if e != nil {
		t.Fatal(e)
	}
	r, e := FromSolid(context.Background(), s, .5, Options{})
	if e != nil {
		t.Fatal(e)
	}
	if r.ExactArea.Cmp(big.NewRat(9, 1)) != 0 {
		t.Fatal(r.ExactArea)
	}
}
