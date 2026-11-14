package trimmed

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestExtrudeDiskWithEccentricBlindAndThroughHoles(t *testing.T) {
	ctx := context.Background()
	frame, err := geom.NewFrame(geom.Vec3{X: 2, Y: 5, Z: -3}, geom.Vec3{Z: 1}, geom.Vec3{X: -1})
	if err != nil {
		t.Fatal(err)
	}
	outer := ExtrusionCircle{geom.Vec2{X: 1, Y: 2}, 4}
	through := []ExtrusionCircle{{geom.Vec2{X: -1, Y: 2}, .75}}
	blind := []ExtrusionBlindHole{{ExtrusionCircle: ExtrusionCircle{geom.Vec2{X: 3, Y: 2}, 1}, Depth: 1.25}}
	for _, z := range []float64{-3, 3} {
		s, err := ExtrudeDiskWithHoles(ctx, frame, outer, through, blind, z)
		if err != nil {
			t.Fatal(err)
		}
		topology := s.Topology()
		if topology.InnerWires != 3 || topology.EulerCharacteristic != 0 || len(topology.Shells) != 1 || len(s.faces) != 15 {
			t.Fatal(topology, len(s.faces))
		}
		for _, face := range s.faces[2:14] {
			if _, ok := face.Surface.(geom.CylinderSurface); !ok {
				t.Fatalf("circular side is %T", face.Surface)
			}
		}
		v, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6})
		want := 3*(16-math.Pow(.75, 2)) - 1.25
		cx := (3*16 + 3*math.Pow(.75, 2) - 1.25*3) / want
		cz := math.Copysign((3*(16-math.Pow(.75, 2))*1.5-1.25*1.25/2)/want, z)
		if err != nil || math.Abs(v.Volume-want*math.Pi) > 1e-7 || v.Centroid.Distance(frame.ToWorld(geom.Vec3{X: cx, Y: 2, Z: cz})) > 1e-7 {
			t.Fatal(v, want*math.Pi, cx, cz, err)
		}
		var file bytes.Buffer
		if err := WriteJSON(&file, s); err != nil {
			t.Fatal(err)
		}
		q, err := ReadJSONContext(ctx, &file, JSONLimits{})
		if err != nil || q.Topology().InnerWires != topology.InnerWires || q.Topology().EulerCharacteristic != topology.EulerCharacteristic {
			t.Fatal(q.Topology(), err)
		}
		for _, c := range []struct {
			x, y, d float64
			want    parametric.Location
		}{
			{1, 2, .5, parametric.Inside},
			{-1, 2, 2, parametric.Outside},
			{-.25, 2, 2, parametric.Boundary},
			{3, 2, .5, parametric.Outside},
			{3, 2, 1.25, parametric.Boundary},
			{3, 2, 2, parametric.Inside},
			{1, 6, 2, parametric.Boundary},
			{1, 6.125, 2, parametric.Outside},
		} {
			p := frame.ToWorld(geom.Vec3{X: c.x, Y: c.y, Z: math.Copysign(c.d, z)})
			got, err := q.Classify(p, 1e-8)
			if err != nil || got != c.want {
				t.Fatal(c, got, err)
			}
		}
		display, err := ExtrudeDiskWithHolesCertified(ctx, frame, outer, through, blind, z, CertifiedRevolvePolygonOptions{Deflection: .08})
		if err != nil {
			t.Fatal(err)
		}
		m := display.Mesh
		if m.ValidateSolid() != nil || math.Abs(m.SignedVolume()-want*math.Pi) > .025*want*math.Pi || !display.Collision.Complete || len(display.Collision.Contacts) != 0 {
			t.Fatal(len(m.Triangles), m.SignedVolume(), display.Collision)
		}
		if len(display.OriginalFace) != len(m.Triangles) || display.IdealBoundarySquaredDistance.Cmp(ratInset(.08*.08)) > 0 {
			t.Fatal("mesh certificate", display.IdealBoundarySquaredDistance)
		}
		perFace := make([]int, len(s.faces))
		for _, i := range display.OriginalFace {
			if i < 0 || i >= len(perFace) {
				t.Fatal("display triangle with no source face", i)
			}
			perFace[i]++
		}
		for i, n := range perFace {
			if n == 0 {
				t.Fatal("missing displayed face", i)
			}
		}
	}
}

func TestExtrudeDiskWithHolesRejectsContacts(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	outer := ExtrusionCircle{Radius: 4}
	a := ExtrusionCircle{geom.Vec2{X: -1}, 1}
	for _, c := range []struct {
		name    string
		through []ExtrusionCircle
		blind   []ExtrusionBlindHole
		want    error
	}{
		{"tangent outer", []ExtrusionCircle{{geom.Vec2{X: 3}, 1}}, nil, geom.ErrDomain},
		{"cross outer", []ExtrusionCircle{{geom.Vec2{X: math.Nextafter(3, 4)}, 1}}, nil, geom.ErrDomain},
		{"coincident", []ExtrusionCircle{{Radius: 4}}, nil, geom.ErrDomain},
		{"touching pockets", []ExtrusionCircle{a}, []ExtrusionBlindHole{{ExtrusionCircle: ExtrusionCircle{geom.Vec2{X: 1}, 1}, Depth: 1}}, geom.ErrDomain},
		{"zero radius", []ExtrusionCircle{{}}, nil, geom.ErrInvalid},
		{"bottomless", nil, []ExtrusionBlindHole{{ExtrusionCircle: a, Depth: 2}}, geom.ErrDomain},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, err := ExtrudeDiskWithHoles(ctx, f, outer, c.through, c.blind, -2)
			if !errors.Is(err, c.want) || len(s.faces) != 0 {
				t.Fatal(err, len(s.faces))
			}
		})
	}
	s, err := ExtrudeDiskWithHoles(ctx, f, outer, nil, nil, 2)
	if err != nil || len(s.faces) != 6 || s.Topology().EulerCharacteristic != 2 {
		t.Fatal(err, s.Topology())
	}
}
