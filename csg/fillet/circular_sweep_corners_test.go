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

func mixedCircularFilletRing(points ...geom.Vec2) []trimmed.ExactCircularLoopSegment {
	r := make([]trimmed.ExactCircularLoopSegment, len(points))
	for i := range r {
		r[i] = trimmed.ExactCircularLoopSegment{Start: points[i], End: points[(i+1)%len(r)]}
	}
	return r
}

func mixedCircularFilletHole(center geom.Vec2, r float64) []trimmed.ExactCircularLoopSegment {
	p := func(x, y float64) geom.Vec2 { return geom.Vec2{X: center.X + x, Y: center.Y + y} }
	a := mixedCircularFilletRing(p(r, 0), p(0, -r), p(-r, 0), p(0, r))
	for i := range a {
		a[i].Center, a[i].Radius = center, r
	}
	return a
}

func mixedCircularFilletFixture() trimmed.ExactCircularSweep {
	p := func(x, y float64) geom.Vec2 { return geom.Vec2{X: x, Y: y} }
	o := mixedCircularFilletRing(p(0, 0), p(9, 0), p(10, 1), p(10, 8), p(0, 8))
	o[1].Center, o[1].Radius = p(9, 1), 1
	square := mixedCircularFilletRing(p(4, 3), p(4, 5), p(6, 5), p(6, 3))
	return trimmed.ExactCircularSweep{Outer: o, Holes: [][]trimmed.ExactCircularLoopSegment{square, mixedCircularFilletHole(p(8, 5), .5)}, Bottom: -1, Top: 1}
}

func TestCircularSweepCornersRetainOriginalCurvedAndHoleSpans(t *testing.T) {
	ctx := context.Background()
	a := mixedCircularFilletFixture()
	out, err := CircularSweepCorners(ctx, a, []CircularSweepCorner{{Ring: 0, Corner: 0, Radius: .5}, {Ring: 1, Corner: 0, Radius: .25}}, OrthogonalPrismOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Rounds) != 2 || len(out.Profile.Holes) != 2 || len(out.Solid.Faces()) != 17 || out.Profile.Outer[2] != a.Outer[1] ||
		out.Profile.Holes[1][0] != a.Holes[1][0] || out.Original[0][0].Minimum.RatString() != "1/18" ||
		out.Original[0][4].Maximum.RatString() != "15/16" || out.Original[1][0].Minimum.RatString() != "1/8" {
		t.Fatal("lost original arc, hole, or exact shortened straight range")
	}
	for _, created := range out.Rounds {
		face := out.Solid.Faces()[created.Wall]
		seen := map[int]bool{}
		for _, u := range face.Outer {
			seen[u.Edge] = true
		}
		if !seen[created.LowerRim] || !seen[created.UpperRim] {
			t.Fatal("new radius lost original cap incidence")
		}
	}
	x := 1 - math.Pi/4
	m, err := out.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
	expectedArea := 234 + 3*math.Pi - .375*x + 1.5*(math.Pi/2-2)
	if err != nil || math.Abs(m.Volume-(150-.375*x)) > 1e-7 || math.Abs(m.Area-expectedArea) > 1e-7 {
		t.Fatalf("independent convex and hole rounding material mismatch: %+v %v", m, err)
	}
}

func TestCircularSweepCornersRejectsNearbyOriginalArc(t *testing.T) {
	ctx := context.Background()
	a := mixedCircularFilletFixture()
	a.Holes[1] = mixedCircularFilletHole(geom.Vec2{X: .1875, Y: .1875}, .125)
	selection := []CircularSweepCorner{{Ring: 0, Corner: 0, Radius: .5}}
	r, err := CircularSweepCorners(ctx, a, selection, OrthogonalPrismOptions{})
	if !errors.Is(err, ErrRadius) || len(r.Solid.Faces()) != 0 {
		t.Fatalf("published a round whose influence reaches an original hole: %v", err)
	}
	a = mixedCircularFilletFixture()
	r, err = CircularSweepCorners(ctx, a, selection, OrthogonalPrismOptions{MaxWork: 1})
	if !errors.Is(err, ErrLimit) || len(r.Solid.Faces()) != 0 {
		t.Fatalf("published round after exhausted exact clearance proof: %v", err)
	}
}

func TestCircularSweepCornersLiveOCCT(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_CIRCULAR_MIXED_FILLETS")
	if bin == "" {
		t.Skip("optional pinned OCCT mixed orthogonal fillets")
	}
	raw, err := exec.Command(bin).CombinedOutput()
	var d struct{ Volume, Area float64 }
	x := 1 - math.Pi/4
	area := 234 + 3*math.Pi - .375*x + 1.5*(math.Pi/2-2)
	if err != nil || json.Unmarshal(raw, &d) != nil || math.Abs(d.Volume-(150-.375*x)) > 1e-7 || math.Abs(d.Area-area) > 1e-7 {
		t.Fatalf("independent mixed fillet material mismatch: %+v %v %s", d, err, raw)
	}
}
