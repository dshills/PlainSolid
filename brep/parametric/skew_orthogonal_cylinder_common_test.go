package parametric

import (
	"bytes"
	"context"
	"errors"
	"math"
	"testing"

	"plainsolid/collision"
	"plainsolid/geom"
)

func TestSkewOrthogonalCylinderCommon(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 2, Y: -1, Z: 3}
	for _, p := range [][3]float64{{3, 2, .5}, {2, 3, .5}, {3, 2, -.5}} {
		out, e := SkewOrthogonalCylinderCommon(ctx, center, p[0], p[1], p[2], 5, 5, 1e-9)
		if e != nil || len(out.XAxisFaces) != 2 || len(out.ZAxisFaces) != 2 {
			t.Fatal(p, e, out)
		}
		m, e := out.Solid.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || math.Abs(m.Centroid.X-center.X)+math.Abs(m.Centroid.Z-center.Z) > 1e-8 {
			t.Fatal(p, m, e)
		}
		// A slice in common y is a rectangle of area 4 X(y) Z(y).
		var volume, moment float64
		const n = 16384
		small, offset, mid, large := p[1], -p[2], p[2], p[0]
		if p[0] < p[1] {
			small, offset, mid, large = p[0], p[2], 0, p[1]
		}
		for i := range n {
			u := ((float64(i)+.5)/n - .5) * math.Pi
			sn, cs := math.Sincos(u)
			y := small * sn
			w := y - offset
			dv := 4 * small * small * cs * cs * math.Sqrt(large*large-w*w) * math.Pi / n
			volume += dv
			moment += dv * (y + mid)
		}
		if math.Abs(volume-m.Volume) > 1e-8 || math.Abs(moment/volume+center.Y-m.Centroid.Y) > 1e-8 {
			t.Fatal(p, m, volume, moment/volume)
		}
		if p != [3]float64{3, 2, .5} {
			continue
		}
		var data bytes.Buffer
		if e := WriteJSON(&data, out.Solid); e != nil {
			t.Fatal(e)
		}
		reloaded, e := ReadJSON(&data, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		m2, e := reloaded.MassProperties(ctx, IntegrationOptions{RelativeTolerance: 1e-7})
		if e != nil || m2 != m {
			t.Fatal("reload changed skew cylinder common", m2, e)
		}
		display, e := reloaded.Tessellate(ctx, TessellationOptions{Deflection: 1.5, AnalyticBound: true, MaxSubdivisions: 32})
		if e != nil || display.ValidateSolid() != nil || display.SignedVolume() <= 0 {
			t.Fatal("invalid skew cylinder mesh", e)
		}
		c, e := collision.SelfIntersectionsBVHContext(ctx, display, collision.Options{})
		if e != nil || !c.Complete || len(c.Contacts) != 0 {
			t.Fatal("skew common mesh contacts", c, e)
		}
	}
	if r, e := SkewOrthogonalCylinderCommon(ctx, center, 3, 2, 5.25, 5, 5, 1e-9); e != nil || len(r.Solid.Faces()) != 0 {
		t.Fatal("disjoint", e)
	}
	for _, d := range []float64{2, 5} {
		if r, e := SkewOrthogonalCylinderCommon(ctx, center, 3, 2, d, 5, 5, 1e-9); !errors.Is(e, geom.ErrConvergence) || len(r.Solid.Faces()) != 0 {
			t.Fatal("unsupported", d, e)
		}
	}
}
