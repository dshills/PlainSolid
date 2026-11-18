package trimmed

import (
	"context"
	"encoding/json"
	"math"
	"math/big"
	"os"
	"os/exec"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func circularSweepOverlayFixture() (ExactCircularSweep, ExactCircularSweep) {
	p := func(x, y float64) geom.Vec2 { return geom.Vec2{X: x, Y: y} }
	outer := exactLoopQuarterDisk(geom.Vec2{}, 5, false)
	o1, o2 := outer[0], outer[0]
	o1.End, o2.Start = p(3, 4), p(3, 4)
	splitOuter := append([]ExactCircularLoopSegment{o1, o2}, outer[1:]...)
	hole := exactLoopPolygon(p(-1, -1), p(-1, 1), p(1, 1), p(1, -1))
	h1, h2 := hole[0], hole[0]
	h1.End, h2.Start = p(-1, 0), p(-1, 0)
	splitHole := append([]ExactCircularLoopSegment{h1, h2}, hole[1:]...)
	return ExactCircularSweep{Outer: outer, Holes: [][]ExactCircularLoopSegment{splitHole}, Bottom: -2, Top: 0},
		ExactCircularSweep{Outer: splitOuter, Holes: [][]ExactCircularLoopSegment{hole}, Bottom: 0, Top: 3}
}

func TestExactCircularSweepCapOverlayPreservesBothSourceParameterMaps(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a, b := circularSweepOverlayFixture()
	got, err := JoinExactCircularSweepCapOverlay(ctx, a, b, ExactCircularLoopOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.SharedRimEdges) != 10 || len(got.Solid.Faces()) != 22 || len(got.Solid.Edges()) != 50 || len(got.Solid.Vertices()) != 30 ||
		len(got.Faces[0][2]) != 2 || len(got.Faces[1][2+5]) != 2 || len(got.Faces[0][1]) != 0 || len(got.Faces[1][0]) != 0 {
		t.Fatal("did not keep both original parent identities and common segmentation")
	}
	aArc, bLine := got.Edges[0][9], got.Edges[1][5]
	if len(aArc) != 2 || len(bLine) != 2 {
		t.Fatal("original rim did not own both child edges")
	}
	for _, parts := range [][]CircularSweepEdgeFragment{aArc, bLine} {
		if parts[0].Minimum.Sign() != 0 || parts[0].Maximum.RatString() != "1/2" ||
			parts[1].Minimum.RatString() != "1/2" || parts[1].Maximum.RatString() != "1" {
			t.Fatalf("wrong ordered source ranges: %+v", parts)
		}
	}
	if aArc[1].Map[1][1].RatString() != "-1/6" || bLine[1].Map[1][1].Sign() != 0 {
		t.Fatal("original curved pcurve law was flattened")
	}
	half := big.NewRat(1, 2)
	p := aArc[1].Map
	s := new(big.Rat).Quo(new(big.Rat).Add(p[0][0], new(big.Rat).Mul(p[0][1], half)), new(big.Rat).Add(p[1][0], new(big.Rat).Mul(p[1][1], half)))
	if s.RatString() != "8/11" {
		t.Fatalf("wrong original raw midpoint %s", s)
	}
	h, err := ExtrudeExactDirectedCircularLoops(ctx, a.Outer, a.Holes, a.Bottom, a.Top, ExactCircularLoopOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sample, err := got.Solid.Edges()[aArc[1].Edge].Curve.Eval(.5)
	x, _ := s.Float64()
	original, oldErr := h.Edges()[9].Curve.Eval(x)
	want := geom.Vec3{X: 57. / 37, Y: 176. / 37}
	if err != nil || oldErr != nil || !closePoint(sample, want, 1e-12) || !closePoint(original, want, 1e-12) {
		t.Fatalf("fractional-linear source parameter misses shared geometry: %+v / %+v %v %v", sample, original, err, oldErr)
	}
	props, err := got.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil || math.Abs(props.Volume-(125*math.Pi-20)) > 1e-7 || math.Abs(props.Area-(100*math.Pi+32)) > 1e-7 {
		t.Fatalf("wrong material from offset rim partitions: %+v %v", props, err)
	}
	mesh, err := got.Solid.Tessellate(ctx, parametric.TessellationOptions{Deflection: .2, Angle: .4, MaxSubdivisions: 16, MaxTriangles: 2000})
	if err != nil {
		t.Fatal(err)
	}
	if err = mesh.ValidateSolid(); err != nil {
		t.Fatal(err)
	}
}

func TestExactCircularSweepCapOverlayRejectsDifferentCurve(t *testing.T) {
	ctx := context.Background()
	a, b := circularSweepOverlayFixture()
	b.Outer[0].Center = geom.Vec2{X: 8, Y: 4}
	if _, err := ExtrudeExactDirectedCircularLoops(ctx, b.Outer, b.Holes, b.Bottom, b.Top, ExactCircularLoopOptions{}); err != nil {
		t.Fatalf("changed source is not an independent valid solid: %v", err)
	}
	if got, err := JoinExactCircularSweepCapOverlay(ctx, a, b, ExactCircularLoopOptions{}); err == nil || len(got.Solid.Faces()) != 0 {
		t.Fatalf("published a join of different exact arcs: %v", err)
	}
}

func TestExactCircularSweepCapOverlayLiveOCCT(t *testing.T) {
	bin := os.Getenv("PLAINSOLID_OCCT_CIRCULAR_SWEEP_INSET")
	if bin == "" {
		t.Skip("optional pinned OCCT cap overlay reference")
	}
	raw, err := exec.Command(bin, "overlay").CombinedOutput()
	var p struct {
		Solids       int
		Volume, Area float64
	}
	if err != nil || json.Unmarshal(raw, &p) != nil || p.Solids != 1 ||
		math.Abs(p.Volume-(125*math.Pi-20)) > 1e-7 || math.Abs(p.Area-(100*math.Pi+32)) > 1e-7 {
		t.Fatalf("independent cap overlay material differs: %+v %v %s", p, err, raw)
	}
}
