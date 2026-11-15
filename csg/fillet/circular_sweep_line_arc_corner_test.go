package fillet

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestCircularSweepLineArcCornersRetainOriginalCircle(t *testing.T) {
	ctx := context.Background()
	p := func(x, y float64) geom.Vec2 { return geom.Vec2{X: x, Y: y} }
	for _, tc := range []struct {
		name                          string
		corner, end, support, center  geom.Vec2
		tangencyOnLine, tangencyOnArc geom.Vec2
		arcRadius                     float64
		rest                          []geom.Vec2
	}{
		{"internal", p(6, 0), p(0, 18), p(0, 8), p(4, 5), p(4, 0), p(8, 2), 10, []geom.Vec2{p(-12, 18), p(-12, 0)}},
		{"external", p(-4, 0), p(5, 3), p(0, 3), p(-6, -5), p(-6, 0), p(-3, -1), 5, []geom.Vec2{p(5, 16), p(-20, 16), p(-20, 0)}},
	} {
		for _, swap := range []bool{false, true} {
			label := tc.name
			if swap {
				label += "-arc-first"
			}
			t.Run(label, func(t *testing.T) {
				ring := mixedCircularFilletRing(append([]geom.Vec2{tc.corner, tc.end}, tc.rest...)...)
				ring[0].Center, ring[0].Radius = tc.support, tc.arcRadius
				selection, parent := 0, 0
				if swap {
					// Reflect the profile, then reverse its traversal to keep
					// the original winding and round the arc-to-line corner.
					reversed := make([]trimmed.ExactCircularLoopSegment, len(ring))
					for i, s := range ring {
						s.Start, s.End = s.End, s.Start
						s.Start.X, s.End.X, s.Center.X = -s.Start.X, -s.End.X, -s.Center.X
						reversed[len(ring)-1-i] = s
					}
					ring, parent = reversed, len(ring)-1
				}
				source := trimmed.ExactCircularSweep{Outer: ring, Bottom: 0, Top: 2}
				before, err := trimmed.ExtrudeExactDirectedCircularLoop(ctx, ring, 0, 2, trimmed.ExactCircularLoopOptions{})
				if err != nil {
					t.Fatal(err)
				}
				got, err := CircularSweepCorners(ctx, source, []CircularSweepCorner{{Ring: 0, Corner: selection, Radius: 5}}, OrthogonalPrismOptions{})
				if err != nil {
					t.Fatal(err)
				}
				round := got.Profile.Outer[0]
				center, lineEnd, arcEnd := tc.center, tc.tangencyOnLine, tc.tangencyOnArc
				if swap {
					center.X, lineEnd.X, arcEnd.X = -center.X, -lineEnd.X, -arcEnd.X
					lineEnd, arcEnd = arcEnd, lineEnd
				}
				if round.Center != center || round.Start != lineEnd || round.End != arcEnd || round.Radius != 5 ||
					got.Profile.Outer[parent+1].Center != ring[parent].Center || got.Profile.Outer[parent+1].Radius != tc.arcRadius {
					t.Fatalf("round or original support changed: %+v", round)
				}
				interval := got.Original[0][parent]
				expectLo, expectHi, expectMid := "1/21", "1", "4/9"
				if swap {
					// Reversing the original arc changes its stereographic
					// parameter by a fractional-linear map, not by 1-t.
					expectLo, expectHi, expectMid = "0", "2/3", "1/3"
				}
				x := big.NewRat(1, 2)
				m := interval.Map
				y := new(big.Rat).Quo(new(big.Rat).Add(m[0][0], new(big.Rat).Mul(m[0][1], x)),
					new(big.Rat).Add(m[1][0], new(big.Rat).Mul(m[1][1], x)))
				if interval.Minimum.RatString() != expectLo || interval.Maximum.RatString() != expectHi || y.RatString() != expectMid {
					t.Fatalf("wrong original arc law: [%s,%s] midpoint %s", interval.Minimum, interval.Maximum, y)
				}
				u, _ := y.Float64()
				oldRim, err1 := before.Edges()[parent].Curve.Eval(u)
				newRim, err2 := got.Solid.Edges()[interval.LowerRim].Curve.Eval(.5)
				if err1 != nil || err2 != nil || math.Hypot(oldRim.X-newRim.X, oldRim.Y-newRim.Y) > 1e-12 || oldRim.Z != newRim.Z {
					t.Fatalf("shortened rim fails continuous original parameter check: %+v %+v", oldRim, newRim)
				}

				cross := func(a, b geom.Vec2) float64 { return a.X*b.Y - a.Y*b.X }
				sub := func(a, b geom.Vec2) geom.Vec2 { return p(a.X-b.X, a.Y-b.Y) }
				angle := func(c, a, b geom.Vec2) float64 {
					x, y := sub(a, c), sub(b, c)
					return math.Atan2(cross(x, y), x.X*y.X+x.Y*y.Y)
				}
				l, a, v := tc.tangencyOnLine, tc.corner, tc.tangencyOnArc
				phi, psi := angle(tc.support, a, v), angle(tc.center, l, v)
				area := (cross(tc.center, sub(v, l)) + 25*psi - cross(l, a) - cross(tc.support, sub(v, a)) - tc.arcRadius*tc.arcRadius*phi) / 2
				perimeter := 5*math.Abs(psi) - math.Abs(a.X-l.X) - tc.arcRadius*math.Abs(phi)
				old, err1 := before.MassProperties(ctx, parametric.IntegrationOptions{})
				now, err2 := got.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
				if err1 != nil || err2 != nil || math.Abs(now.Volume-old.Volume-2*area) > 1e-7 ||
					math.Abs(now.Area-old.Area-2*(area+perimeter)) > 1e-7 {
					t.Fatalf("wrong signed line-arc material change: before %+v, after %+v, area %g perimeter %g", old, now, area, perimeter)
				}
				if bin := os.Getenv("PLAINSOLID_OCCT_CIRCULAR_LINE_ARC_FILLETS"); bin != "" && !swap {
					raw, err := exec.CommandContext(ctx, bin, tc.name).CombinedOutput()
					var ref struct{ VolumeChange, AreaChange float64 }
					if err != nil || json.Unmarshal(raw, &ref) != nil || math.Abs(now.Volume-old.Volume-ref.VolumeChange) > 1e-7 ||
						math.Abs(now.Area-old.Area-ref.AreaChange) > 1e-7 {
						t.Fatalf("pinned OCCT material differs: %+v, %v, %s", ref, err, raw)
					}
				}
				if tc.name == "internal" && !swap {
					got, err := CircularSweepCorners(ctx, source, []CircularSweepCorner{{Ring: 0, Corner: selection, Radius: 4}}, OrthogonalPrismOptions{})
					if !errors.Is(err, geom.ErrUnrepresentable) || len(got.Solid.Faces()) != 0 {
						t.Fatalf("irrational tangent was silently fit: %v", err)
					}
				}
			})
		}
	}
}
