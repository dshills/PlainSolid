package plaincadbridge

import (
	"context"
	"errors"
	"fmt"
	"math"
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
