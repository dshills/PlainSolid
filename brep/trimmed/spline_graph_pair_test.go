package trimmed

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestSealSplineGraphPair(t *testing.T) {
	k := []float64{0, 0, 0, 1, 1, 1}
	q := [3]float64{.25, -.25, .25}
	const radiusSquared = .1171875
	var lo, hi []geom.Vec3
	for i := range 3 {
		for j := range 3 {
			// Both caps are curved. Their middle height is h=1+.5*rho²
			// and their signed separation is radiusSquared-rho².
			x, y, r2 := float64(i)/2, float64(j)/2, q[i]+q[j]
			lo = append(lo, geom.Vec3{X: x, Y: y, Z: 1 - radiusSquared/2 + r2})
			hi = append(hi, geom.Vec3{X: x, Y: y, Z: 1 + radiusSquared/2})
		}
	}
	// Add the same quadratic saddle to both caps, preserving the difference.
	for i := range lo {
		z := .25 * (q[i/3] - q[i%3])
		lo[i].Z += z
		hi[i].Z += z
	}
	build := func(p []geom.Vec3) *geom.BSplineSurface {
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	ctx := context.Background()
	a, b := build(lo), build(hi)
	for _, pair := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
		s, h, e := SealSplineGraphPair(ctx, pair[0], pair[1], 1e-7, intersect.SplinePlaneOptions{}, geom.SurfaceInjectivityOptions{})
		if e != nil || len(h) != 1 {
			t.Fatal(h, e)
		}
		if s.faces[h[0].FaceA].Surface != pair[0] || s.faces[h[0].FaceB].Surface != pair[1] || s.Topology().EulerCharacteristic != 2 {
			t.Fatal(h, s.Topology())
		}
		var data bytes.Buffer
		if e = WriteJSON(&data, s); e != nil {
			t.Fatal(e)
		}
		s, e = ReadJSON(&data, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6})
		volume := math.Pi * radiusSquared * radiusSquared / 2
		z := 1 + radiusSquared/6
		if e != nil || math.Abs(p.Volume-volume) > 1e-8 || p.Centroid.Distance(geom.Vec3{X: .5, Y: .5, Z: z}) > 1e-7 {
			t.Fatal(p, volume, z, e)
		}
	}
	hi[0].X += .01
	if _, _, e := SealSplineGraphPair(ctx, a, build(hi), 1e-7, intersect.SplinePlaneOptions{}, geom.SurfaceInjectivityOptions{}); e == nil {
		t.Fatal("accepted unmatched projected charts")
	}
	hi[0].X = lo[0].X
	hi[0].Z = math.Ldexp(1, -54)
	s, h, e := SealSplineGraphPair(ctx, a, build(hi), 1e-7, intersect.SplinePlaneOptions{}, geom.SurfaceInjectivityOptions{})
	if e != nil || len(h) != 1 || s.Topology().EulerCharacteristic != 2 {
		t.Fatal("exact nonrepresentable pole difference", h, s.Topology(), e)
	}
	var native bytes.Buffer
	if e = WriteJSON(&native, s); e != nil {
		t.Fatal(e)
	}
	s, e = ReadJSON(&native, JSONLimits{})
	if e != nil || s.Topology().EulerCharacteristic != 2 {
		t.Fatal("exact difference persistence", s.Topology(), e)
	}
}

func TestSealOpenSplineGraphPair(t *testing.T) {
	k := []float64{0, 0, 0, 1, 1, 1}
	var ap, bp []geom.Vec3
	for i := range 3 {
		for j := range 3 {
			z := 1.
			if j == 2 {
				z += .25
			}
			b := geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z}
			a := b
			a.Z -= .25
			if i == 2 {
				a.Z++
			}
			ap, bp = append(ap, a), append(bp, b)
		}
	}
	makeSpline := func(p []geom.Vec3) *geom.BSplineSurface {
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	a, b := makeSpline(ap), makeSpline(bp)
	ctx := context.Background()
	for _, above := range []bool{true, false} {
		s, h, e := SealOpenSplineGraphPair(a, b, above, 1e-8, intersect.SplinePlaneOptions{})
		if e != nil || len(h.WallFaces) != 3 || s.Topology().EulerCharacteristic != 2 ||
			s.faces[h.FaceA].Surface != a || s.faces[h.FaceB].Surface != b {
			t.Fatal(h, s.Topology(), e)
		}
		var native bytes.Buffer
		if e = WriteJSON(&native, s); e != nil {
			t.Fatal(e)
		}
		s, e = ReadJSON(&native, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		props, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7})
		wantV, wantX := 1./12, 3./16
		if above {
			wantV, wantX = 1./6, 27./32
		}
		if e != nil || math.Abs(props.Volume-wantV) > 1e-8 ||
			math.Abs(props.Centroid.X-wantX) > 1e-8 || math.Abs(props.Centroid.Y-.5) > 1e-8 {
			t.Fatal(above, props, wantV, wantX, e)
		}
		mesh, e := s.Tessellate(ctx, parametric.TessellationOptions{Deflection: .02})
		if e != nil || mesh.ValidateSolid() != nil || math.Abs(mesh.SignedVolume()-wantV) > .006 {
			t.Fatal(above, mesh.SignedVolume(), wantV, e)
		}
	}
}

func TestFillSplineGraphPair(t *testing.T) {
	k := []float64{0, 0, 0, 1, 1, 1}
	makeSpline := func(c float64) *geom.BSplineSurface {
		t.Helper()
		var p []geom.Vec3
		for i := range 3 {
			for j := range 3 {
				// Both surfaces have identical paraboloid curvature.
				x, y, z := float64(i)/2, float64(j)/2, c
				if i == 2 {
					z += .25
				}
				if j == 2 {
					z += .5
				}
				p = append(p, geom.Vec3{X: x, Y: y, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, p, k, k, nil)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	ctx := context.Background()
	a, b := makeSpline(1), makeSpline(.5)
	for _, pair := range [][2]*geom.BSplineSurface{{a, b}, {b, a}} {
		s, h, e := FillSplineGraphPair(pair[0], pair[1], 1e-8, intersect.SplinePlaneOptions{})
		if e != nil || len(h.WallFaces) != 4 || len(h.ContourEdges) != 0 || s.Topology().EulerCharacteristic != 2 {
			t.Fatal(h, s.Topology(), e)
		}
		p, e := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || math.Abs(p.Volume-.5) > 1e-9 || p.Centroid.Distance(geom.Vec3{X: .5, Y: .5, Z: 1}) > 1e-8 {
			t.Fatal(p, e)
		}
	}
	if _, _, e := FillSplineGraphPair(a, a, 1e-8, intersect.SplinePlaneOptions{}); e == nil {
		t.Fatal("accepted coincident surfaces as a regular solid")
	}
}
