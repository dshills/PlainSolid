package geom

import (
	"context"
	"errors"
	"math"
	"testing"
)

func TestCertifiedTessellationCircleSagitta(t *testing.T) {
	c, _ := NewBezier([]Vec3{{1, 0, 0}, {1, 1, 0}, {0, 1, 0}}, []float64{1, math.Sqrt(.5), 1})
	r, e := TessellateCurveCertified(c, CertifiedTessellationOptions{Deflection: 1e-3})
	if e != nil || !r.Complete || len(r.Segments) < 2 {
		t.Fatal(r, e)
	}
	for i, s := range r.Segments {
		if s.ErrorUpper > 1e-3 {
			t.Fatal(s)
		}
		angle := math.Atan2(s.End.Y, s.End.X) - math.Atan2(s.Start.Y, s.Start.X)
		sagitta := 1 - math.Cos(angle/2)
		if sagitta > s.ErrorUpper+1e-14 {
			t.Fatal(s, sagitta)
		}
		if i > 0 && (r.Segments[i-1].Domain.End != s.Domain.Start || r.Segments[i-1].End != s.Start) {
			t.Fatal("disconnected", r)
		}
		for j := 0; j <= 10; j++ {
			u := float64(j) / 10
			point, _ := c.Eval(s.Domain.Start + (s.Domain.End-s.Domain.Start)*u)
			chord := s.Start.Scale(1 - u).Add(s.End.Scale(u))
			if point.Sub(chord).Length() > s.ErrorUpper+1e-14 {
				t.Fatal(s, u)
			}
		}
	}
}
func TestCertifiedTessellationScaleAndKnots(t *testing.T) {
	for _, scale := range []float64{1e-200, 1, 1e150} {
		c, _ := NewBezier([]Vec3{{}, {scale, 2 * scale, 0}, {2 * scale, 0, 0}}, nil)
		c, _ = c.InsertKnot(.3, 1)
		r, e := TessellateCurveCertified(c, CertifiedTessellationOptions{Deflection: .01 * scale})
		if e != nil || !r.Complete {
			t.Fatal(scale, r, e)
		}
		boundary := false
		for _, s := range r.Segments {
			if s.Domain.End == .3 {
				boundary = true
			}
			if s.ErrorUpper > .01*scale {
				t.Fatal(scale, s)
			}
		}
		if !boundary {
			t.Fatal("knot omitted")
		}
	}
}
func TestCertifiedTessellationRoundoffAndLimits(t *testing.T) {
	// Extracted constant X=2/3 cannot be represented by the endpoint float.
	s, _ := NewBSplineSurface(1, 1, 2, 2, []Vec3{{0, 0, 0}, {0, 1, 0}, {1, 0, 0}, {1, 1, 0}}, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, []float64{1, 1, 2, 2})
	p, _ := NewSurfacePatch(s, SurfacePatchOptions{})
	iso, _ := p.IsoCurve(0, .5, SurfacePatchOptions{})
	r, e := iso.TessellateCertified(CertifiedTessellationOptions{Deflection: 1e-15})
	if e != nil || !r.Complete || len(r.Segments) != 1 || r.Segments[0].ErrorUpper <= 0 {
		t.Fatal(r, e)
	}
	r, e = iso.TessellateCertified(CertifiedTessellationOptions{Deflection: 1e-20, MaxSegments: 2})
	if !errors.Is(e, ErrConvergence) || r.Complete {
		t.Fatal(r, e)
	}
	c, _ := NewBezier([]Vec3{{}, {1, 2, 0}, {2, 0, 0}}, nil)
	r, e = TessellateCurveCertified(c, CertifiedTessellationOptions{MaxSegments: 1})
	if !errors.Is(e, ErrConvergence) || r.Complete {
		t.Fatal(r, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e = TessellateCurveCertifiedContext(ctx, c, CertifiedTessellationOptions{})
	if !errors.Is(e, context.Canceled) || r.Complete {
		t.Fatal(r, e)
	}
}

func TestCertifiedTessellationCancellationRetainsPrefix(t *testing.T) {
	c, _ := NewBezier([]Vec3{{}, {1, 2, 0}, {2, 0, 0}}, nil)
	r, e := TessellateCurveCertifiedContext(distanceCancelContext(4), c, CertifiedTessellationOptions{Deflection: .5})
	if !errors.Is(e, context.Canceled) || r.Complete || len(r.Segments) != 1 || r.Segments[0].Domain.Start != 0 || r.Segments[0].Domain.End != .5 {
		t.Fatal(r, e)
	}
}
