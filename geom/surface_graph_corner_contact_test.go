package geom

import (
	"context"
	"errors"
	"testing"
)

func cornerContactPatch(t *testing.T, z [4]float64, swap bool) *SurfacePatch {
	t.Helper()
	p := []Vec3{{X: 0, Y: 0, Z: z[0]}, {X: 0, Y: 1, Z: z[1]}, {X: 1, Y: 0, Z: z[2]}, {X: 1, Y: 1, Z: z[3]}}
	if swap {
		for i := range p {
			p[i].X, p[i].Y = p[i].Y, p[i].X
		}
	}
	s, e := NewBSplineSurface(1, 1, 2, 2, p, []float64{0, 0, 1, 1}, []float64{0, 0, 1, 1}, nil)
	if e != nil {
		t.Fatal(e)
	}
	r, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestGraphCornerContact(t *testing.T) {
	ctx := context.Background()
	a := cornerContactPatch(t, [4]float64{0, 0, 0, .125}, false)
	b := cornerContactPatch(t, [4]float64{0, .25, .25, .5}, false)
	for _, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
		r, e := CertifyGraphCornerContact(ctx, pair[0], pair[1], SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
		want := -1
		if pair[0] == b {
			want = 1
		}
		if r.HeightSign() != want || !r.Binds(pair[0], pair[1]) || r.Binds(pair[1], pair[0]) || r.ExactOperations() <= 0 {
			t.Fatal("binding")
		}
		uv := r.Parameters()
		for _, v := range uv {
			for _, x := range v {
				if x.Sign() != 0 {
					t.Fatal(x)
				}
			}
		}
		uv[0][0].SetInt64(9)
		if r.Parameters()[0][0].Sign() != 0 {
			t.Fatal("alias")
		}
		p := r.Point()
		p[0].SetInt64(9)
		if r.Point()[0].Sign() != 0 {
			t.Fatal("point alias")
		}
		if r.Domains() != ([2]ParameterRectangle{a.Domain(), b.Domain()}) {
			t.Fatal("domains")
		}
	}
	// Positive nonconstant rational weights with exactly affine XY: W=1+2u,
	// weighted Bernstein X controls [0,1/2,3], weights [1,2,3].
	var poles []Vec3
	var weights []float64
	for i, x := range []float64{0, .25, 1} {
		for _, y := range []float64{0, 1} {
			poles = append(poles, Vec3{X: x, Y: y, Z: x + y})
			weights = append(weights, float64(i+1))
		}
	}
	s, e := NewBSplineSurface(2, 1, 3, 2, poles, []float64{0, 0, 0, 1, 1, 1}, []float64{0, 0, 1, 1}, weights)
	if e != nil {
		t.Fatal(e)
	}
	rounded, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if proof, e := CertifyGraphCornerContact(ctx, rounded, cornerContactPatch(t, [4]float64{}, false), SurfacePatchOptions{}); e == nil || proof != nil {
		t.Fatal("rounded weight identity accepted")
	}
	// Supply the exact homogeneous weight field: the public constructor normalizes
	// by max=3 in binary64, which would change this exact affine identity.
	s.weights = append([]float64(nil), weights...)
	weighted, e := NewSurfacePatch(s, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = NewAffineXYGraph(ctx, weighted, SurfacePatchOptions{}); e != nil {
		t.Fatal("weighted affine", e)
	}
	flat := cornerContactPatch(t, [4]float64{}, false)
	if _, e = CertifyGraphCornerContact(ctx, weighted, flat, SurfacePatchOptions{}); e != nil {
		t.Fatal("positive weights", e)
	}
}
func TestGraphCornerContactReject(t *testing.T) {
	ctx := context.Background()
	flat := cornerContactPatch(t, [4]float64{}, false)
	for _, z := range [][4]float64{{0, 0, 1, 1}, {0, 1, -1, 1}, {0, 0, 0, 0}, {1, 1, 1, 1}} {
		r, e := CertifyGraphCornerContact(ctx, cornerContactPatch(t, z, false), flat, SurfacePatchOptions{})
		if e == nil || r != nil {
			t.Fatal("false contact certificate", z, e)
		}
	}
	if r, e := CertifyGraphCornerContact(ctx, cornerContactPatch(t, [4]float64{0, 1, 1, 2}, true), flat, SurfacePatchOptions{}); e == nil || r != nil {
		t.Fatal("unproved chart correspondence")
	}
	for _, o := range []SurfacePatchOptions{{MaxOperations: 1}, {MaxPoles: 1}} {
		r, e := CertifyGraphCornerContact(ctx, flat, flat, o)
		if e == nil || r != nil {
			t.Fatal("budget")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := CertifyGraphCornerContact(canceled, flat, flat, SurfacePatchOptions{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	// Domain identity is part of the evidence, never inferred from proximity.
	sub, e := flat.Trim(ParameterRectangle{UMin: 0, UMax: .5, VMin: 0, VMax: 1})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = CertifyGraphCornerContact(ctx, sub, flat, SurfacePatchOptions{}); !errors.Is(e, ErrDomain) {
		t.Fatal(e)
	}
}
