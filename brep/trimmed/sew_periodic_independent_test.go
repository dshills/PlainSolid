package trimmed

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func periodicIndependentParts(s Solid) []SewFace {
	parts := make([]SewFace, len(s.faces))
	for i, f := range s.faces {
		parts[i] = s.splineSewFace(f)
	}
	return parts
}
func periodicIndependentCounts(s Solid) (seams, closed, poles int) {
	for _, edge := range s.edges {
		if edge.Curve == nil {
			poles++
		} else if edge.Start == edge.End {
			closed++
		}
	}
	for _, face := range s.faces {
		uses := map[int]int{}
		for _, loop := range append([][]Use{face.Outer}, face.Holes...) {
			for _, u := range loop {
				uses[u.Edge]++
			}
		}
		for id, n := range uses {
			if n == 2 && s.edges[id].Curve != nil {
				seams++
			}
		}
	}
	return
}

func TestSewPeriodicIndependentPrimitives(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: .25, Y: -.5, Z: .75}
	frame, e := geom.FrameFromNormal(center, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	sphere, e := parametric.Sphere(center, 1)
	if e != nil {
		t.Fatal(e)
	}
	torus, e := parametric.Torus(frame, 2, .5)
	if e != nil {
		t.Fatal(e)
	}
	cylinder, e := parametric.Cylinder(frame, 1, 2)
	if e != nil {
		t.Fatal(e)
	}
	angle := math.Atan(.5)
	cone, e := parametric.Cone(frame, angle, 2)
	if e != nil {
		t.Fatal(e)
	}
	radius := 2 * math.Tan(angle)
	for _, tc := range []struct {
		name                        string
		solid                       parametric.Solid
		volume, area                float64
		seams, closed, poles, euler int
	}{
		{"sphere", sphere, 4 * math.Pi / 3, 4 * math.Pi, 1, 0, 2, 2},
		{"torus", torus, math.Pi * math.Pi, 4 * math.Pi * math.Pi, 2, 2, 0, 0},
		{"cylinder", cylinder, 2 * math.Pi, 6 * math.Pi, 3, 2, 2, 2},
		{"cone", cone, 2 * math.Pi * radius * radius / 3, math.Pi * radius * (radius + math.Hypot(radius, 2)), 2, 1, 2, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, e := FromParametric(tc.solid)
			if e != nil {
				t.Fatal(e)
			}
			parts := periodicIndependentParts(input)
			out, h, e := Sew(ctx, parts, SewOptions{Tolerance: input.tolerance})
			if e != nil {
				t.Fatal(e)
			}
			checkPeriodicIndependentBounds(t, out)
			seams, closed, poles := periodicIndependentCounts(out)
			if seams != tc.seams || closed != tc.closed || poles != tc.poles || out.Topology().EulerCharacteristic != tc.euler || len(out.Topology().Shells) != 1 {
				t.Fatalf("seams%d closed%d poles%d Euler%d", seams, closed, poles, out.Topology().EulerCharacteristic)
			}
			if len(out.edges) != len(input.edges) || len(out.vertices) != len(input.vertices) || len(out.faces) != len(input.faces) {
				t.Fatal("changed primitive graph counts")
			}
			for i, p := range parts {
				if len(h.Edges[i]) != len(p.Edges) || len(h.Vertices[i]) != len(p.Vertices) {
					t.Fatal("missing local history")
				}
				for j, id := range h.Vertices[i] {
					if out.vertices[id] != p.Vertices[j] {
						t.Fatal("moved seam/pole vertex")
					}
				}
			}
			proof, e := out.ValidateEdgeAgreement(ctx, edgecheck.Options{})
			if e != nil || !proof.Complete {
				t.Fatal("continuous agreement", proof.Complete, e)
			}
			mass, e := out.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil || math.Abs(mass.Volume-tc.volume) > 1e-7 || math.Abs(mass.Area-tc.area) > 1e-7 {
				t.Fatal("analytic mass", mass, e)
			}
			m, e := out.Tessellate(ctx, parametric.TessellationOptions{Deflection: .02, MaxSubdivisions: 128, MaxTriangles: 250000})
			if e != nil {
				t.Fatal(e)
			}
			if e = m.ValidateSolid(); e != nil {
				t.Fatal("open periodic mesh", e)
			}
			if math.Abs(m.SignedVolume()-tc.volume) > .03*math.Max(1, tc.volume) {
				t.Fatal("mesh volume", m.SignedVolume(), tc.volume)
			}
		})
	}
}

