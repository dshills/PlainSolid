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

func TestNativeEncompassingTorusCylinder(t *testing.T) {
	ctx := context.Background()
	for _, op := range []geom.TorusCylinderOperation{geom.TorusCylinderCommon, geom.TorusCylinderDifference, geom.TorusCylinderUnion} {
		s, e := NewEncompassingTorusCylinderSet(ctx, 5, 2, 8, -3, 4, op, 1e-9)
		if e != nil {
			t.Fatal(e)
		}
		empty, e := s.Empty()
		if e != nil || empty != (op == geom.TorusCylinderDifference) {
			t.Fatal("empty")
		}
		m, e := s.Measures()
		if e != nil {
			t.Fatal(e)
		}
		owners, _ := s.SourceFaces()
		if empty {
			if len(s.Solids()) != 0 || m.VolumePi.Sign() != 0 || m.VolumePi2.Sign() != 0 {
				t.Fatal("empty measure")
			}
			if _, e = s.Solid(); e == nil {
				t.Fatal("bogus empty solid")
			}
			continue
		}
		body, e := s.Solid()
		if e != nil {
			t.Fatal(e)
		}
		p, e := body.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-9})
		if e != nil {
			t.Fatal(e)
		}
		v, a, cz := 40*math.Pi*math.Pi, 40*math.Pi*math.Pi, 0.
		if op == geom.TorusCylinderUnion {
			v, a, cz = 448*math.Pi, 240*math.Pi, .5
			if len(owners[0]) != 0 || owners[1][0].Face != 0 || owners[2][0].Face != 1 || owners[3][0].Face != 2 {
				t.Fatal("buried torus owner")
			}
			if m.VolumePi.Cmp(big.NewRat(448, 1)) != 0 || m.FirstZPi.Cmp(big.NewRat(224, 1)) != 0 {
				t.Fatal(m)
			}
		} else {
			if owners[0][0].Face != 0 || len(owners[1]) != 0 || m.VolumePi2.Cmp(big.NewRat(40, 1)) != 0 {
				t.Fatal("original torus")
			}
		}
		if math.Abs(p.Volume-v) > 1e-6 || math.Abs(p.Area-a) > 1e-6 || math.Abs(p.Centroid.Z-cz) > 1e-8 {
			t.Fatal(p, v, a, cz)
		}
		var raw bytes.Buffer
		if e = WriteJSON(&raw, body); e != nil {
			t.Fatal(e)
		}
		reload, e := ReadJSON(bytes.NewReader(raw.Bytes()), JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		var canonical bytes.Buffer
		if e = WriteJSON(&canonical, reload); e != nil || !bytes.Equal(raw.Bytes(), canonical.Bytes()) {
			t.Fatal("reload", e)
		}
		mesh, e := reload.Tessellate(ctx, TessellationOptions{Deflection: .5, AnalyticBound: true})
		if e != nil {
			t.Fatal(e)
		}
		if e = mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		report, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
		if e != nil || !report.Complete || len(report.Contacts) != 0 || mesh.SignedVolume() <= 0 {
			t.Fatal("display", e)
		}
		t.Log(op, len(mesh.Triangles), p.Volume, p.Area)
	}
	for _, c := range []float64{7, math.Nextafter(7, math.Inf(1))} {
		if _, e := NewEncompassingTorusCylinderSet(ctx, 5, 2, c, -3, 4, geom.TorusCylinderUnion, 1e-9); e == nil {
			t.Fatal("contact tolerance")
		}
	}
}
