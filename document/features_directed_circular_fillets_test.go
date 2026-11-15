package document

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestDirectedCircularCornerFilletRetainsNamedWallsThroughEdits(t *testing.T) {
	ctx := context.Background()
	p := func(x, y float64) geom.Vec2 { return geom.Vec2{X: x, Y: y} }
	outer := circularDocumentRing("stock", p(0, 0), p(9, 0), p(10, 1), p(10, 8), p(0, 8))
	outer[1].Curve.Center, outer[1].Curve.Radius = p(9, 1), 1
	recipe := &DirectedCircularExtrusionRecipe{
		Outer: outer,
		Holes: [][]NamedCircularProfileSegment{
			circularDocumentRing("square", p(4, 3), p(4, 5), p(6, 5), p(6, 3)),
			circularDocumentHole("round", 8, 5, .5),
		},
		Bottom: -1, Top: 1,
	}
	pose := geom.Affine{{0, 0, -1, 12}, {0, 1, 0, 3}, {1, 0, 0, 4}}
	rounds := []NamedCircularCornerRound{{Segment: "square-0", ArcID: "inner-round", Radius: .25}, {Segment: "stock-0", ArcID: "outer-round", Radius: .5}}
	d, err := New(State{LengthUnit: .001, Features: map[string]Feature{
		"source": {Kind: DirectedCircularExtrusionFeature, DirectedCircularExtrusion: recipe},
		"fillet": {Kind: DirectedCircularCornerFilletFeature, Input: "source", CircularCornerRounds: rounds},
		"placed": {Kind: TransformFeature, Input: "fillet", Transform: pose},
	}}, 4)
	if err != nil {
		t.Fatal(err)
	}
	rounds[1].Radius = 2
	check := func(d *Document, radius float64) {
		t.Helper()
		s, _ := d.Snapshot()
		out, err := s.EvaluateFeatures(ctx)
		if err != nil {
			t.Fatal(err)
		}
		solid := out.Trimmed["placed"]
		measure, err := solid.MassProperties(ctx, parametric.IntegrationOptions{})
		footChange := (radius*radius - .25*.25) * (1 - math.Pi/4)
		area := 234 + 3*math.Pi - 2*footChange + 2*(radius+.25)*(math.Pi/2-2)
		if err != nil || math.Abs(measure.Volume-(150-2*footChange)) > 1e-7 || math.Abs(measure.Area-area) > 1e-7 {
			t.Fatalf("wrong rounded material: %+v, %v", measure, err)
		}
		faces, edges, vertices := solid.Faces(), solid.Edges(), solid.Vertices()
		if len(faces) != 17 || len(faces[1].Holes) != 2 {
			t.Fatal("rounded feature lost an original wire")
		}
		for _, example := range []struct {
			name  string
			index int
			start geom.Vec2
		}{
			{"outer-round", 0, p(0, radius)},
			{"stock-0", 1, p(radius, 0)},
			{"stock-1", 2, p(9, 0)},
			{"inner-round", 6, p(4+.25, 3)},
			{"round-0", 11, p(8.5, 5)},
		} {
			j, err := s.ResolveDirectedCircularExtrusionFace(FeatureFace{"placed", "wall:" + example.name})
			if err != nil || j != 2+example.index {
				t.Fatalf("wrong wall for %s: %d %v", example.name, j, err)
			}
			e, err := s.ResolveDirectedCircularExtrusionEdge(CircularExtrusionEdge{"placed", example.name, CircularExtrusionUpperRim})
			if err != nil || e != 15+example.index {
				t.Fatalf("wrong rim for %s: %d %v", example.name, e, err)
			}
			found := false
			for _, use := range faces[j].Outer {
				found = found || use.Edge == e
			}
			want := pose.Apply(geom.Vec3{X: example.start.X, Y: example.start.Y, Z: 1})
			if !found || vertices[edges[e].Start] != want {
				t.Fatalf("wall %s lost its exact named rim start: %+v, want %+v", example.name, vertices[edges[e].Start], want)
			}
		}
	}
	check(d, .5)

	edit := d.Begin()
	edit.State.Features["fillet"].CircularCornerRounds[1].Radius = .375
	if err := edit.Commit(); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []NamedCircularCornerRound{
		{Segment: "stock-0", ArcID: "round-0", Radius: .375},
		{Segment: "stock-1", ArcID: "outer-round", Radius: .375},
		{Segment: "stock-0", ArcID: "outer-round", Radius: 5},
	} {
		edit = d.Begin()
		edit.State.Features["fillet"].CircularCornerRounds[1] = bad
		if err := edit.Commit(); err == nil {
			t.Fatalf("committed invalid named round: %+v", bad)
		}
	}
	check(d, .375)
	if err := d.Undo(); err != nil {
		t.Fatal(err)
	}
	check(d, .5)
	if err := d.Redo(); err != nil {
		t.Fatal(err)
	}
	var wire bytes.Buffer
	if err := d.Write(&wire); err != nil {
		t.Fatal(err)
	}
	reopened, err := Read(bytes.NewReader(wire.Bytes()), 0, 4)
	if err != nil {
		t.Fatal(err)
	}
	check(reopened, .375)
}
