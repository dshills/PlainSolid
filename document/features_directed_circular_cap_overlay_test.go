package document

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/exchange/iges"
	"plainsolid/exchange/step"
	"plainsolid/geom"
)

func TestDirectedCircularCapOverlayKeepsNamedOriginalFragments(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	p := func(x, y float64) geom.Vec2 { return geom.Vec2{X: x, Y: y} }
	outer := circularDocumentRing("rim", p(5, 0), p(0, 5), p(-5, 0), p(0, -5))
	for i := range outer {
		outer[i].Curve.Radius = 5
	}
	arcA, arcB := outer[0], outer[0]
	arcA.ID, arcB.ID = "arc-A", "arc-B"
	arcA.Curve.End, arcB.Curve.Start = p(3, 4), p(3, 4)
	upperOuter := append([]NamedCircularProfileSegment{arcA, arcB}, outer[1:]...)
	hole := circularDocumentRing("hole", p(-1, -1), p(-1, 1), p(1, 1), p(1, -1))
	lineA, lineB := hole[0], hole[0]
	lineA.ID, lineB.ID = "line-A", "line-B"
	lineA.Curve.End, lineB.Curve.Start = p(-1, 0), p(-1, 0)
	lowerHole := append([]NamedCircularProfileSegment{lineA, lineB}, hole[1:]...)
	pose := geom.Affine{{0, 0, -1, 8}, {0, 1, 0, 2}, {1, 0, 0, 4}}
	d, err := New(State{LengthUnit: .001, Features: map[string]Feature{
		"lower": {Kind: DirectedCircularExtrusionFeature, DirectedCircularExtrusion: &DirectedCircularExtrusionRecipe{
			Outer: outer, Holes: [][]NamedCircularProfileSegment{lowerHole}, Bottom: -2, Top: 0}},
		"upper": {Kind: DirectedCircularExtrusionFeature, DirectedCircularExtrusion: &DirectedCircularExtrusionRecipe{
			Outer: upperOuter, Holes: [][]NamedCircularProfileSegment{hole}, Bottom: 0, Top: 3}},
		"join":  {Kind: DirectedCircularCapOverlayFeature, Input: "lower", Tool: "upper"},
		"posed": {Kind: TransformFeature, Input: "join", Transform: pose},
	}, Nodes: map[string]Node{
		"step-group": {Transform: geom.Affine{{1, 0, 0, 10}, {0, 1, 0, -1}, {0, 0, 1, 6}}},
		"step-part":  {Name: "Placed complete curved join", Asset: "posed", Parent: "step-group", Transform: geom.Identity()},
		"iges-part":  {Asset: "join", Transform: geom.Affine{{0, -1, 0, 14}, {1, 0, 0, -2}, {0, 0, 1, 7}}},
	}}, 2)
	if err != nil {
		t.Fatal(err)
	}

	check := func(d *Document, top float64) {
		t.Helper()
		s, _ := d.Snapshot()
		proof, err := s.EvaluateCircularCapOverlay(ctx, "posed")
		if err != nil {
			t.Fatal(err)
		}
		a, b := proof.Members[0], proof.Members[1]
		faces, edges, vertices := proof.Solid.Faces(), proof.Solid.Edges(), proof.Solid.Vertices()
		if a.Feature != "lower" || b.Feature != "upper" || len(a.Caps[0]) != 1 || len(a.Caps[1]) != 0 ||
			len(b.Caps[0]) != 0 || len(b.Caps[1]) != 1 || len(proof.SharedRimEdges) != 10 || len(faces) != 22 {
			t.Fatal("lost one or both original cap roles or retained an internal cap")
		}
		arc, line := a.Segments["rim-0"], b.Segments["hole-0"]
		if len(arc.Walls) != 2 || len(line.Walls) != 2 || len(arc.Rims[1]) != 2 || len(line.Rims[0]) != 2 {
			t.Fatal("original wall or rim lost a refined fragment")
		}
		for i, name := range []string{"arc-A", "arc-B"} {
			edge := arc.Rims[1][i].Edge
			fine := b.Segments[name]
			if len(fine.Rims[0]) != 1 || fine.Rims[0][0].Edge != edge || arc.Walls[i] == fine.Walls[0] {
				t.Fatalf("seam edge %d did not retain separate wall owners", edge)
			}
			for _, face := range []int{arc.Walls[i], fine.Walls[0]} {
				owned := false
				for _, use := range faces[face].Outer {
					owned = owned || use.Edge == edge
				}
				if !owned {
					t.Fatal("named wall does not use its output seam edge", face, edge)
				}
			}
		}
		for i, name := range []string{"line-A", "line-B"} {
			if line.Rims[0][i].Edge != a.Segments[name].Rims[1][0].Edge || line.Rims[0][i].Map[1][1].Sign() != 0 {
				t.Fatal("straight rim lost its affine source on a shared edge")
			}
		}
		m := arc.Rims[1][1].Map
		mid := big.NewRat(1, 2)
		u := new(big.Rat).Quo(
			new(big.Rat).Add(m[0][0], new(big.Rat).Mul(m[0][1], mid)),
			new(big.Rat).Add(m[1][0], new(big.Rat).Mul(m[1][1], mid)),
		)
		if u.RatString() != "8/11" || arc.Rims[1][1].Minimum.RatString() != "1/2" ||
			vertices[arc.StartVertices[1]] != pose.Apply(geom.Vec3{X: 5}) ||
			edges[arc.Vertical.Edge].End != arc.StartVertices[1] {
			t.Fatal("placed named arc lost its exact source parameter or vertex")
		}
		props, err := proof.Solid.MassProperties(ctx, parametric.IntegrationOptions{})
		foot, height := 25*math.Pi-4, top+2
		if err != nil || math.Abs(props.Volume-height*foot) > 1e-7 || math.Abs(props.Area-(2*foot+height*(10*math.Pi+8))) > 1e-7 {
			t.Fatalf("wrong material after joining complete caps: %+v %v", props, err)
		}
	}
	check(d, 3)
	tx := d.Begin()
	upper := tx.State.Features["upper"].DirectedCircularExtrusion
	upper.Top = 4
	upper.Outer = append(upper.Outer[2:], upper.Outer[:2]...)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	tx = d.Begin()
	tx.State.Features["upper"].DirectedCircularExtrusion.Bottom = .125
	if err := tx.Commit(); err == nil {
		t.Fatal("published a document with separated cap planes")
	}
	var raw bytes.Buffer
	if err := d.Write(&raw); err != nil {
		t.Fatal(err)
	}
	reopened, err := Read(bytes.NewReader(raw.Bytes()), 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	check(reopened, 4)

	s, _ := reopened.Snapshot()
	var source [2]trimmed.ExactCircularSweep
	for i, id := range [2]string{"lower", "upper"} {
		d := s.Features[id].DirectedCircularExtrusion
		rings, _, err := d.profile()
		if err != nil {
			t.Fatal(err)
		}
		source[i] = trimmed.ExactCircularSweep{Outer: rings[0], Holes: rings[1:], Bottom: d.Bottom, Top: d.Top}
	}
	frame, err := geom.NewFrame(geom.Vec3{X: 18, Y: 1, Z: 10}, geom.Vec3{Z: 1}, geom.Vec3{X: -1})
	if err != nil {
		t.Fatal(err)
	}
	var stepRaw, igesRaw bytes.Buffer
	if err := s.WriteCircularCapOverlaySTEP(ctx, &stepRaw, "step-part"); err != nil {
		t.Fatal(err)
	}
	stepSource := step.ExactCircularSweepCapOverlay{Name: "Placed complete curved join", Frame: frame, Sweeps: source, LengthUnit: .001}
	a, err := step.ReadExactCircularSweepCapOverlay(ctx, &stepRaw, stepSource, step.Limits{})
	if err != nil || len(a.Faces[0][2]) != 2 || len(a.Faces[1][0]) != 0 {
		t.Fatalf("STEP changed the edited source or composed placement: %v", err)
	}
	for i := range source {
		source[i].Bottom += 7
		source[i].Top += 7
		for _, ring := range append([][]trimmed.ExactCircularLoopSegment{source[i].Outer}, source[i].Holes...) {
			for j := range ring {
				q := &ring[j]
				q.Start, q.End = p(14-q.Start.Y, q.Start.X-2), p(14-q.End.Y, q.End.X-2)
				if q.Radius > 0 {
					q.Center = p(14-q.Center.Y, q.Center.X-2)
				}
			}
		}
	}
	if err := s.WriteCircularCapOverlayIGES(ctx, &igesRaw, "iges-part"); err != nil {
		t.Fatal(err)
	}
	b, err := iges.ReadExactCircularSweepCapOverlay(ctx, &igesRaw, iges.ExactCircularSweepCapOverlay{Sweeps: source, Unit: .001}, iges.Limits{})
	if err != nil || len(b.Faces[0][2]) != 2 || len(b.Faces[1][0]) != 0 {
		t.Fatalf("IGES changed the original split wires after placement: %v", err)
	}
	igesRaw.Reset()
	if err := s.WriteCircularCapOverlayIGES(ctx, &igesRaw, "step-part"); !errors.Is(err, geom.ErrDomain) || igesRaw.Len() != 0 {
		t.Fatalf("IGES wrote an unsupported tilted placement: %v", err)
	}
	group, part := s.Nodes["step-group"], s.Nodes["step-part"]
	group.Transform[0][3], part.Transform[0][3] = 1e16, .25
	s.Nodes["step-group"], s.Nodes["step-part"] = group, part
	stepRaw.Reset()
	if err := s.WriteCircularCapOverlaySTEP(ctx, &stepRaw, "step-part"); !errors.Is(err, geom.ErrUnrepresentable) || stepRaw.Len() != 0 {
		t.Fatalf("STEP rounded the composed occurrence translation: %v", err)
	}
}
