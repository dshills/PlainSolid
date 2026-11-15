package fillet

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestCircularSweepObliqueLineCorners(t *testing.T) {
	ctx := context.Background()
	p := func(u, v float64) geom.Vec2 { return geom.Vec2{X: u, Y: v} }
	rotate := func(u, v float64) geom.Vec2 { return p(3*u-4*v, 4*u+3*v) }
	a := mixedCircularFilletRing(p(0, 0), p(18, 24), p(-24, 24), p(-24, 0))
	b := mixedCircularFilletRing(rotate(6, 0), rotate(0, 18), rotate(-12, 18), rotate(-12, 0))
	b[0].Center, b[0].Radius = rotate(0, 8), 50
	r := math.Atan2(4, 3)
	for _, tc := range []struct {
		mode            string
		profile         []trimmed.ExactCircularLoopSegment
		round           trimmed.ExactCircularLoopSegment
		lo, hi          string
		area, perimeter float64
	}{
		{"oblique-lines", a, trimmed.ExactCircularLoopSegment{Start: p(-5, 0), End: p(3, 4), Center: p(-5, 10), Radius: 10},
			"1/6", "19/24", -100 * (.5 - .5*r), 10*r - 10},
		{"oblique-line-arc", b, trimmed.ExactCircularLoopSegment{Start: rotate(4, 0), End: rotate(8, 2), Center: rotate(4, 5), Radius: 25},
			"1/21", "8/9", 25 * (2 + 12.5*r - 100*math.Atan(1./7)), 5 * (5*r - 2 - 20*math.Atan(1./7))},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			source := trimmed.ExactCircularSweep{Outer: tc.profile, Top: 2}
			got, err := CircularSweepCorners(ctx, source, []CircularSweepCorner{{Corner: 0, Radius: tc.round.Radius}}, OrthogonalPrismOptions{})
			if err != nil {
				t.Fatal(err)
			}
			original := got.Original[0]
			if got.Profile.Outer[0] != tc.round || original[0].Minimum.RatString() != tc.lo || original[len(original)-1].Maximum.RatString() != tc.hi {
				t.Fatalf("round or original source intervals changed: %+v, %s %s", got.Profile.Outer[0], original[0].Minimum, original[len(original)-1].Maximum)
			}
			old, err := trimmed.ExtrudeExactDirectedCircularLoop(ctx, tc.profile, 0, 2, trimmed.ExactCircularLoopOptions{})
			if err != nil {
				t.Fatal(err)
			}
			m0, err0 := old.MassProperties(ctx, parametric.IntegrationOptions{})
			m1, err1 := got.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
			dv, da := m1.Volume-m0.Volume, m1.Area-m0.Area
			if err0 != nil || err1 != nil || math.Abs(dv-2*tc.area) > 1e-7 || math.Abs(da-2*(tc.area+tc.perimeter)) > 1e-7 {
				t.Fatalf("bad signed changes (%g, %g); expected (%g, %g)", dv, da, 2*tc.area, 2*(tc.area+tc.perimeter))
			}
			if bin := os.Getenv("PLAINSOLID_OCCT_CIRCULAR_LINE_ARC_FILLETS"); bin != "" {
				raw, err := exec.CommandContext(ctx, bin, tc.mode).CombinedOutput()
				var ref struct{ VolumeChange, AreaChange float64 }
				if err != nil || json.Unmarshal(raw, &ref) != nil || math.Abs(dv-ref.VolumeChange) > 1e-7 ||
					math.Abs(da-ref.AreaChange) > 1e-7 {
					t.Fatalf("native changes (%g, %g) differ from pinned OCCT: %+v, %v, %s", dv, da, ref, err, raw)
				}
			}
		})
	}
	a[0].End.X, a[1].Start.X = 19, 19
	failed, err := CircularSweepCorners(ctx, trimmed.ExactCircularSweep{Outer: a, Top: 2},
		[]CircularSweepCorner{{Corner: 0, Radius: 10}}, OrthogonalPrismOptions{})
	if !errors.Is(err, geom.ErrUnrepresentable) || len(failed.Solid.Faces()) != 0 {
		t.Fatalf("rounded a line with an irrational normal using binary64 positions: %v", err)
	}
}
