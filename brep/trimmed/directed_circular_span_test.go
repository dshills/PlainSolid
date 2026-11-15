package trimmed

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestSplitExactDirectedCircularSpanMajorHalfAndFull(t *testing.T) {
	ctx := context.Background()
	p := func(x, y float64) geom.Vec2 { return geom.Vec2{X: x, Y: y} }
	live := func(mode string, signed parametric.Properties) {
		t.Helper()
		bin := os.Getenv("PLAINSOLID_OCCT_DIRECTED_CIRCULAR_SPAN")
		if bin == "" {
			return
		}
		raw, err := exec.CommandContext(ctx, bin, mode).CombinedOutput()
		var reference struct{ Volume, Area float64 }
		if err != nil || json.Unmarshal(raw, &reference) != nil || math.Abs(signed.Volume-reference.Volume) > 1e-7 ||
			math.Abs(signed.Area-reference.Area) > 1e-7 {
			t.Fatalf("independent OCCT %s span disagrees: %+v %v %s", mode, reference, err, raw)
		}
	}
	major, err := SplitExactDirectedCircularSpan(ExactDirectedCircularSpan{Start: p(5, 0), End: p(0, -5), Radius: 5})
	if err != nil || len(major) != 3 || major[0].End != p(0, 5) || major[1].End != p(-5, 0) || major[2].End != p(0, -5) {
		t.Fatalf("wrong major-arc phase order: %+v %v", major, err)
	}
	major = append(major, ExactCircularLoopSegment{Start: p(0, -5), End: p(5, 0)})
	solid, err := ExtrudeExactDirectedCircularLoop(ctx, major, 0, 2, ExactCircularLoopOptions{})
	if err != nil {
		t.Fatal(err)
	}
	m, err := solid.MassProperties(ctx, parametric.IntegrationOptions{})
	base := 18.75*math.Pi + 12.5
	if err != nil || math.Abs(m.Volume-2*base) > 1e-7 || math.Abs(m.Area-(2*base+2*(7.5*math.Pi+5*math.Sqrt2))) > 1e-7 {
		t.Fatalf("major arc was replaced by its complementary sector: %+v %v", m, err)
	}
	live("major", m)

	half, err := SplitExactDirectedCircularSpan(ExactDirectedCircularSpan{Start: p(3, 4), End: p(-3, -4), Radius: 5, Clockwise: true})
	if err != nil || len(half) != 3 || half[0].End != p(5, 0) || half[1].End != p(0, -5) || half[2].End != p(-3, -4) {
		t.Fatalf("half-circle direction not retained: %+v %v", half, err)
	}
	full, err := SplitExactDirectedCircularSpan(ExactDirectedCircularSpan{
		Start: p(3, 4), End: p(3, 4), Radius: 5, Clockwise: true, FullCircle: true,
	})
	if err != nil || len(full) != 5 || full[0].End != p(5, 0) || full[1].End != p(0, -5) ||
		full[2].End != p(-5, 0) || full[3].End != p(0, 5) || full[4].End != p(3, 4) {
		t.Fatalf("full circle did not retain its authored starting point: %+v %v", full, err)
	}
	outer := exactLoopPolygon(p(-8, -8), p(8, -8), p(8, 8), p(-8, 8))
	withHole, err := ExtrudeExactDirectedCircularLoops(ctx, outer, [][]ExactCircularLoopSegment{full}, -1, 1, ExactCircularLoopOptions{})
	if err != nil {
		t.Fatal(err)
	}
	m, err = withHole.MassProperties(ctx, parametric.IntegrationOptions{})
	base = 256 - 25*math.Pi
	if err != nil || math.Abs(m.Volume-2*base) > 1e-7 || math.Abs(m.Area-(2*base+2*(64+10*math.Pi))) > 1e-7 ||
		len(withHole.Faces()[0].Holes) != 1 || len(withHole.Faces()[0].Holes[0]) != 5 {
		t.Fatalf("closed authored bore lost material or split ownership: %+v %v", m, err)
	}
	live("full", m)
}

func TestSplitExactDirectedCircularSpanChecksOnlyTraversedStations(t *testing.T) {
	const x = 1 << 53
	s := ExactDirectedCircularSpan{Start: geom.Vec2{X: x + 4, Y: 3}, End: geom.Vec2{X: x - 4, Y: 3}, Center: geom.Vec2{X: x}, Radius: 5}
	cuts, err := SplitExactDirectedCircularSpan(s)
	if err != nil || len(cuts) != 2 || cuts[0].End != (geom.Vec2{X: x, Y: 5}) {
		t.Fatalf("rejected a short exact span because of an unrelated unrepresentable axis: %+v %v", cuts, err)
	}
	s.Clockwise = true
	if cuts, err := SplitExactDirectedCircularSpan(s); !errors.Is(err, geom.ErrUnrepresentable) || cuts != nil {
		t.Fatalf("published an inexact station on the long span: %+v %v", cuts, err)
	}
	// Exact nonaxis stations still cover this long directed span.
	a, b := geom.Vec2{X: x + 4, Y: -3}, geom.Vec2{X: x - 4, Y: -3}
	cuts, err = PartitionExactDirectedCircularSpan(s, []geom.Vec2{a, b})
	if err != nil || len(cuts) != 3 || cuts[0].End != a || cuts[1].End != b || cuts[2].End != s.End {
		t.Fatalf("lost the original span despite exact interior stations: %+v %v", cuts, err)
	}
	for _, bad := range [][]geom.Vec2{{b, a}, {a}, {a, {X: x - 4, Y: -2}}} {
		if cuts, err := PartitionExactDirectedCircularSpan(s, bad); !errors.Is(err, geom.ErrDomain) || cuts != nil {
			t.Fatalf("accepted an out-of-order, insufficient or off-circle station: %+v %v", cuts, err)
		}
	}
	s.End = s.Start
	if cuts, err := SplitExactDirectedCircularSpan(s); !errors.Is(err, geom.ErrInvalid) || cuts != nil {
		t.Fatalf("equal endpoints automatically became a full circle: %+v %v", cuts, err)
	}
}
