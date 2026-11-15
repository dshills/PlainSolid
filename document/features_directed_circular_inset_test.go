package document

import (
	"bytes"
	"context"
	"math"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestDirectedCircularInsetPreservesOriginalMemberRoles(t *testing.T) {
	ctx := context.Background()
	p := func(x, y float64) geom.Vec2 { return geom.Vec2{X: x, Y: y} }
	host := &DirectedCircularExtrusionRecipe{
		Outer:  circularDocumentRing("stock", p(0, 0), p(8, 0), p(8, 6), p(0, 6)),
		Holes:  [][]NamedCircularProfileSegment{circularDocumentHole("existing", 1, 1, .5)},
		Bottom: -2, Top: 0,
	}
	// The same circle helper makes a clockwise inner loop. Reverse it to
	// obtain the guest's CCW exterior, keeping each segment's direction.
	outer := circularDocumentHole("boss", 4, 3, 2)
	for i, j := 0, len(outer)-1; i < j; i, j = i+1, j-1 {
		outer[i], outer[j] = outer[j], outer[i]
	}
	for i := range outer {
		outer[i].Curve.Start, outer[i].Curve.End = outer[i].Curve.End, outer[i].Curve.Start
	}
	guest := &DirectedCircularExtrusionRecipe{
		Outer: outer, Holes: [][]NamedCircularProfileSegment{circularDocumentHole("opening", 4, 3, .5)},
		Bottom: 0, Top: 3,
	}
	pose := geom.Affine{{0, 0, -1, 10}, {0, 1, 0, 2}, {1, 0, 0, 0}}
	d, e := New(State{LengthUnit: .001, Features: map[string]Feature{
		"plate":  {Kind: DirectedCircularExtrusionFeature, DirectedCircularExtrusion: host},
		"boss":   {Kind: DirectedCircularExtrusionFeature, DirectedCircularExtrusion: guest},
		"joined": {Kind: DirectedCircularInsetJoinFeature, Input: "plate", Tool: "boss"},
		"placed": {Kind: TransformFeature, Input: "joined", Transform: pose},
	}}, 2)
	if e != nil {
		t.Fatal(e)
	}
	check := func(d *Document, r float64) {
		t.Helper()
		s, _ := d.Snapshot()
		m, e := s.EvaluateFeatures(ctx)
		if e != nil {
			t.Fatal(e)
		}
		solid := m.Trimmed["placed"]
		properties, e := solid.MassProperties(ctx, parametric.IntegrationOptions{})
		if e != nil || math.Abs(properties.Volume-(96+(11.5-3*r*r)*math.Pi)) > 1e-7 {
			t.Fatalf("wrong joined material %+v %v", properties, e)
		}
		fragments, e := s.ResolveCircularInsetFaces(CircularInsetFace{"placed", CircularInsetHost, "upper-cap"})
		if e != nil || !reflect.DeepEqual(fragments, []int{1, 19}) {
			t.Fatalf("lost original host cap fragments: %v %v", fragments, e)
		}
		removed, e := s.ResolveCircularInsetFaces(CircularInsetFace{"placed", CircularInsetGuest, "lower-cap"})
		if e != nil || len(removed) > 0 {
			t.Fatalf("guest interface was preserved as a material face: %v %v", removed, e)
		}
		edge, e := s.ResolveCircularInsetEdge(CircularInsetEdge{"placed", "opening-0", CircularInsetGuest, CircularExtrusionLowerRim})
		if e != nil || edge != 28 {
			t.Fatalf("original guest floor rim identity: %d %v", edge, e)
		}
		faces, edges, vertices := solid.Faces(), solid.Edges(), solid.Vertices()
		owned := false
		for _, u := range faces[fragments[1]].Outer {
			owned = owned || u.Edge == edge
		}
		if !owned || vertices[edges[edge].Start] != pose.Apply(geom.Vec3{X: 4 + r, Y: 3}) {
			t.Fatal("floor did not retain the original guest rim")
		}
	}
	check(d, .5)
	tx := d.Begin()
	tx.State.Features["boss"].DirectedCircularExtrusion.Holes[0] = circularDocumentHole("opening", 4, 3, .75)
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	check(d, .75)
	tx = d.Begin()
	tx.State.Features["boss"].DirectedCircularExtrusion.Bottom = .125
	if e = tx.Commit(); e == nil {
		t.Fatal("committed a false gap as interface")
	}
	check(d, .75)
	var raw bytes.Buffer
	if e = d.Write(&raw); e != nil {
		t.Fatal(e)
	}
	reopened, e := Read(bytes.NewReader(raw.Bytes()), 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	check(reopened, .75)
	if e = d.Undo(); e != nil {
		t.Fatal(e)
	}
	check(d, .5)
}
