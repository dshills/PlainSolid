package plaincadbridge

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"plainsolid/brep/boolean"
)

// TestOverlapOf states the rule of the overlap volume (CO2): the volume of
// the common, and where the common is not a manifold the volume a cut
// removes.
func TestOverlapOf(t *testing.T) {
	pinched := fmt.Errorf("N2: %w", boolean.ErrNonManifold)
	unsupported := fmt.Errorf("X6: %w", boolean.ErrUnsupported)
	stopped := fmt.Errorf("cut: %w", context.Canceled)
	for _, c := range []struct {
		name      string
		common    error
		cut       error
		want      float64
		wantErr   error
		wantAsked bool
	}{
		{"the common is a solid", nil, nil, 7, nil, false},
		{"the common is pinched", pinched, nil, 30 - 18, nil, true},
		{"the common is refused otherwise", unsupported, nil, 0, unsupported, false},
		{"pinched, and the cut fails", pinched, unsupported, 0, pinched, true},
		{"pinched, and the cut is stopped", pinched, stopped, 0, stopped, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			asked := false
			got, err := overlapOf(
				func() (boolean.Result, error) { return boolean.Result{Volume: 7}, c.common },
				func() (boolean.Result, error) {
					asked = true
					return boolean.Result{VolumeA: 30, Volume: 18}, c.cut
				},
			)
			if got != c.want || !errors.Is(err, c.wantErr) || (err == nil) != (c.wantErr == nil) || asked != c.wantAsked {
				t.Fatalf("volume %g, error %v, cut asked %v", got, err, asked)
			}
		})
	}
}

// cylindersCommon is the volume common to two cylinders of radii big and
// small whose axes pass at distance d and make an angle of the sine
// given (A2 of specs/general-boolean-quartic): the section at height y
// along the common perpendicular is a parallelogram with half-widths
// √(big² − y²) and √(small² − (y − d)²). The substitution
// y = middle + half·sin φ takes the square roots out of the ends.
func cylindersCommon(big, small, d, sine float64) float64 {
	lo, hi := math.Max(-big, d-small), math.Min(big, d+small)
	middle, half := (lo+hi)/2, (hi-lo)/2
	f := func(phi float64) float64 {
		s, c := math.Sincos(phi)
		y := middle + half*s
		return math.Sqrt(math.Max(big*big-y*y, 0)*math.Max(small*small-(y-d)*(y-d), 0)) * half * c
	}
	const n = 20000
	h := math.Pi / n
	sum := f(-math.Pi/2) + f(math.Pi/2)
	for k := 1; k < n; k++ {
		sum += float64(2+2*(k%2)) * f(-math.Pi/2+h*float64(k))
	}
	return 4 / sine * sum * h / 3
}

// TestGeneralOverlapOfCrossingBores measures the overlap of a stock that
// has one bore with the tool of a bore across it. The common is the tool
// less the part both bores share, which for equal radii is two parts
// that meet at two points.
func TestGeneralOverlapOfCrossingBores(t *testing.T) {
	type q = map[string]any
	for _, c := range []struct {
		name         string
		max          vector
		first        [2]vector
		firstHeight  float64
		second       [2]vector
		secondHeight float64
		radius       float64
		want         float64
		through      float64 // the length of the second bore inside the stock
		sine         float64 // of the angle between the two axes
	}{
		{"right angle", vector{20, 24, 18}, [2]vector{{0, 9, 7}, {1, 0, 0}}, 20, [2]vector{{8, 9, 18}, {0, 0, -1}}, 18, 3, 18*9*math.Pi - 144, 18, 1},
		{"slant", vector{20, 16, 40}, [2]vector{{10, 8, -2}, {0, 0, 1}}, 44, [2]vector{{-4, 8, 9.5}, {4, 0, 3}}, 35, 2, 100*math.Pi - 160./3, 25, 0.8},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := newGeneral()
			stock := box(t, b, vector{}, c.max)
			first := circle(t, b, c.first[0], c.first[1], c.radius, c.firstHeight)
			bored := shape(t, b, q{"op": "cut", "a": stock.Handle, "b": first.Handle})
			second := circle(t, b, c.second[0], c.second[1], c.radius, c.secondHeight)
			before := len(b.shapes)
			near(t, overlapMeasure(t, b, bored.Handle, second.Handle), c.want)
			near(t, overlapMeasure(t, b, second.Handle, bored.Handle), c.want)
			if len(b.shapes) != before {
				t.Fatal("the overlap published a shape")
			}
			// A tool of half the radius across the bore: the tool less
			// what the two cylinders have in common.
			other := circle(t, b, c.second[0], c.second[1], c.radius/2, c.secondHeight)
			half := c.radius / 2
			near(t, overlapMeasure(t, b, bored.Handle, other.Handle), math.Pi*half*half*c.through-cylindersCommon(c.radius, half, 0, c.sine))
		})
	}
}

