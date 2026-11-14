package parametric

import (
	"bytes"
	"context"
	"math"
	"math/big"
	"plainsolid/collision"
	"testing"
)

func TestCylinderTorusCavity(t *testing.T) {
	ctx := context.Background()
	s, e := NewCylinderTorusCavitySet(ctx, 5, 2, 8, -3, 4, 1e-9)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := s.Solid()
	owners, _ := s.SourceFaces()
	if len(b.faces) != 4 || !b.faces[3].Reversed || owners[0][0].Face != 3 {
		t.Fatal("cavity ownership")
	}
	p, e := b.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-9})
	v := 448*math.Pi - 40*math.Pi*math.Pi
	a := 240*math.Pi + 40*math.Pi*math.Pi
	if e != nil || math.Abs(p.Volume-v) > 1e-6 || math.Abs(p.Area-a) > 1e-6 || math.Abs(p.Centroid.Z-224*math.Pi/v) > 1e-8 {
		t.Fatal(p, e)
	}
	m, _ := s.Measures()
	if m.VolumePi2.Cmp(big.NewRat(-40, 1)) != 0 || m.AreaPi2.Cmp(big.NewRat(40, 1)) != 0 || m.FirstZPi.Cmp(big.NewRat(224, 1)) != 0 {
		t.Fatal(m)
	}
	var data bytes.Buffer
	if e = WriteJSON(&data, b); e != nil {
		t.Fatal(e)
	}
	reload, e := ReadJSON(bytes.NewReader(data.Bytes()), JSONLimits{})
	if e != nil {
		t.Fatal(e)
	}
	var data2 bytes.Buffer
	WriteJSON(&data2, reload)
	if !bytes.Equal(data.Bytes(), data2.Bytes()) {
		t.Fatal("reload")
	}
	mesh, e := reload.Tessellate(ctx, TessellationOptions{Deflection: .5, AnalyticBound: true})
	if e != nil {
		t.Fatal(e)
	}
	if e = mesh.ValidateSolid(); e != nil {
		t.Fatal(e)
	}
	contacts, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
	if e != nil || !contacts.Complete || len(contacts.Contacts) != 0 || mesh.SignedVolume() <= 0 {
		t.Fatal("contacts", e)
	}
}
