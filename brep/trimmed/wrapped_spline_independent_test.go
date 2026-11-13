package trimmed

import (
	"context"
	"math"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/geom"
)

// Dyadic weights deliberately describe a rational rounded-square toroidal
// profile, not a circle with rounded irrational weights. Every tensor pole
// and product weight is represented exactly, as are both periodic seams.
func independentWrappedToroid(t *testing.T) *geom.WrappedSurfacePatch {
	t.Helper()
	circle := []geom.Vec2{{X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}, {X: -1, Y: 1}, {X: -1, Y: 0}, {X: -1, Y: -1}, {X: 0, Y: -1}, {X: 1, Y: -1}, {X: 1, Y: 0}}
	var poles []geom.Vec3
	var weights []float64
	for i, u := range circle {
		for j, v := range circle {
			poles = append(poles, geom.Vec3{X: (2 + .5*v.X) * u.X, Y: (2 + .5*v.X) * u.Y, Z: .5 * v.Y})
			a, b := 1., 1.
			if i%2 == 1 {
				a = .75
			}
			if j%2 == 1 {
				b = .75
			}
			weights = append(weights, a*b)
		}
	}
	k := []float64{0, 0, 0, 1, 1, 2, 2, 3, 3, 4, 4, 4}
	s, e := geom.NewBSplineSurface(2, 2, 9, 9, poles, k, k, weights)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, proof, e := geom.NewWrappedSurfacePatch(context.Background(), p, geom.WrappedSurfaceOptions{WrapU: true, WrapV: true, ContinuityOrder: 1})
	if e != nil || proof.FirstPartialsMatched != ([2]bool{true, true}) {
		t.Fatal("exact seams", proof, e)
	}
	return w
}

func TestIndependentWrappedSplineToroidSew(t *testing.T) {
	ctx := context.Background()
	w := independentWrappedToroid(t)
	f, e := WrappedSplineFace(ctx, w, false, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(f.Vertices) != 1 || len(f.Edges) != 4 {
		t.Fatal("wrapped independent topology")
	}
	s, _, e := Sew(ctx, []SewFace{f}, SewOptions{Tolerance: 1e-9})
	if e != nil {
		t.Fatal(e)
	}
	material, embedding, e := NewWrappedToroid(ctx, w, 1e-9, geom.SurfacePatchOptions{})
	if e != nil || !embedding.Complete || !embedding.Outward || len(material.faces) != 1 ||
		embedding.MajorRadius.RatString() != "2" || embedding.RadialMinorRadius.RatString() != "1/2" {
		t.Fatal(embedding, e)
	}
	topo := s.Topology()
	if topo.Vertices != 1 || topo.Edges != 2 || topo.Faces != 1 || topo.EulerCharacteristic != 0 || len(topo.Shells) != 1 {
		t.Fatal(topo)
	}
	seams, closed, poles := periodicIndependentCounts(s)
	if seams != 2 || closed != 2 || poles != 0 {
		t.Fatal(seams, closed, poles)
	}
	proof, e := s.ValidateEdgeAgreement(ctx, edgecheck.Options{})
	if e != nil || !proof.Complete {
		t.Fatal(proof, e)
	}
	// Independent composite Simpson integration of the rational profile's
	// divergence-theorem volume, authored directly from three quadratic poles.
	const n = 4096
	iu, iv := 0., 0.
	cp := []geom.Vec2{{X: 1, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}, {X: -1, Y: 1}, {X: -1, Y: 0}, {X: -1, Y: -1}, {X: 0, Y: -1}, {X: 1, Y: -1}, {X: 1, Y: 0}}
	for arc := 0; arc < 4; arc++ {
		for i := 0; i <= n; i++ {
			t0 := float64(i) / n
			q := 1 - t0
			b := [3]float64{q * q, 1.5 * q * t0, t0 * t0}
			db := [3]float64{-2 * q, 1.5 * (1 - 2*t0), 2 * t0}
			x, y, dx, dy, w, dw := 0., 0., 0., 0., 0., 0.
			for j := 0; j < 3; j++ {
				p := cp[2*arc+j]
				x += b[j] * p.X
				y += b[j] * p.Y
				dx += db[j] * p.X
				dy += db[j] * p.Y
				w += b[j]
				dw += db[j]
			}
			dx = (dx*w - x*dw) / (w * w)
			dy = (dy*w - y*dw) / (w * w)
			x /= w
			y /= w
			factor := 2.
			if i%2 == 1 {
				factor = 4
			}
			if i == 0 || i == n {
				factor = 1
			}
			iu += factor * (x*dy - y*dx) / (3 * n)
			iv += factor * (2 + .5*x) * (2 + .5*x) * .5 * dy / (3 * n)
		}
	}
	want := .5 * iu * iv

	m, e := s.refine(ctx, 5, 0, 0)
	if e != nil || m.ValidateSolid() != nil {
		t.Fatal("periodic mesh", e)
	}
	if math.Abs(m.SignedVolume()-want) > .15 {
		t.Fatal("mesh mass", m.SignedVolume(), want)
	}
}
