package document

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func circularDocumentRing(name string, p ...geom.Vec2) []NamedCircularProfileSegment {
	z := make([]NamedCircularProfileSegment, len(p))
	for i := range p {
		z[i] = NamedCircularProfileSegment{fmt.Sprintf("%s-%d", name, i), trimmed.ExactCircularLoopSegment{Start: p[i], End: p[(i+1)%len(p)]}}
	}
	return z
}

func circularDocumentHole(name string, x, y, r float64) []NamedCircularProfileSegment {
	p := func(a, b float64) geom.Vec2 { return geom.Vec2{X: a, Y: b} }
	h := circularDocumentRing(name, p(x+r, y), p(x, y-r), p(x-r, y), p(x, y+r))
	for i := range h {
		h[i].Curve.Center, h[i].Curve.Radius = p(x, y), r
	}
	return h
}

func TestDirectedCircularExtrusionNamedRolesSurviveEdits(t *testing.T) {
	ctx := context.Background()
	p := func(x, y float64) geom.Vec2 { return geom.Vec2{X: x, Y: y} }
	r := &DirectedCircularExtrusionRecipe{
		Outer:  circularDocumentRing("stock", p(0, 0), p(6, 0), p(6, 4), p(0, 4)),
		Holes:  [][]NamedCircularProfileSegment{circularDocumentHole("west", 2, 2, .5), circularDocumentHole("east", 4, 2, .5)},
		Bottom: -1, Top: 1,
	}
	pose := geom.Affine{{0, 0, -1, 10}, {0, 1, 0, 2}, {1, 0, 0, 0}}
	d, err := New(State{LengthUnit: .001, Features: map[string]Feature{
		"part":  {Kind: DirectedCircularExtrusionFeature, DirectedCircularExtrusion: r},
		"posed": {Kind: TransformFeature, Input: "part", Transform: pose},
	}}, 2)
	if err != nil {
		t.Fatal(err)
	}
	// The authored input and client snapshots cannot mutate committed state.
	r.Holes[0][0].ID = "caller-modified"
	detached, _ := d.Snapshot()
	detached.Features["part"].DirectedCircularExtrusion.Holes[0][0].ID = "snapshot-modified"

	check := func(doc *Document, radius, top float64, wantIndex int) {
		t.Helper()
		s, _ := doc.Snapshot()
		native, err := s.EvaluateFeatures(ctx)
		if err != nil {
			t.Fatal(err)
		}
		solid := native.Trimmed["posed"]
		m, err := solid.MassProperties(ctx, parametric.IntegrationOptions{})
		foot := 24 - math.Pi*(radius*radius+.25)
		h := top + 1
		if err != nil || math.Abs(m.Volume-h*foot) > 1e-8 ||
			math.Abs(m.Area-(2*foot+h*(20+2*math.Pi*(radius+.5)))) > 1e-8 {
			t.Fatalf("wrong independently measured material: %+v %v", m, err)
		}
		fs := solid.Faces()
		for i, role := range []string{"lower-cap", "upper-cap"} {
			j, e := s.ResolveDirectedCircularExtrusionFace(FeatureFace{"posed", role})
			if e != nil || j != i || len(fs[j].Holes) != 2 {
				t.Fatalf("%s lost independent original hole wires: %d %v", role, j, e)
			}
		}
		wall, err := s.ResolveDirectedCircularExtrusionFace(FeatureFace{"posed", "wall:west-0"})
		if err != nil || wall != wantIndex+2 {
			t.Fatalf("wrong resolved west wall %d, want %d: %v", wall, wantIndex+2, err)
		}
		for i, role := range []CircularExtrusionEdgeRole{CircularExtrusionLowerRim, CircularExtrusionUpperRim, CircularExtrusionVertical} {
			id, err := s.ResolveDirectedCircularExtrusionEdge(CircularExtrusionEdge{"posed", "west-0", role})
			if err != nil || id != wantIndex+12*i {
				t.Fatalf("%s lost stable owner: %d %v", role, id, err)
			}
			owned := false
			for _, use := range fs[wall].Outer {
				owned = owned || use.Edge == id
			}
			if !owned {
				t.Fatalf("resolved rim/vertical %d is not on the named wall", id)
			}
			z := -1.
			if i == 1 {
				z = top
			}
			es, vs := solid.Edges(), solid.Vertices()
			target := pose.Apply(geom.Vec3{X: 2 + radius, Y: 2, Z: z})
			if vs[es[id].Start] != target {
				t.Fatalf("resolved segment start moved from its exact placed vertex: %+v / %+v", vs[es[id].Start], target)
			}
		}
	}
	check(d, .5, 1, 4)

	tx := d.Begin()
	edit := tx.State.Features["part"].DirectedCircularExtrusion
	west := circularDocumentHole("west", 2, 2, .75)
	edit.Holes[0], edit.Holes[1] = edit.Holes[1], append(west[1:], west[0])
	edit.Top = 2
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	check(d, .75, 2, 11)
	bad := d.Begin()
	badR := bad.State.Features["part"].DirectedCircularExtrusion
	badR.Holes[1][0].ID = badR.Holes[0][0].ID
	if err = bad.Commit(); err == nil {
		t.Fatal("committed duplicate identity across independent wires")
	}
	bad = d.Begin()
	badR = bad.State.Features["part"].DirectedCircularExtrusion
	badR.Holes[1] = circularDocumentHole("west", 2, 2, 1.5)
	if err = bad.Commit(); err == nil {
		t.Fatal("committed two touching hole rings")
	}
	check(d, .75, 2, 11)

	if err = d.Undo(); err != nil {
		t.Fatal(err)
	}
	check(d, .5, 1, 4)
	if err = d.Redo(); err != nil {
		t.Fatal(err)
	}
	check(d, .75, 2, 11)
	var b, again bytes.Buffer
	if err = d.Write(&b); err != nil {
		t.Fatal(err)
	}
	reopened, err := Read(bytes.NewReader(b.Bytes()), 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	check(reopened, .75, 2, 11)
	if err = reopened.Write(&again); err != nil || !bytes.Equal(b.Bytes(), again.Bytes()) {
		t.Fatalf("document recipe did not reopen canonically: %v", err)
	}
}

func TestDirectedCircularExtrusionOffsetChainRetainsOriginalRoles(t *testing.T) {
	ctx := context.Background()
	p := func(x, y float64) geom.Vec2 { return geom.Vec2{X: x, Y: y} }
	recipe := &DirectedCircularExtrusionRecipe{
		Outer:  circularDocumentRing("stock", p(0, 0), p(6, 0), p(6, 4), p(0, 4)),
		Holes:  [][]NamedCircularProfileSegment{circularDocumentHole("hole", 2, 2, .5)},
		Bottom: -1, Top: 1,
	}
	pose := geom.Affine{{1, 0, 0, 10}, {0, 1, 0, 2}, {0, 0, 1, 0}}
	d, err := New(State{LengthUnit: .001, Features: map[string]Feature{
		"source": {Kind: DirectedCircularExtrusionFeature, DirectedCircularExtrusion: recipe},
		"side":   {Kind: DirectedCircularExtrusionOffsetFeature, Input: "source", Distance: .125},
		"out":    {Kind: DirectedCircularExtrusionOffsetFeature, Input: "side", Distance: .125},
		"placed": {Kind: TransformFeature, Input: "out", Transform: pose},
	}}, 2)
	if err != nil {
		t.Fatal(err)
	}
	check := func(a *Document, distance float64) {
		t.Helper()
		s, _ := a.Snapshot()
		r, err := s.EvaluateFeatures(ctx)
		if err != nil {
			t.Fatal(err)
		}
		solid := r.Trimmed["placed"]
		f, err := s.ResolveDirectedCircularExtrusionFace(FeatureFace{Feature: "placed", Role: "wall:hole-0"})
		if err != nil || f != 6 {
			t.Fatal(f, err)
		}
		index, err := s.ResolveDirectedCircularExtrusionEdge(CircularExtrusionEdge{Feature: "placed", Segment: "hole-0", Role: CircularExtrusionUpperRim})
		if err != nil || index != 12 {
			t.Fatal(index, err)
		}
		v := solid.Vertices()[solid.Edges()[index].Start]
		radius := .5 - distance
		if v != pose.Apply(geom.Vec3{X: 2 + radius, Y: 2, Z: 1}) {
			t.Fatalf("offset moved the original cap or lost its hole identity: %+v", v)
		}
		m, err := solid.MassProperties(ctx, parametric.IntegrationOptions{})
		want := 2 * ((6+2*distance)*(4+2*distance) - math.Pi*radius*radius)
		if err != nil || math.Abs(m.Volume-want) > 1e-8 {
			t.Fatalf("wrong composed signed offset material: %+v %v, want %v", m, err, want)
		}
	}
	check(d, .25)
	tx := d.Begin()
	f := tx.State.Features["out"]
	f.Distance = -.125
	tx.State.Features["out"] = f
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	check(d, 0)
	bad := d.Begin()
	f = bad.State.Features["side"]
	f.Distance = .5
	bad.State.Features["side"] = f
	if err := bad.Commit(); err == nil {
		t.Fatal("offset edit collapsed the authored hole")
	}
	check(d, 0)
	if err := d.Undo(); err != nil {
		t.Fatal(err)
	}
	check(d, .25)
	var b bytes.Buffer
	if err := d.Write(&b); err != nil {
		t.Fatal(err)
	}
	reopened, err := Read(bytes.NewReader(b.Bytes()), 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	check(reopened, .25)
}
