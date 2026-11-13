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

func TestExtrudeRoundedRectangleWithCircularFeatures(t *testing.T) {
	ctx := context.Background()
	f, e := geom.NewFrame(geom.Vec3{X: 2, Y: -3, Z: 4}, geom.Vec3{Y: 1}, geom.Vec3{X: -1})
	if e != nil {
		t.Fatal(e)
	}
	p := RoundedRectangle{Max: geom.Vec2{X: 8, Y: 5}, Radii: [4]float64{.5, 1, 0, 1.5}}
	through := []ExtrusionCircle{{Center: geom.Vec2{X: 2, Y: 2}, Radius: .5}}
	blind := []ExtrusionBlindHole{{ExtrusionCircle: ExtrusionCircle{Center: geom.Vec2{X: 5, Y: 3}, Radius: .75}, Depth: 1}}
	for _, z := range []float64{-3, 3} {
		s, err := ExtrudeRoundedRectangle(ctx, f, p, through, blind, z)
		if err != nil {
			t.Fatal(err)
		}
		topology := s.Topology()
		if len(s.faces) != 18 || topology.InnerWires != 3 || topology.EulerCharacteristic != 0 || len(topology.Shells) != 1 {
			t.Fatal(topology)
		}
		if len(s.faces[0].Outer) != 7 || len(s.faces[1].Outer) != 7 {
			t.Fatal("missing straight or circular cap trims")
		}
		var arcs int
		for _, u := range s.faces[0].Outer {
			if _, ok := u.Curve.(anchoredCircularUV); ok {
				arcs++
			}
		}
		if arcs != 3 {
			t.Fatal("missing anchored quarter-circle trims", arcs)
		}
		m, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		area := 40 - (1-math.Pi/4)*(.25+1+2.25)
		perimeter := 26 - (2-math.Pi/2)*3
		wantV := 3*(area-math.Pi*.25) - math.Pi*.5625
		wantA := 2*(area-math.Pi*.25) + 3*perimeter + 3*math.Pi + 1.5*math.Pi
		if err != nil || math.Abs(m.Volume-wantV) > 1e-7 || math.Abs(m.Area-wantA) > 1e-7 {
			t.Fatal(m, wantV, wantA, err)
		}
		display, err := ExtrudeRoundedRectangleCertified(ctx, f, p, through, blind, z, CertifiedRevolvePolygonOptions{Deflection: .08})
		if err != nil {
			t.Fatal(err)
		}
		if !display.Collision.Complete || len(display.Collision.Contacts) != 0 ||
			display.Mesh.ValidateSolid() != nil || len(display.OriginalFace) != len(display.Mesh.Triangles) ||
			display.IdealBoundarySquaredDistance.Cmp(ratInset(.08*.08)) > 0 {
			t.Fatal("missing or invalid display certificate")
		}
		wallCounts := make([]int, len(s.faces))
		for _, face := range display.OriginalFace {
			wallCounts[face]++
		}
		for i, u := range s.faces[0].Outer {
			want := 2
			if u.Curve != nil {
				want *= display.Segments / 4
			}
			if wallCounts[2+i] != want {
				t.Fatal("outer wall sample ownership", i, wallCounts[2+i], want)
			}
		}
		if math.Abs(display.Mesh.SignedVolume()-wantV) > .005*wantV {
			t.Fatal("display volume disagrees", display.Mesh.SignedVolume(), wantV)
		}
		var file bytes.Buffer
		if err := WriteJSON(&file, s); err != nil {
			t.Fatal(err)
		}
		q, err := ReadJSONContext(ctx, &file, JSONLimits{})
		if err != nil || q.Topology().EulerCharacteristic != 0 {
			t.Fatal(q.Topology(), err)
		}
		for _, tc := range []struct {
			u, v, depth float64
			want        parametric.Location
		}{
			{.01, .01, 2, parametric.Outside},
			{.4, .4, 2, parametric.Inside},
			{8, 5, 2, parametric.Boundary}, // unrounded corner
			{2, 2, 2, parametric.Outside},
			{5, 3, .5, parametric.Outside},
			{5, 3, 1, parametric.Boundary},
			{5, 3, 2, parametric.Inside},
		} {
			pt := f.ToWorld(geom.Vec3{X: tc.u, Y: tc.v, Z: math.Copysign(tc.depth, z)})
			got, e := q.Classify(pt, 1e-9)
			if e != nil || got != tc.want {
				t.Fatal(tc, pt, got, e)
			}
		}
	}
}

func TestExtrudeRoundedRectangleRejectsUnresolvedOrTouchingGeometry(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	p := RoundedRectangle{Max: geom.Vec2{X: 4, Y: 4}, Radii: [4]float64{1, 2, 2}}
	if s, e := ExtrudeRoundedRectangle(ctx, f, p, nil, nil, 1); !errors.Is(e, geom.ErrDomain) || len(s.faces) != 0 {
		t.Fatal("coincident tangent points", e)
	}
	p.Radii = [4]float64{1, .1}
	if s, e := ExtrudeRoundedRectangle(ctx, f, p, nil, nil, 1); !errors.Is(e, geom.ErrUnrepresentable) || len(s.faces) != 0 {
		t.Fatal("rounded tangent coordinate accepted", e)
	}
	p.Radii = [4]float64{1}
	for _, h := range []ExtrusionCircle{
		{Center: geom.Vec2{X: .5, Y: .5}, Radius: .125}, // center on corner chord
		{Center: geom.Vec2{X: 3, Y: 1}, Radius: 1},      // tangent straight sides
	} {
		if s, e := ExtrudeRoundedRectangle(ctx, f, p, []ExtrusionCircle{h}, nil, 1); !errors.Is(e, geom.ErrDomain) || len(s.faces) != 0 {
			t.Fatal("invalid contained disk", h, e)
		}
	}
	p.Radii = [4]float64{}
	s, e := ExtrudeRoundedRectangle(ctx, f, p, nil, nil, -1)
	if e != nil || len(s.faces) != 6 {
		t.Fatal("sharp rectangle", len(s.faces), e)
	}
}