func TestSewPeriodicRejectsWrongIdentity(t *testing.T) {
	ctx := context.Background()
	frame, e := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	p, e := parametric.Torus(frame, 2, .5)
	if e != nil {
		t.Fatal(e)
	}
	s, e := FromParametric(p)
	if e != nil {
		t.Fatal(e)
	}
	parts := periodicIndependentParts(s)
	// Preserve the rectangular wire closure while perturbing one periodic side
	// by much less than the acceptance tolerance: a seam identity cannot snap.
	const delta = 1e-12
	parts[0].Face.Outer[1].UVStart.X -= delta
	parts[0].Face.Outer[1].UVEnd.X -= delta
	parts[0].Face.Outer[0].UVEnd.X -= delta
	parts[0].Face.Outer[2].UVStart.X -= delta
	if _, _, e = Sew(ctx, parts, SewOptions{Tolerance: 1e-8}); e == nil {
		t.Fatal("accepted wrong periodic seam chart")
	}
	shifted, e := p.Transformed(geom.Translation(geom.Vec3{X: delta}))
	if e != nil {
		t.Fatal(e)
	}
	shiftedTrim, e := FromParametric(shifted)
	if e != nil {
		t.Fatal(e)
	}
	bad := periodicIndependentParts(s)
	shiftedParts := periodicIndependentParts(shiftedTrim)
	bad[0].Edges[0].Curve = shiftedParts[0].Edges[0].Curve
	if _, _, e = Sew(ctx, bad, SewOptions{Tolerance: 1e-8}); e == nil {
		t.Fatal("accepted shifted periodic model curve")
	}
}

func TestAnalyticPeriodicIdentityRejectsLargeTurns(t *testing.T) {
	frame, e := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	surface, e := geom.NewCylinderSurface(frame, 1)
	if e != nil {
		t.Fatal(e)
	}
	shift := math.Ldexp(2*math.Pi, 50)
	a, _ := analyticUseLine(surface, Use{UVStart: geom.Vec2{}, UVEnd: geom.Vec2{Y: 1}})
	b, _ := analyticUseLine(surface, Use{UVStart: geom.Vec2{X: shift}, UVEnd: geom.Vec2{X: shift, Y: 1}})
	p, _ := surface.Eval(0, .5)
	q, _ := surface.Eval(shift, .5)
	if p.Sub(q).Length() < .01 {
		t.Fatal("fixture lacks significant represented period error")
	}
	if sameAnalyticUVLines(a, b) {
		t.Fatal("large represented turns falsely certified as identical")
	}
}

func TestSewPeriodicAffinePlacements(t *testing.T) {
	ctx := context.Background()
	frame, e := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	torus, e := parametric.Torus(frame, 2, .5)
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name      string
		placement geom.Affine
		volume    float64
	}{
		{"scaled", geom.Affine{{2, 0, 0, .25}, {0, 2, 0, -.5}, {0, 0, 2, .75}}, 8 * math.Pi * math.Pi},
		{"reflected-shear", geom.Affine{{-1, .25, 0, .25}, {0, 1, .5, -.5}, {0, 0, 1, .75}}, math.Pi * math.Pi},
	} {
		t.Run(tc.name, func(t *testing.T) {
			placed, e := torus.Transformed(tc.placement)
			if e != nil {
				t.Fatal(e)
			}
			input, e := FromParametric(placed)
			if e != nil {
				t.Fatal(e)
			}
			out, _, e := Sew(ctx, periodicIndependentParts(input), SewOptions{Tolerance: input.tolerance})
			if e != nil {
				t.Fatal(e)
			}
			checkPeriodicIndependentBounds(t, out)
			seams, closed, poles := periodicIndependentCounts(out)
			if seams != 2 || closed != 2 || poles != 0 || out.Topology().EulerCharacteristic != 0 {
				t.Fatal("lost torus seams", seams, closed, poles)
			}
			proof, e := out.ValidateEdgeAgreement(ctx, edgecheck.Options{})
			if e != nil || !proof.Complete {
				t.Fatal("placed edge identities", e)
			}
			p, e := out.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil || math.Abs(p.Volume-tc.volume) > 1e-6 || p.Centroid.Sub(tc.placement.Apply(geom.Vec3{})).Length() > 1e-8 {
				t.Fatal("placed mass", p, e)
			}
			m, e := out.Tessellate(ctx, parametric.TessellationOptions{Deflection: .03, MaxSubdivisions: 128, MaxTriangles: 250000})
			if e != nil {
				t.Fatal(e)
			}
			if e = m.ValidateSolid(); e != nil {
				t.Fatal("collapsed closed seam in placed mesh", e)
			}
			if math.Abs(m.SignedVolume()-tc.volume) > .03*tc.volume {
				t.Fatal(m.SignedVolume(), tc.volume)
			}
		})
	}
}

