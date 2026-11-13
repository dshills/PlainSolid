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
