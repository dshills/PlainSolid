package parametric

import (
	"bytes"
	"context"
	"math"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestSkewOrthogonalOverlapCommon(t *testing.T) {
	ctx := context.Background()
	for _, v := range [][3]float64{{3, 2, 2}, {3, 2, -2}, {2, 2, 1}, {2, 3, 2}} {
		out, e := SkewOrthogonalOverlapCommon(ctx, geom.Vec3{}, v[0], v[1], v[2], 5, 5, 1e-9)
		if e != nil || len(out.XAxisFaces) != 2 || len(out.ZAxisFaces) != 2 || len(out.Solid.Vertices()) != 4 {
			t.Fatal(v, e, len(out.Solid.Vertices()))
		}
		m, e := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || math.Abs(m.Centroid.X)+math.Abs(m.Centroid.Z) > 1e-9 {
			t.Fatal(v, m, e)
		}
		var volume, moment float64
		n := 8192
		L := v[0] + v[1] - math.Abs(v[2])
		for i := range n {
			u := (float64(i) + .5) * math.Pi / (2 * float64(n))
			s, c := math.Sincos(u)
			y := v[0] - L*c*c
			z := math.Sqrt(v[0]*v[0] - y*y)
			x := math.Sqrt(v[1]*v[1] - (y-math.Abs(v[2]))*(y-math.Abs(v[2])))
			dv := 4 * x * z * 2 * L * s * c * math.Pi / (2 * float64(n))
			volume += dv
			moment += dv * y * math.Copysign(1, v[2])
		}
		if math.Abs(m.Volume-volume) > 1e-8 || math.Abs(m.Centroid.Y-moment/volume) > 1e-8 {
			t.Fatal(v, m, volume, moment/volume)
		}
		if v != [3]float64{3, 2, 2} {
			continue
		}
		var data bytes.Buffer
		if e = WriteJSON(&data, out.Solid); e != nil {
			t.Fatal(e)
		}
		s, e := ReadJSON(&data, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		r, e := s.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || r != m {
			t.Fatal("reload changed overlap", r, e)
		}
		mesh, e := s.Tessellate(ctx, TessellationOptions{Deflection: 1.5, AnalyticBound: true, MaxSubdivisions: 32})
		if e != nil || mesh.ValidateSolid() != nil || mesh.SignedVolume() <= 0 {
			t.Fatal("invalid overlap mesh", e)
		}
		x, e := collision.SelfIntersectionsBVHContext(ctx, mesh, collision.Options{})
		if e != nil || !x.Complete || len(x.Contacts) != 0 {
			t.Fatal("overlap mesh contact", x, e)
		}
	}
}
