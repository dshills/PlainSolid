package document

import (
	"bytes"
	"context"
	"errors"
	"math"
	"plainsolid/internal/slowtest"
	"reflect"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestDirectedCircularInsetPreservesOriginalMemberRoles(t *testing.T) {
	slowtest.Skip(t)
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
	d, e := New(State{LengthUnit: .001, Nodes: map[string]Node{
		"group":     {Transform: geom.Affine{{1, 0, 0, 5}, {0, 1, 0, -1}, {0, 0, 1, 0}}},
		"step-part": {Name: "placed inset", Parent: "group", Asset: "placed", Transform: geom.Affine{{1, 0, 0, 3}, {0, 1, 0, 4}, {0, 0, 1, 5}}},
		"iges-part": {Parent: "group", Asset: "joined", Transform: geom.Affine{{0, -1, 0, 1}, {1, 0, 0, -2}, {0, 0, 1, 3}}},
	}, Features: map[string]Feature{
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
	snapshot, _ := reopened.Snapshot()
	var sb, ib bytes.Buffer
	if e = snapshot.WriteCircularInsetSTEP(ctx, &sb, "step-part"); e != nil {
		t.Fatal(e)
	}
	a, e := step.ReadExactCircularSweepInset(ctx, &sb, step.Limits{})
	if e != nil || a.Descriptor.Name != "placed inset" || a.Descriptor.LengthUnit != .001 ||
		a.Descriptor.Frame.Origin() != (geom.Vec3{X: 18, Y: 5, Z: 5}) ||
		a.Descriptor.Frame.ZAxis() != (geom.Vec3{X: -1}) || a.Descriptor.Guest.Holes[0][0].Radius != .75 {
		t.Fatalf("STEP lost signed feature and assembly placement: %+v %v", a.Descriptor, e)
	}
	if e = snapshot.WriteCircularInsetIGES(ctx, &ib, "iges-part"); e != nil {
		t.Fatal(e)
	}
	b, e := iges.ReadExactCircularSweepInset(ctx, &ib, iges.Limits{})
	if e != nil || b.Descriptor.Unit != .001 || b.Descriptor.Host.Bottom != 1 || b.Descriptor.Guest.Top != 6 ||
		b.Descriptor.Guest.Holes[0][0].Center != (geom.Vec2{X: 3, Y: 1}) || b.Descriptor.Guest.Holes[0][0].Radius != .75 {
		t.Fatalf("IGES lost original placed cap and ring geometry: %+v %v", b.Descriptor, e)
	}
	ib.Reset()
	if e = snapshot.WriteCircularInsetIGES(ctx, &ib, "step-part"); !errors.Is(e, geom.ErrDomain) || ib.Len() != 0 {
		t.Fatalf("IGES accepted nonaxial placement: %v", e)
	}
	group := snapshot.Nodes["group"]
	group.Transform[0][3] = 1e16
	snapshot.Nodes["group"] = group
	sb.Reset()
	if e = snapshot.WriteCircularInsetSTEP(ctx, &sb, "step-part"); !errors.Is(e, geom.ErrUnrepresentable) || sb.Len() != 0 {
		t.Fatalf("STEP rounded accumulated node placement: %v", e)
	}
	if e = d.Undo(); e != nil {
		t.Fatal(e)
	}
	check(d, .5)
}
