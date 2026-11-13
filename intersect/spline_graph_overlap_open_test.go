package intersect

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestOverlapOpenParabola(t *testing.T) {
	var poles []geom.Vec3
	for _, u := range []float64{0, 1} {
		for j, z := range []float64{.25, .375, .25} {
			poles = append(poles, geom.Vec3{X: u, Y: float64(j) / 2, Z: u - z})
		}
	}
	a, e := geom.NewBSplineSurface(1, 2, 2, 3, poles, []float64{0, 0, 1, 1}, []float64{0, 0, 0, 1, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	k := []float64{0, 0, 1, 1}
	b, e := geom.NewBSplineSurface(1, 1, 2, 2, []geom.Vec3{{X: -.5, Y: .25}, {X: -.5, Y: 1.25}, {X: 1.5, Y: .25}, {X: 1.5, Y: 1.25}}, k, k, nil)
	if e != nil {
		t.Fatal(e)
	}
	pa, pb := Patch{Surface: a, Max: geom.Vec2{X: 1, Y: 1}}, Patch{Surface: b, Max: geom.Vec2{X: 1, Y: 1}}

	for swapped, pair := range [][2]Patch{{pa, pb}, {pb, pa}} {
		r, e := CertifyOpenSplinePlanePair(context.Background(), pair[0], pair[1], OverlappingGraphTraceOptions{})
		if e != nil || !r.Complete || len(r.Trace.Curves) != 1 || len(r.Terminals) != 2 {
			t.Fatal(e, r.Complete)
		}
		if r.Trace.BranchGraph.Branches[0].Closed {
			t.Fatal("open branch marked closed")
		}
		low, high := false, false
		for _, terminal := range r.Terminals {
			p := terminal.Discovery.Point
			if p[2].Sign() != 0 {
				t.Fatal("not on plane")
			}
			if p[1].Cmp(big.NewRat(1, 4)) == 0 {
				low = true
				if p[0].Cmp(big.NewRat(19, 64)) != 0 || terminal.OriginalSides[1-swapped] != 4 || terminal.OriginalSides[swapped] != 0 {
					t.Fatal("clipped terminal ownership", terminal)
				}
			} else if p[1].Cmp(big.NewRat(1, 1)) == 0 {
				high = true
				if p[0].Cmp(big.NewRat(1, 4)) != 0 || terminal.OriginalSides[swapped] != 8 || terminal.OriginalSides[1-swapped] != 0 {
					t.Fatal("original terminal ownership", terminal)
				}
			} else {
				t.Fatal("unexpected exact endpoint", p)
			}
		}
		if !low || !high {
			t.Fatal("missing terminals")
		}
	}
	options := OverlappingGraphTraceOptions{}
	options.Discovery.Tubes.MaxOperations = 1
	bad, e := CertifyOpenSplinePlanePair(context.Background(), pa, pb, options)
	if e == nil || bad.Complete || len(bad.Terminals) != 0 {
		t.Fatal("partial limit output", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = CertifyOpenSplinePlanePair(ctx, pa, pb, OverlappingGraphTraceOptions{})
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	// Generic graph discovery must not silently drop either boundary terminal.
	unresolved, e := CertifyOverlappingAffineXYGraphPair(context.Background(), pa, pb, OverlappingGraphTraceOptions{})
	if e == nil || unresolved.Complete {
		t.Fatal("unsupported generic open trace", e)
	}
}
