package document

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/csg/fillet"
	"plainsolid/geom"
)

func TestSelectiveFilletRecipePersistence(t *testing.T) {
	feature := Feature{Kind: PrismFilletFeature,
		Profile: []geom.Vec3{{}, {X: 4}, {X: 4, Y: 3}, {Y: 3}}, Offset: geom.Vec3{Z: 2},
		CornerRadii: []fillet.CornerRadius{{Corner: 0, Radius: .25}, {Corner: 2, Radius: .5}}}
	d, e := New(State{LengthUnit: 1, Features: map[string]Feature{"part": feature}}, 2)
	if e != nil {
		t.Fatal(e)
	}
	feature.CornerRadii[0].Radius = 10
	var data bytes.Buffer
	if e := d.Write(&data); e != nil {
		t.Fatal(e)
	}
	copy, e := Read(&data, 0, 2)
	if e != nil {
		t.Fatal(e)
	}
	check := func(d *Document, wantRadius float64) {
		t.Helper()
		s, _ := d.Snapshot()
		f := s.Features["part"]
		if len(f.CornerRadii) != 2 || f.CornerRadii[0].Radius != wantRadius {
			t.Fatal(f.CornerRadii)
		}
		m, e := s.EvaluateFeatures(context.Background())
		if e != nil {
			t.Fatal(e)
		}
		p, e := m.Trimmed["part"].MassProperties(context.Background(), parametric.IntegrationOptions{})
		want := 24 - 2*(1-math.Pi/4)*(wantRadius*wantRadius+.25)
		if e != nil || math.Abs(p.Volume-want) > 1e-8 {
			t.Fatal(p, want, e)
		}
	}
	check(copy, .25)
	tx := copy.Begin()
	f := tx.State.Features["part"]
	f.CornerRadii[0].Radius = .125
	tx.State.Features["part"] = f
	if e := tx.Commit(); e != nil {
		t.Fatal(e)
	}
	check(copy, .125)
	if e := copy.Undo(); e != nil {
		t.Fatal(e)
	}
	check(copy, .25)
}

func TestReentrantOrthogonalFilletRecipeRegeneratesExactMaterial(t *testing.T) {
	ctx := context.Background()
	f := Feature{Kind: OrthogonalPrismFilletFeature,
		Profile: []geom.Vec3{{Z: -1}, {X: 4, Z: -1}, {X: 4, Y: 1, Z: -1}, {X: 1, Y: 1, Z: -1}, {X: 1, Y: 4, Z: -1}, {Y: 4, Z: -1}},
		Offset:  geom.Vec3{Z: 2}, CornerRadii: []fillet.CornerRadius{{Corner: 1, Radius: .25}, {Corner: 3, Radius: .25}},
	}
	d, err := New(State{LengthUnit: .001, Features: map[string]Feature{"part": f}}, 2)
	if err != nil {
		t.Fatal(err)
	}
	check := func(doc *Document, radius float64) {
		t.Helper()
		s, _ := doc.Snapshot()
		g, err := s.EvaluateFeatures(ctx)
		if err != nil {
			t.Fatal(err)
		}
		got := g.Trimmed["part"]
		m, err := got.MassProperties(ctx, parametric.IntegrationOptions{})
		want := 14 + 2*(radius*radius-.25*.25)*(1-math.Pi/4)
		if err != nil || len(got.Faces()) != 10 || math.Abs(m.Volume-want) > 1e-8 {
			t.Fatalf("wrong edited exact reentrant material: %+v, expected %.12f, %v", m, want, err)
		}
	}
	check(d, .25)
	x := d.Begin()
	f = x.State.Features["part"]
	f.CornerRadii[1].Radius = .5
	x.State.Features["part"] = f
	if err := x.Commit(); err != nil {
		t.Fatal(err)
	}
	check(d, .5)
	rejected := d.Begin()
	f = rejected.State.Features["part"]
	f.CornerRadii[1].Radius = 3
	rejected.State.Features["part"] = f
	if err := rejected.Commit(); err == nil {
		t.Fatal("invalid fillet suppressed a retained original side")
	}
	check(d, .5)
	var raw bytes.Buffer
	if err := d.Write(&raw); err != nil {
		t.Fatal(err)
	}
	reopened, err := Read(bytes.NewReader(raw.Bytes()), 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	check(reopened, .5)
}
