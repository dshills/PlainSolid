package parametric

import (
	"context"
	"math"
	"plainsolid/collision"
	"plainsolid/geom"
	"testing"
)

func TestAlgebraicFiniteSphereCommon(t *testing.T) {
	f, _ := geom.NewFrame(geom.Vec3{X: 1, Y: -2}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	for _, rangeZ := range [][2]float64{{-4.75, 4.875}, {0, 4.875}, {-2, 2}, {4.625, 4.875}} {
		lo, hi := rangeZ[0], rangeZ[1]
		r, e := SphereFiniteCylinderCommonAlgebraic(context.Background(), f, 5, 2, lo, hi, 1e-8, geom.ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(rangeZ, e)
		}
		primitive := func(z float64) float64 { return 25*z - z*z*z/3 }
		cross := math.Sqrt(21)
		cuts := []float64{lo}
		if -cross > lo && -cross < hi {
			cuts = append(cuts, -cross)
		}
		if cross > lo && cross < hi {
			cuts = append(cuts, cross)
		}
		cuts = append(cuts, hi)
		want := 0.
		for i := 0; i+1 < len(cuts); i++ {
			a, b := cuts[i], cuts[i+1]
			mid := (a + b) / 2
			if 25-mid*mid > 4 {
				want += 4 * (b - a)
			} else {
				want += primitive(b) - primitive(a)
			}
		}
		lower, _ := r.VolumePiInterval[0].Float64()
		upper, _ := r.VolumePiInterval[1].Float64()
		if want < lower-1e-12 || want > upper+1e-12 {
			t.Fatal("volume interval", lower, want, upper)
		}
		props, e := r.Solid.MassProperties(context.Background(), IntegrationOptions{})
		if e != nil || math.Abs(props.Volume-math.Pi*want) > 1e-6 {
			t.Fatal("native volume", e, props.Volume, want*math.Pi)
		}
		for _, rim := range r.Rims {
			if rim.SeamSquaredBound.Sign() < 0 {
				t.Fatal("seam")
			}
			bound, _ := rim.SeamSquaredBound.Float64()
			if bound > 1e-16/16 {
				t.Fatal("seam too large", bound)
			}
		}
		if lo == 0 {
			m, e := r.Solid.Tessellate(context.Background(), TessellationOptions{Deflection: .15, MaxSubdivisions: 128, MaxTriangles: 100000})
			if e != nil {
				t.Fatal(e)
			}
			if e = m.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
			report, e := collision.SelfIntersectionsBVHContext(context.Background(), m, collision.Options{})
			if e != nil || !report.Complete || len(report.Contacts) != 0 {
				t.Fatal("contacts", e, len(report.Contacts))
			}
		}
	}
}
func TestAlgebraicFiniteCommonRejects(t *testing.T) {
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	for _, z := range [][2]float64{{math.Sqrt(21), 4.9}, {2, 1}} {
		out, e := SphereFiniteCylinderCommonAlgebraic(context.Background(), f, 5, 2, z[0], z[1], 1e-8, geom.ExactCurvePlaneEventOptions{})
		if e == nil || len(out.Solid.Faces()) != 0 {
			t.Fatal("accepted", z)
		}
	}
	out, e := SphereFiniteCylinderCommonAlgebraic(context.Background(), f, 5, 2, -4.75, 4.875, 1e-8, geom.ExactCurvePlaneEventOptions{MaxOperations: 1})
	if e == nil || len(out.Solid.Faces()) != 0 {
		t.Fatal("partial budget")
	}
}

func TestAlgebraicFiniteCommonRetainsPoles(t *testing.T) {
	ctx := context.Background()
	f, _ := geom.FrameFromNormal(geom.Vec3{}, geom.Vec3{Z: 1})
	for _, z := range [][2]float64{{-5, 4}, {4, 5}, {-6, 6}} {
		out, e := SphereFiniteCylinderCommonAlgebraic(ctx, f, 5, 2, z[0], z[1], 1e-8, geom.ExactCurvePlaneEventOptions{})
		if e != nil || len(out.SphericalFaces) == 0 {
			t.Fatal(z, e)
		}
		for i, h := range z {
			if math.Abs(h) < 5 {
				continue
			}
			rim := out.Rims[i*(len(out.Rims)-1)]
			if rim.NativeRadius != 0 || rim.RadiusRoot != nil || rim.Radius.Lower.Sign() != 0 || math.Abs(rim.Latitude) != math.Pi/2 {
				t.Fatal(z, rim)
			}
		}
	}
}

func TestAlgebraicFiniteCommonSphereOnly(t *testing.T) {
	f, _ := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1, Y: 2}, geom.Vec3{Z: 1})
	r, e := SphereFiniteCylinderCommonAlgebraic(context.Background(), f, 5, 6, -2, 2, 1e-8, geom.ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.CylindricalFaces) != 0 || len(r.SphericalFaces) != 4 || len(r.Rims) != 2 || r.Rims[0].RadiusRoot == nil {
		t.Fatal("sphere-only ancestry")
	}
	mass, e := r.Solid.MassProperties(context.Background(), IntegrationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	a, _ := r.VolumePiInterval[0].Float64()
	b, _ := r.VolumePiInterval[1].Float64()
	if math.Abs(mass.Volume-math.Pi*(a+b)/2) > 1e-6 {
		t.Fatal("affine volume")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := SphereFiniteCylinderCommonAlgebraic(ctx, f, 5, 2, -4.75, 4.875, 1e-8, geom.ExactCurvePlaneEventOptions{}); e == nil {
		t.Fatal("cancel")
	}
}
