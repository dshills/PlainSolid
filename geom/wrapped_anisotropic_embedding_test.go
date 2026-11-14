package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func anisotropicSource(t *testing.T, change func(*BSplineSurfaceData)) *WrappedSurfacePatch {
	t.Helper()
	xy := [9][2]float64{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}, {1, 0}}
	var poles []Vec3
	var weights []float64
	for i, q := range xy {
		for j := 0; j < 2; j++ {
			sx, sy := 3., 1.
			if j == 1 {
				sx, sy = 1, 2
			}
			poles = append(poles, Vec3{X: .25 + sx*q[0], Y: -.5 + sy*q[1], Z: float64(j)})
			w := 1.
			if i%2 == 1 {
				w = .75
			}
			weights = append(weights, w)
		}
	}
	s, e := NewBSplineSurface(2, 1, 9, 2, poles, []float64{-2, -2, -2, -1, -1, 1, 1, 4, 4, 8, 8, 8}, []float64{2, 2, 5, 5}, weights)
	if e != nil {
		t.Fatal(e)
	}
	if change != nil {
		data := s.Data()
		change(&data)
		s, e = NewBSplineSurface(data.DegreeU, data.DegreeV, data.CountU, data.CountV, data.Poles, data.KnotsU, data.KnotsV, data.Weights)
		if e != nil {
			t.Fatal(e)
		}
	}
	p, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	w, _, e := NewWrappedSurfacePatch(context.Background(), p, WrappedSurfaceOptions{WrapU: true})
	if e != nil {
		t.Fatal(e)
	}
	return w
}

func TestWrappedAnisotropicEmbeddingAndSection(t *testing.T) {
	ctx := context.Background()
	r := big.NewRat
	w := anisotropicSource(t, nil)
	proof, e := CertifyWrappedAnisotropicEmbedding(ctx, w, SurfacePatchOptions{})
	if e != nil || !proof.Complete {
		t.Fatal(proof, e)
	}
	if _, e = CertifyWrappedFrustumEmbedding(ctx, w, SurfacePatchOptions{}); e == nil {
		t.Fatal("old homothetic gate widened")
	}
	section, e := SectionWrappedAnisotropicRational(ctx, w, r(1, 3), SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if section.Scale[0].Cmp(r(7, 3)) != 0 || section.Scale[1].Cmp(r(4, 3)) != 0 || section.ParameterV.Cmp(r(3, 1)) != 0 {
		t.Fatal(section)
	}
	// Mid-first quadrant normalized rational point is (5/7,5/7),
	// independently evaluated from controls (1,0),(1,1),(0,1), w=3/4.
	u := r(-3, 2)
	jet, e := section.Curve.patch.EvaluateRational(ctx, u, r(0, 1), SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	want := [3]*big.Rat{r(23, 12), r(19, 42), r(1, 3)}
	for i := range want {
		if jet.Point[i].Cmp(want[i]) != 0 {
			t.Fatal("independent section", jet.Point, want)
		}
	}
	original, e := w.patch.EvaluateRational(ctx, u, section.ParameterV, SurfacePatchEvaluationOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for i := range want {
		if original.Point[i].Cmp(jet.Point[i]) != 0 {
			t.Fatal("source identity")
		}
	}
	// Nonzero circumferential tangent and strictly positive height derivative.
	if original.Dv[2].Sign() <= 0 || original.Du[0].Sign() == 0 && original.Du[1].Sign() == 0 {
		t.Fatal("lost regularity")
	}
	proof.Center[0].SetInt64(99)
	section.Scale[0].SetInt64(99)
	again, e := CertifyWrappedAnisotropicEmbedding(ctx, w, SurfacePatchOptions{})
	if e != nil || again.Center[0].Cmp(r(1, 4)) != 0 {
		t.Fatal("mutated source")
	}
}

func TestWrappedAnisotropicGuards(t *testing.T) {
	ctx := context.Background()
	w := anisotropicSource(t, nil)
	for _, change := range []func(*BSplineSurfaceData){
		func(d *BSplineSurfaceData) { d.Poles[2].X += .125 },
		func(d *BSplineSurfaceData) { d.Weights[3] = .5 },
		func(d *BSplineSurfaceData) {
			for i := 1; i < len(d.Poles); i += 2 {
				d.Poles[i].Y = -.5
			}
		},
		func(d *BSplineSurfaceData) {
			for i := 1; i < len(d.Poles); i += 2 {
				d.Poles[i].X += .125
			}
		},
		func(d *BSplineSurfaceData) {
			for i := 1; i < len(d.Poles); i += 2 {
				d.Poles[i].Z = 0
			}
		},
	} {
		bad := anisotropicSource(t, change)
		if got, e := CertifyWrappedAnisotropicEmbedding(ctx, bad, SurfacePatchOptions{}); e == nil || got.Complete {
			t.Fatal("accepted noncanonical source", e)
		}
	}
	for _, o := range []SurfacePatchOptions{{MaxOperations: 1}, {MaxPoles: 17}, {MaxCoefficientBits: 1}} {
		if got, e := SectionWrappedAnisotropicRational(ctx, w, big.NewRat(1, 3), o); e == nil || got.Curve != nil {
			t.Fatal("partial result on quota", e)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := CertifyWrappedAnisotropicEmbedding(cancelled, w, SurfacePatchOptions{}); !errors.Is(e, context.Canceled) || got.Complete {
		t.Fatal(got, e)
	}
	if got, e := SectionWrappedAnisotropicRational(ctx, w, big.NewRat(2, 1), SurfacePatchOptions{}); !errors.Is(e, ErrDomain) || got.Curve != nil {
		t.Fatal(got, e)
	}
}
