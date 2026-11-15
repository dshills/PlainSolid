package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func exactLoopPolygon(points ...geom.Vec2) []ExactCircularLoopSegment {
	r := make([]ExactCircularLoopSegment, len(points))
	for i, p := range points {
		r[i] = ExactCircularLoopSegment{Start: p, End: points[(i+1)%len(points)]}
	}
	return r
}

func exactLoopQuarterDisk(center geom.Vec2, radius float64, clockwise bool) []ExactCircularLoopSegment {
	p := [4]geom.Vec2{{X: center.X + radius, Y: center.Y}, {X: center.X, Y: center.Y + radius},
		{X: center.X - radius, Y: center.Y}, {X: center.X, Y: center.Y - radius}}
	if clockwise {
		p[1], p[3] = p[3], p[1]
	}
	q := exactLoopPolygon(p[:]...)
	for i := range q {
		q[i].Center, q[i].Radius = center, radius
	}
	return q
}

func TestExtrudeExactDirectedCircularLoopsRetainsIndependentHoleCharts(t *testing.T) {
	ctx := context.Background()
	outer := exactLoopPolygon(geom.Vec2{}, geom.Vec2{X: 6}, geom.Vec2{X: 6, Y: 4}, geom.Vec2{Y: 4})
	holes := [][]ExactCircularLoopSegment{exactLoopQuarterDisk(geom.Vec2{X: 2, Y: 2}, .5, true), exactLoopQuarterDisk(geom.Vec2{X: 4, Y: 2}, .5, true)}
	s, err := ExtrudeExactDirectedCircularLoops(ctx, outer, holes, -1, 1, ExactCircularLoopOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Faces()) != 14 || len(s.Edges()) != 36 || len(s.Vertices()) != 24 {
		t.Fatal("separate circular cap and wall topology was lost")
	}
	faces := s.Faces()
	for cap := 0; cap < 2; cap++ {
		f := faces[cap]
		if len(f.Holes) != 2 || len(f.Holes[0]) != 4 || len(f.Holes[1]) != 4 {
			t.Fatal("inner wires lost source partition", cap)
		}
		for i, p := range []geom.Vec2{{X: 2, Y: 2}, {X: 4, Y: 2}, {X: 3, Y: 2}} {
			got, proof, err := LocateInFaceTrim(ctx, f, p, trimvalidate.Options{})
			want := []trimvalidate.Location{trimvalidate.Outside, trimvalidate.Outside, trimvalidate.Inside}[i]
			if err != nil || proof.Status != trimvalidate.Certified || got != want {
				t.Fatalf("cap %d at %v, got %v %v %v", cap, p, got, proof, err)
			}
		}
	}
	properties, err := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil || math.Abs(properties.Volume-(48-math.Pi)) > 1e-7 || math.Abs(properties.Area-(88+3*math.Pi)) > 1e-7 {
		t.Fatalf("wall/cap signed measures disagree with the two independent circles: %+v %v", properties, err)
	}
	var raw bytes.Buffer
	if err := WriteJSON(&raw, s); err != nil {
		t.Fatal(err)
	}
	reopened, err := ReadJSONContext(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	var after bytes.Buffer
	if err := WriteJSON(&after, reopened); err != nil || !bytes.Equal(raw.Bytes(), after.Bytes()) {
		t.Fatal("native rewrite changed original circular hole curves", err)
	}
}

func TestExtrudeExactDirectedCircularLoopsRejectWrongWindingAndWireContact(t *testing.T) {
	ctx := context.Background()
	outer := exactLoopPolygon(geom.Vec2{}, geom.Vec2{X: 6}, geom.Vec2{X: 6, Y: 4}, geom.Vec2{Y: 4})
	a := exactLoopQuarterDisk(geom.Vec2{X: 2, Y: 2}, .5, true)
	for _, holes := range [][][]ExactCircularLoopSegment{
		{exactLoopQuarterDisk(geom.Vec2{X: 2, Y: 2}, .5, false)},
		{a, exactLoopQuarterDisk(geom.Vec2{X: 3, Y: 2}, .5, true)},
		{a, exactLoopQuarterDisk(geom.Vec2{X: 2, Y: 2}, .25, true)},
		{exactLoopQuarterDisk(geom.Vec2{X: .5, Y: 2}, .5, true)},
	} {
		if s, err := ExtrudeExactDirectedCircularLoops(ctx, outer, holes, 0, 1, ExactCircularLoopOptions{}); err == nil || len(s.Faces()) != 0 {
			t.Fatalf("unproved inner wire returned material: %v", err)
		}
	}
}

func TestExtrudeExactDirectedCircularLoopsLiveOCCT(t *testing.T) {
	oracle := os.Getenv("PLAINSOLID_OCCT_DIRECTED_CIRCULAR_LOOPS")
	if oracle == "" {
		t.Skip("optional pinned OCCT circular-hole reference")
	}
	data, err := exec.Command(oracle).CombinedOutput()
	var expected struct {
		Faces        int
		Volume, Area float64
	}
	if err != nil || json.Unmarshal(data, &expected) != nil {
		t.Fatalf("independent OCCT circular-hole reference: %v %s", err, data)
	}
	ctx := context.Background()
	outer := exactLoopPolygon(geom.Vec2{}, geom.Vec2{X: 6}, geom.Vec2{X: 6, Y: 4}, geom.Vec2{Y: 4})
	holes := [][]ExactCircularLoopSegment{exactLoopQuarterDisk(geom.Vec2{X: 2, Y: 2}, .5, true), exactLoopQuarterDisk(geom.Vec2{X: 4, Y: 2}, .5, true)}
	s, err := ExtrudeExactDirectedCircularLoops(ctx, outer, holes, -1, 1, ExactCircularLoopOptions{})
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.MassProperties(ctx, parametric.IntegrationOptions{})
	if err != nil || len(s.Faces()) != expected.Faces || math.Abs(p.Volume-expected.Volume) > 1e-7 || math.Abs(p.Area-expected.Area) > 1e-7 {
		t.Fatalf("original analytical circular-hole model changed: OCCT %+v Go %+v %v", expected, p, err)
	}
}
