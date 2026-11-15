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

func TestCircularSweepArcPairCornersRetainBothOriginalCircles(t *testing.T) {
	ctx := context.Background()
	p := func(x, y float64) geom.Vec2 { return geom.Vec2{X: x, Y: y} }
	for _, tc := range []struct {
		name string
		arc  [2]trimmed.ExactCircularLoopSegment
		rest []geom.Vec2
		span [2]string
	}{
		{"arc-pair", [2]trimmed.ExactCircularLoopSegment{
			{Start: p(-1.5, -6.5), End: p(4.5, 2.5), Center: p(-1.5, 0), Radius: 6.5},
			{Start: p(4.5, 2.5), End: p(-1.5, 5.5), Center: p(-1.5, -2), Radius: 7.5},
		}, []geom.Vec2{p(-12, 5.5), p(-12, -6.5)}, [2]string{"2/3", "2/7"}},
		{"mixed-arc-pair", [2]trimmed.ExactCircularLoopSegment{
			{Start: p(-1.25, -6.25), End: p(3.75, 3.75), Center: p(-1.25, 0), Radius: 6.25},
			{Start: p(3.75, 3.75), End: p(2.5, 5), Center: p(3.75, 5), Radius: 1.25},
		}, []geom.Vec2{p(2.5, 10), p(-12, 10), p(-12, -6.25)}, [2]string{"1/2", "1/3"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ring := mixedCircularFilletRing(append([]geom.Vec2{tc.arc[0].Start, tc.arc[0].End, tc.arc[1].End}, tc.rest...)...)
			ring[0], ring[1] = tc.arc[0], tc.arc[1]
			source := trimmed.ExactCircularSweep{Outer: ring, Top: 2}
			rounds := []CircularSweepCorner{{Ring: 0, Corner: 1, Radius: 5}}
			got, err := CircularSweepCorners(ctx, source, rounds, OrthogonalPrismOptions{})
			if err != nil {
				t.Fatal(err)
			}
			a, b := p(5, 0), p(3, 4)
			newArc := trimmed.ExactCircularLoopSegment{Start: a, End: b, Radius: 5}
			originalIn, originalOut := tc.arc[0], tc.arc[1]
			originalIn.End, originalOut.Start = a, b
			if got.Profile.Outer[0] != originalIn || got.Profile.Outer[1] != newArc || got.Profile.Outer[2] != originalOut {
				t.Fatalf("original circles or selected branch changed: %+v", got.Profile.Outer[:3])
			}
			in, out := got.Original[0][0], got.Original[0][1]
			if in.Maximum.RatString() != tc.span[0] || out.Minimum.RatString() != tc.span[1] || out.Map[1][1].Sign() == 0 {
				t.Fatalf("wrong original curved intervals %s %s", in.Maximum, out.Minimum)
			}
			before, err := trimmed.ExtrudeExactDirectedCircularLoop(ctx, ring, 0, 2, trimmed.ExactCircularLoopOptions{})
			if err != nil {
				t.Fatal(err)
			}
			for i, edge := range []CircularSweepCornerEdge{in, out} {
				m, h := edge.Map, big.NewRat(1, 2)
				u := new(big.Rat).Quo(new(big.Rat).Add(m[0][0], new(big.Rat).Mul(m[0][1], h)),
					new(big.Rat).Add(m[1][0], new(big.Rat).Mul(m[1][1], h)))
				f, _ := u.Float64()
				old, err0 := before.Edges()[i].Curve.Eval(f)
				new, err1 := got.Solid.Edges()[edge.LowerRim].Curve.Eval(.5)
				if err0 != nil || err1 != nil || math.Hypot(old.X-new.X, old.Y-new.Y) > 1e-12 {
					t.Fatalf("child %d no longer lies on the exact original clock", i)
				}
			}
			cross := func(a, b geom.Vec2) float64 { return a.X*b.Y - a.Y*b.X }
			properties := func(s trimmed.ExactCircularLoopSegment) (signedArea, length float64) {
				x, y := p(s.Start.X-s.Center.X, s.Start.Y-s.Center.Y), p(s.End.X-s.Center.X, s.End.Y-s.Center.Y)
				w := math.Atan2(cross(x, y), x.X*y.X+x.Y*y.Y)
				return (cross(s.Center, p(s.End.X-s.Start.X, s.End.Y-s.Start.Y)) + s.Radius*s.Radius*w) / 2, s.Radius * math.Abs(w)
			}
			removedIn, removedOut := tc.arc[0], tc.arc[1]
			removedIn.Start, removedOut.End = a, b
			a0, l0 := properties(removedIn)
			a1, l1 := properties(removedOut)
			an, ln := properties(newArc)
			expectedV, expectedA := 2*(an-a0-a1), 2*(an-a0-a1+ln-l0-l1)
			old, err0 := before.MassProperties(ctx, parametric.IntegrationOptions{})
			new, err1 := got.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
			changeV, changeA := new.Volume-old.Volume, new.Area-old.Area
			if err0 != nil || err1 != nil || math.Abs(changeV-expectedV) > 1e-7 || math.Abs(changeA-expectedA) > 1e-7 {
				t.Fatalf("measured changes (%g, %g), want (%g, %g)", changeV, changeA, expectedV, expectedA)
			}
			if bin := os.Getenv("PLAINSOLID_OCCT_CIRCULAR_LINE_ARC_FILLETS"); bin != "" {
				raw, err := exec.CommandContext(ctx, bin, tc.name).CombinedOutput()
				var ref struct{ VolumeChange, AreaChange float64 }
				if err != nil || json.Unmarshal(raw, &ref) != nil || math.Abs(changeV-ref.VolumeChange) > 1e-7 || math.Abs(changeA-ref.AreaChange) > 1e-7 {
					t.Fatalf("native changes (%g, %g) differ from pinned OCCT: %+v, %v, %s", changeV, changeA, ref, err, raw)
				}
			}
			if tc.name == "arc-pair" {
				rounds[0].Radius = 4
				failed, err := CircularSweepCorners(ctx, source, rounds, OrthogonalPrismOptions{})
				if !errors.Is(err, geom.ErrUnrepresentable) || len(failed.Solid.Faces()) != 0 {
					t.Fatalf("irrational center produced material: %v", err)
				}
			}
		})
	}
}