// TestInAnyOrder states the fold of cutMany (CO1): a step returned as
// unsupported is set aside and tried again after the others, a round that
// applies none ends with its first error, and any other error ends at
// once.
func TestInAnyOrder(t *testing.T) {
	unsupported := func(i int) error { return fmt.Errorf("step %d: %w", i, boolean.ErrUnsupported) }
	invalid := fmt.Errorf("bad: %w", boolean.ErrInvalid)
	for _, c := range []struct {
		name string
		n    int
		// needs[i] lists the steps that must be applied before step i can be.
		needs   map[int][]int
		fails   map[int]error
		order   []int
		calls   int
		wantErr string
	}{
		{"every step applies at once", 3, nil, nil, []int{0, 1, 2}, 3, ""},
		{"one step waits for a later one", 3, map[int][]int{0: {2}}, nil, []int{1, 2, 0}, 4, ""},
		{"each step waits for the next", 4, map[int][]int{0: {1}, 1: {2}, 2: {3}}, nil, []int{3, 2, 1, 0}, 10, ""},
		{"a step that never applies", 3, map[int][]int{1: {1}}, nil, []int{0, 2}, 4, "step 1"},
		{"no step applies", 2, map[int][]int{0: {0}, 1: {1}}, nil, nil, 2, "step 0"},
		{"another error ends at once", 3, nil, map[int]error{1: invalid}, []int{0}, 2, "bad"},
	} {
		t.Run(c.name, func(t *testing.T) {
			done, calls := map[int]bool{}, 0
			var order []int
			err := inAnyOrder(c.n, func(i int) error {
				calls++
				if e := c.fails[i]; e != nil {
					return e
				}
				for _, need := range c.needs[i] {
					if !done[need] {
						return unsupported(i)
					}
				}
				done[i] = true
				order = append(order, i)
				return nil
			})
			if fmt.Sprint(order) != fmt.Sprint(c.order) || calls != c.calls || calls > c.n*(c.n+1)/2 {
				t.Fatalf("applied %v in %d calls, want %v in %d", order, calls, c.order, c.calls)
			}
			if (err == nil) != (c.wantErr == "") || err != nil && !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("error %v, want %q", err, c.wantErr)
			}
		})
	}
}

// TestGeneralCutManyInAnyOrder cuts three bores from a block in every
// order: a small one along z, a larger one along x across it, and one
// along z that covers the small one. The small one and the one across it
// meet along the curves of specs/general-boolean-quartic, and once the
// covering one is cut the small one lies in its hole. The two that cross
// with unequal radii are also cut alone, in both orders, to the volume
// their common gives.
func TestGeneralCutManyInAnyOrder(t *testing.T) {
	type q = map[string]any
	b := newGeneral()
	stock := box(t, b, vector{}, vector{20, 20, 20})
	small := circle(t, b, vector{10, 10, -1}, vector{0, 0, 1}, 1, 22)
	across := circle(t, b, vector{-1, 10, 10}, vector{1, 0, 0}, 2, 22)
	covering := circle(t, b, vector{10, 10, -1}, vector{0, 0, 1}, 2, 22)
	want := 8000 - 2*80*math.Pi + 16*8./3
	for _, order := range [][]string{
		{small.Handle, across.Handle, covering.Handle},
		{small.Handle, covering.Handle, across.Handle},
		{covering.Handle, small.Handle, across.Handle},
		{across.Handle, small.Handle, covering.Handle},
		{across.Handle, covering.Handle, small.Handle},
		{covering.Handle, across.Handle, small.Handle},
	} {
		got := shape(t, b, q{"op": "cutMany", "a": stock.Handle, "tools": order})
		near(t, got.GeometryAssertions.Volume, want)
	}
	for _, order := range [][]string{{small.Handle, across.Handle}, {across.Handle, small.Handle}} {
		got := shape(t, b, q{"op": "cutMany", "a": stock.Handle, "tools": order})
		near(t, got.GeometryAssertions.Volume, 8000-20*math.Pi-80*math.Pi+cylindersCommon(2, 1, 0, 1))
	}
}

// TestGeneralCrossedResultUse takes a part with crossing bores through
// what a part needs afterwards: a mesh that is a valid solid and keeps the
// volume, and a STEP file that reads back to the same part.
func TestGeneralCrossedResultUse(t *testing.T) {
	type q = map[string]any
	type tool struct {
		origin, axis vector
		height       float64
	}
	for _, c := range []struct {
		name     string
		min, max vector
		radius   float64
		tools    []tool
		volume   float64
	}{
		{"right angle", vector{}, vector{20, 24, 18}, 3, []tool{{vector{0, 9, 7}, vector{1, 0, 0}, 20}, {vector{8, 9, 18}, vector{0, 0, -1}, 18}}, 8640 - 38*9*math.Pi + 144},
		{"slant", vector{}, vector{20, 16, 40}, 2, []tool{{vector{10, 8, -2}, vector{0, 0, 1}, 44}, {vector{-4, 8, 9.5}, vector{4, 0, 3}, 35}}, 12800 - 260*math.Pi + 160./3},
		{"three at one point", vector{2, 4, -2}, vector{34, 40, 38}, 3, []tool{{vector{2, 19, 16}, vector{1, 0, 0}, 32}, {vector{17, 19, 38}, vector{0, 0, -1}, 40}, {vector{17, 4, 16}, vector{0, 1, 0}, 36}}, 32*36*40 - 108*9*math.Pi + 216*math.Sqrt2},
	} {
		t.Run(c.name, func(t *testing.T) {
			b := newGeneral()
			part := box(t, b, c.min, c.max)
			for _, tl := range c.tools {
				bit := circle(t, b, tl.origin, tl.axis, c.radius, tl.height)
				part = shape(t, b, q{"op": "cut", "a": part.Handle, "b": bit.Handle})
			}
			near(t, part.GeometryAssertions.Volume, c.volume)
			m, err := b.shapes[part.Handle].tessellate(request{})
			if err != nil {
				t.Fatal(err)
			}
			if err := m.ValidateSolid(); err != nil {
				t.Fatal(err)
			}
			if off := math.Abs(float32MeshVolume(m) - c.volume); off >= c.volume*meshVolumeBudget {
				t.Fatalf("the mesh is off the volume by %.3g, over the budget of %.3g", off, c.volume*meshVolumeBudget)
			}
			file := generalSTEPExported(t, b, part.Handle)
			sameShape(t, shape(t, b, q{"op": "importStep", "source": file}), part)
			sameShape(t, shape(t, New(), q{"op": "importStep", "source": file}), part)
		})
	}
}
