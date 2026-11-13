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

func TestAlgebraicSteinmetzCaps(t *testing.T) {
	ctx := context.Background()
	center := geom.Vec3{X: 8, Y: -4, Z: 2}
	for _, cap := range []float64{1, -1, 2, -2} {
		r, e := PerpendicularCylinderAlgebraicCap(ctx, center, 5, cap, 6, 7, 1e-8, geom.ExactCurvePlaneEventOptions{})
		if e != nil {
			t.Fatal(cap, e)
		}
		c := math.Abs(cap)
		a := math.Sqrt(25 - c*c)
		tail := 2*a*a*a/3 + 50*(a-c*math.Acos(c/5))
		want := 2000.0/3 - tail
		faces := 12
		if cap < 0 {
			want = tail
			faces = 4
		}
		if len(r.Solid.Faces()) != faces {
			t.Fatal("topology")
		}
		lo, _ := r.IdealVolumeBounds[0].Float64()
		hi, _ := r.IdealVolumeBounds[1].Float64()
		if hi-lo > 1e-6 || want < lo-1e-10 || want > hi+1e-10 {
			t.Fatal("volume bounds", lo, hi, want)
		}
		delta := big.NewRat(int64(25-cap*cap), 1)
		interval := r.OrdinateInterval
		if new(big.Rat).Mul(interval.Lower, interval.Lower).Cmp(delta) > 0 || new(big.Rat).Mul(interval.Upper, interval.Upper).Cmp(delta) < 0 {
			t.Fatal("root interval")
		}
		r.OrdinateInterval.Lower.SetInt64(99)
		if r.OrdinateRoot.Interval().Lower.Cmp(big.NewRat(99, 1)) == 0 {
			t.Fatal("mutable root")
		}
		var b bytes.Buffer
		if e = WriteJSON(&b, r.Solid); e != nil {
			t.Fatal(e)
		}
		loaded, e := ReadJSON(&b, JSONLimits{})
		if e != nil {
			t.Fatal(e)
		}
		mass, e := loaded.MassProperties(ctx, IntegrationOptions{})
		if e != nil || math.Abs(mass.Volume-want) > 1e-6 {
			t.Fatal(mass.Volume, want, e)
		}
		if math.Abs(cap) == 2 {
			m, e := loaded.Tessellate(ctx, TessellationOptions{Deflection: .3, MaxSubdivisions: 128, MaxTriangles: 100000})
			if e != nil {
				t.Fatal(e)
			}
			if e = m.ValidateSolid(); e != nil {
				t.Fatal(e)
			}
			report, e := collision.SelfIntersectionsBVHContext(ctx, m, collision.Options{})
			if e != nil || !report.Complete || len(report.Contacts) != 0 {
				t.Fatal(e, len(report.Contacts))
			}
		}
	}
}
func TestAlgebraicSteinmetzCapLimits(t *testing.T) {
	for _, o := range []geom.ExactCurvePlaneEventOptions{{MaxOperations: 1}, {MaxSubdivisions: 1}} {
		r, e := PerpendicularCylinderAlgebraicCap(context.Background(), geom.Vec3{}, 5, 1, 6, 7, 1e-8, o)
		if e == nil || len(r.Solid.Faces()) != 0 {
			t.Fatal(r, e)
		}
	}
	if _, e := PerpendicularCylinderPositiveCap(context.Background(), geom.Vec3{}, 5, 1, 6, 7, 1e-8); e == nil {
		t.Fatal("strict API relaxed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := PerpendicularCylinderAlgebraicCap(ctx, geom.Vec3{}, 5, 1, 6, 7, 1e-8, geom.ExactCurvePlaneEventOptions{}); e != context.Canceled {
		t.Fatal(e)
	}
}
