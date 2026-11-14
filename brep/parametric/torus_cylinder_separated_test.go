package parametric

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestNativeSeparatedTorusCylinder(t *testing.T) {
	ctx := context.Background()
	for _, op := range []geom.TorusCylinderOperation{geom.TorusCylinderCommon, geom.TorusCylinderDifference, geom.TorusCylinderUnion} {
		s, e := NewSeparatedTorusCylinderSet(ctx, 5, 2, 2, -3, 4, op, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		empty, e := s.Empty()
		if e != nil || empty != (op == geom.TorusCylinderCommon) {
			t.Fatal("empty", e)
		}
		expected := 0
		if op == geom.TorusCylinderDifference {
			expected = 1
		}
		if op == geom.TorusCylinderUnion {
			expected = 2
		}
		if len(s.Solids()) != expected {
			t.Fatal("component collapse")
		}
		m, e := s.Measures()
		if e != nil {
			t.Fatal(e)
		}
		wantV, wantA, wantFirst := 0., 0., 0.
		if op != geom.TorusCylinderCommon {
			wantV = 40 * math.Pi * math.Pi
			wantA = 40 * math.Pi * math.Pi
			if m.VolumePi2.Cmp(big.NewRat(40, 1)) != 0 || m.AreaPi2.Cmp(big.NewRat(40, 1)) != 0 {
				t.Fatal("torus exact coefficient")
			}
		}
		if op == geom.TorusCylinderUnion {
			wantV += 28 * math.Pi
			wantA += 36 * math.Pi
			wantFirst = 14 * math.Pi
			if m.VolumePi.Cmp(big.NewRat(28, 1)) != 0 || m.AreaPi.Cmp(big.NewRat(36, 1)) != 0 || m.FirstZPi.Cmp(big.NewRat(14, 1)) != 0 {
				t.Fatal("cylinder exact coefficient")
			}
		}
		m.VolumePi2.SetInt64(99)
		fresh, _ := s.Measures()
		if fresh.VolumePi2.Cmp(big.NewRat(99, 1)) == 0 {
			t.Fatal("measure alias")
		}
		owners, e := s.SourceFaces()
		if e != nil {
			t.Fatal(e)
		}
		if empty {
			if _, e = s.Compound(); e == nil {
				t.Fatal("bogus empty boundary")
			}
			for _, v := range owners {
				if len(v) != 0 {
					t.Fatal("empty owners")
				}
			}
			continue
		}
		body, e := s.Compound()
		if e != nil {
			t.Fatal(e)
		}
		if owners[0][0] != (TorusCylinderNativeOwner{0, 0, 0}) {
			t.Fatal("torus ancestry")
		}
		if op == geom.TorusCylinderUnion {
			for i := 1; i < 4; i++ {
				if owners[i][0] != (TorusCylinderNativeOwner{1, i - 1, i}) {
					t.Fatal("cylinder ancestry")
				}
			}
		}
		owners[0][0].Face = 99
		again, _ := s.SourceFaces()
		if again[0][0].Face != 0 {
			t.Fatal("owner alias")
		}
		props, e := body.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil {
			t.Fatal(e)
		}
		if math.Abs(props.Volume-wantV) > 1e-6 || math.Abs(props.Area-wantA) > 1e-6 || math.Abs(props.Centroid.Z*props.Volume-wantFirst) > 1e-6 {
			t.Fatal("analytic", props, wantV, wantA, wantFirst)
		}
		var data bytes.Buffer
		if e = WriteJSON(&data, body); e != nil {
			t.Fatal(e)
		}
		loaded, e := ReadJSON(bytes.NewReader(data.Bytes()), JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		var canonical bytes.Buffer
		if e = WriteJSON(&canonical, loaded); e != nil || !bytes.Equal(data.Bytes(), canonical.Bytes()) {
			t.Fatal("reload", e)
		}
		mesh, e := loaded.Tessellate(ctx, TessellationOptions{Deflection: .5, AnalyticBound: true})
		if e != nil {
			t.Fatal(e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		if mesh.SignedVolume() <= 0 || math.Abs(mesh.SignedVolume()-wantV) > .06*wantV {
			t.Fatal("display mass")
		}
		contacts, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
		if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 {
			t.Fatal("global embedding", e, len(contacts.Contacts))
		}
		t.Log(op, expected, len(mesh.Triangles), props.Volume, props.Area)
	}
	for _, c := range []float64{3, 4, math.Nextafter(3, 0)} {
		if s, e := NewSeparatedTorusCylinderSet(ctx, 5, 2, c, -3, 4, geom.TorusCylinderUnion, 1e-9); e == nil || len(s.Solids()) != 0 {
			t.Fatal("contact/clearance", c)
		}
	}
	if _, e := NewSeparatedTorusCylinderSet(ctx, 5, 2, 2, -3, math.Nextafter(4, 0), geom.TorusCylinderUnion, 1e-9); e == nil {
		t.Fatal("nonrepresented native height")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := NewSeparatedTorusCylinderSet(canceled, 5, 2, 2, -3, 4, geom.TorusCylinderCommon, 1e-9); e == nil {
		t.Fatal("cancel")
	}
}
