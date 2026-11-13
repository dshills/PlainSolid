package elbow

import (
	"context"
	"math"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
	"testing"
)

func TestCertifiedSignedContinuation(t *testing.T) {
	frame, _ := geom.NewFrame(geom.Vec3{X: 2, Y: 3, Z: -1}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	for _, sign := range []float64{1, -1} {
		a, e := NewSigned(context.Background(), frame, 3, .5, sign*.5)
		if e != nil {
			t.Fatal(e)
		}
		r, e := ContinueCertified(context.Background(), a, sign*.5, trimmed.CertifiedRevolvePolygonOptions{Deflection: .15})
		if e != nil {
			t.Fatal(e)
		}
		c := r.Display.Mesh
		if e := c.Mesh.ValidateSolid(); e != nil {
			t.Fatal(e)
		}
		if !c.Collision.Complete || len(c.Collision.Contacts) != 0 {
			t.Fatal("contacts")
		}
		first, _ := a.VolumePiCoefficient()
		if r.Display.VolumePiCoefficient.Cmp(mul(first, rat(2))) != 0 {
			t.Fatal("additive ideal mass")
		}
		volume, _ := r.Display.VolumePiCoefficient.Float64()
		if math.Abs(c.Mesh.SignedVolume()-math.Pi*volume) > .1 {
			t.Fatal("display approximation", c.Mesh.SignedVolume(), math.Pi*volume)
		}
		if len(c.Solid.Faces()) != 3 || c.IdealBoundarySquaredDistance.Cmp(rat(.15*.15)) > 0 {
			t.Fatal("external shell/certificate")
		}
		owners := map[int]bool{}
		for _, id := range c.OriginalFace {
			owners[id] = true
		}
		if len(owners) != 3 {
			t.Fatal("extra common cap")
		}
		// For sweeps ±1 (<π), all material is on the corresponding side of Y=3.
		// This catches the double sign reversal that a naive adapter would cause.
		for _, p := range c.Mesh.Vertices {
			if sign*(p.Y-3) < -1e-12 {
				t.Fatal("negative frame double reversal", sign, p)
			}
		}
		if c.SignedDisplayVolume.Sign() <= 0 {
			t.Fatal("orientation")
		}
	}
}
func TestCertifiedContinuationAtomicLimit(t *testing.T) {
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	s, _ := NewSigned(context.Background(), f, 3, .5, 1)
	r, e := ContinueCertified(context.Background(), s, 1, trimmed.CertifiedRevolvePolygonOptions{Deflection: .01, MaxTriangles: 4})
	if e == nil || len(r.Display.Mesh.Mesh.Vertices) != 0 {
		t.Fatal("partial result")
	}
}