func checkPeriodicIndependentBounds(t *testing.T, s Solid) {
	t.Helper()
	bounds, e := s.Bounds()
	if e != nil {
		t.Fatal(e)
	}
	for _, face := range s.faces {
		d, e := faceUVBounds(face)
		if e != nil {
			t.Fatal(e)
		}
		for i := 0; i <= 8; i++ {
			for j := 0; j <= 8; j++ {
				u := d.UMin + (d.UMax-d.UMin)*float64(i)/8
				v := d.VMin + (d.VMax-d.VMin)*float64(j)/8
				p, e := face.Surface.Eval(u, v)
				if e != nil || !bounds.Contains(p) {
					t.Fatal("periodic surface escaped bounds", p, bounds, e)
				}
			}
		}
	}
}

func TestSewPeriodicAffinePolesAndCaps(t *testing.T) {
	ctx := context.Background()
	frame, e := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	if e != nil {
		t.Fatal(e)
	}
	sphere, e := parametric.Sphere(geom.Vec3{}, 1)
	if e != nil {
		t.Fatal(e)
	}
	cylinder, e := parametric.Cylinder(frame, 1, 2)
	if e != nil {
		t.Fatal(e)
	}
	angle := math.Atan(.5)
	cone, e := parametric.Cone(frame, angle, 2)
	if e != nil {
		t.Fatal(e)
	}
	radius := 2 * math.Tan(angle)
	placement := geom.Affine{{-1, .25, 0, .25}, {0, 1, .5, -.5}, {0, 0, 1, .75}}
	for _, tc := range []struct {
		name      string
		s         parametric.Solid
		volume, z float64
	}{
		{"sphere", sphere, 4 * math.Pi / 3, 0}, {"cylinder", cylinder, 2 * math.Pi, 1}, {"cone", cone, 2 * math.Pi * radius * radius / 3, 1.5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, e := tc.s.Transformed(placement)
			if e != nil {
				t.Fatal(e)
			}
			input, e := FromParametric(p)
			if e != nil {
				t.Fatal(e)
			}
			out, _, e := Sew(ctx, periodicIndependentParts(input), SewOptions{Tolerance: input.tolerance})
			if e != nil {
				t.Fatal(e)
			}
			if len(out.edges) != len(input.edges) || out.Topology().CollapsedEdges != 2 || out.Topology().EulerCharacteristic != 2 {
				t.Fatal("placed poles/caps changed")
			}
			proof, e := out.ValidateEdgeAgreement(ctx, edgecheck.Options{})
			if e != nil || !proof.Complete {
				t.Fatal("placed analytic identity", e)
			}
			checkPeriodicIndependentBounds(t, out)
			mass, e := out.MassProperties(ctx, parametric.IntegrationOptions{})
			if e != nil || math.Abs(mass.Volume-tc.volume) > 1e-7 || mass.Centroid.Sub(placement.Apply(geom.Vec3{Z: tc.z})).Length() > 1e-8 {
				t.Fatal(mass, e)
			}
			m, e := out.Tessellate(ctx, parametric.TessellationOptions{Deflection: .03, MaxSubdivisions: 128, MaxTriangles: 250000})
			if e != nil {
				t.Fatal(e)
			}
			if e = m.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
