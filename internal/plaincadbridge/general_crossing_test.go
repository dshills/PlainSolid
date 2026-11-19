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

// TestGeneralOverlapOfCrossingBores measures the overlap of a stock that
// has one bore with the tool of a bore across it. The common is the tool
// less the part both bores share, which is two parts that meet at two
// points.
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
	}{
		{"right angle", vector{20, 24, 18}, [2]vector{{0, 9, 7}, {1, 0, 0}}, 20, [2]vector{{8, 9, 18}, {0, 0, -1}}, 18, 3, 18*9*math.Pi - 144},
		{"slant", vector{20, 16, 40}, [2]vector{{10, 8, -2}, {0, 0, 1}}, 44, [2]vector{{-4, 8, 9.5}, {4, 0, 3}}, 35, 2, 100*math.Pi - 160./3},
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
			// A tool of another radius across the bore has no closed form,
			// and its overlap is refused as the cut is.
			other := circle(t, b, c.second[0], c.second[1], c.radius/2, c.secondHeight)
			if r := ask(t, b, q{"op": "overlapVolume", "a": bored.Handle, "b": other.Handle}); r.OK || r.Error.Code != "UNSUPPORTED" {
				t.Fatalf("overlap with a tool of another radius: ok=%v, %+v", r.OK, r.Error)
			}
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
// along z that covers the small one. Taken small, across, covering, the
// second meets the first along a curve with no closed form and waits for
// the third. Taken with the small one after the one across it, the small
// one waits for the covering one, and then lies in its hole, where it is
// proved clear of what is left of the wall of the bore across. A request
// that has a pair with no closed form in every order fails and publishes
// nothing.
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
	before := len(b.shapes)
	for _, order := range [][]string{{small.Handle, across.Handle}, {across.Handle, small.Handle}} {
		r := ask(t, b, q{"op": "cutMany", "a": stock.Handle, "tools": order})
		if r.OK || r.Error.Code != "UNSUPPORTED" || !strings.Contains(r.Error.Message, "not parallel") || len(b.shapes) != before {
			t.Fatalf("tools %v: ok=%v, %+v", order, r.OK, r.Error)
		}
	}
}
