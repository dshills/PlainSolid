package geom

import (
	"context"
	"errors"
	"math/big"
	"slices"
	"testing"
)

func originalPhysicalRectangleBase(t *testing.T, skew bool) *SurfacePatch {
	t.Helper()
	var net []Vec3
	for _, y := range []float64{1, 0} {
		for _, x := range []float64{1, 0} {
			p := Vec3{X: 2 + 2*x, Y: -1 + 3*y, Z: 1.0/8 + x/4 + 3*y/16}
			if skew {
				p.X += p.Y / 8
			}
			net = append(net, p)
		}
	}
	base, e := NewBSplineSurface(1, 1, 2, 2, net, []float64{8, 8, 10, 10}, []float64{-2, -2, 2, 2}, nil)
	if e != nil {
		t.Fatal(e)
	}
	p, e := NewSurfacePatch(base, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return p
}

func TestAffineXYRectangleReference(t *testing.T) {
	ctx := context.Background()
	base := originalPhysicalRectangleBase(t, false)
	ref, e := NewAffineXYRectangleReference(ctx, base, SurfacePatchOptions{})
	if e != nil || ref.Domain() != (ParameterRectangle{UMin: 2, UMax: 4, VMin: -1, VMax: 2}) {
		t.Fatal("native physical XY parameter rectangle", e)
	}
	m, e := AlignAffineXYCharts(ctx, ref, base, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	rows, e := m.MapCoefficients()
	want := [2][3]*big.Rat{{new(big.Rat), big.NewRat(-2, 3), big.NewRat(28, 3)}, {big.NewRat(-2, 1), new(big.Rat), big.NewRat(6, 1)}}
	if e != nil {
		t.Fatal(e)
	}
	for i, row := range rows {
		if !slices.EqualFunc(row[:], want[i][:], func(a, b *big.Rat) bool { return a.Cmp(b) == 0 }) {
			t.Fatal("exact original swapped and reflected base map", i, row, want[i])
		}
	}
	u, v := big.NewRat(5, 2), big.NewRat(1, 8)
	z := big.NewRat(33, 128)
	for _, p := range []*SurfacePatch{ref, m.TargetOnSource()} {
		s, e := p.EvaluateRational(ctx, u, v, SurfacePatchEvaluationOptions{})
		if e != nil || s.Point[0].Cmp(u) != 0 || s.Point[1].Cmp(v) != 0 {
			t.Fatal("physical chart evaluates exactly in model XY", e)
		}
		if p == ref && s.Point[2].Sign() != 0 || p != ref && s.Point[2].Cmp(z) != 0 {
			t.Fatal("reference and actual sloping original height", s.Point[2], z)
		}
	}
	for _, limit := range []int{ref.ExactOperations() - 1, ref.ExactOperations()} {
		got, e := NewAffineXYRectangleReference(ctx, base, SurfacePatchOptions{MaxOperations: limit})
		if (e == nil && got != nil) != (limit == ref.ExactOperations()) {
			t.Fatal("one cumulative reference construction budget", limit, e)
		}
	}
}

func TestAffineXYRectangleReferenceRefusals(t *testing.T) {
	ctx := context.Background()
	base := originalPhysicalRectangleBase(t, false)
	net := [][4]*big.Rat{
		{big.NewRat(1, 3), new(big.Rat), new(big.Rat), big.NewRat(1, 1)},
		{big.NewRat(1, 3), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)},
		{big.NewRat(1, 1), new(big.Rat), new(big.Rat), big.NewRat(1, 1)},
		{big.NewRat(1, 1), big.NewRat(1, 1), new(big.Rat), big.NewRat(1, 1)},
	}
	badCorner, e := NewExactHomogeneousBezierSurface(ctx, 1, 1, net, SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	rotation := Identity()
	rotation[0][1], rotation[1][0] = -1, 1
	rotated, e := base.Transformed(rotation)
	if e != nil {
		t.Fatal(e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	for _, tc := range []struct {
		ctx context.Context
		p   *SurfacePatch
		err error
	}{
		{ctx, originalPhysicalRectangleBase(t, true), ErrDomain},
		{ctx, rotated, ErrDomain},
		{ctx, badCorner, ErrUnrepresentable},
		{cancelled, base, context.Canceled},
	} {
		if got, e := NewAffineXYRectangleReference(tc.ctx, tc.p, SurfacePatchOptions{}); got != nil || !errors.Is(e, tc.err) {
			t.Fatal("refuse an unsupported exact physical reference atomically", tc.err, e)
		}
	}
}

func TestProjectedPolynomialVolumesOnPhysicalRectangle(t *testing.T) {
	ctx := context.Background()
	ref, e := NewAffineXYRectangleReference(ctx, originalPhysicalRectangleBase(t, false), SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	stations := []*big.Rat{big.NewRat(1, 4), big.NewRat(3, 4)}
	delta := arrangementGap(t, stations, []int{3, 2}, rationalPolynomial{big.NewRat(-1, 1)})
	xform := Identity()
	xform[0][0], xform[0][3], xform[1][1], xform[1][3] = 2, 2, 3, -1
	for axis := 0; axis < 2; axis++ {
		physical, _, _ := polynomialPairGraphFixture(t, axis, delta)
		a, e := independentProjectedPolynomialRoof(physical).Transformed(xform)
		if e != nil {
			t.Fatal(e)
		}
		b, e := independentPolynomialArrangementRoof(t, axis, 1, 25.0/8).Transformed(xform)
		if e != nil {
			t.Fatal(e)
		}
		for order, roof := range [][2]*SurfacePatch{{a, b}, {b, a}} {
			measured, e := IntegrateProjectedSplineGraphPolynomialVolumes(ctx, ref, roof[0], roof[1], axis, GraphPointContactOptions{})
			if e != nil || !measured.Projection.Arrangement.Proven || len(measured.Projection.Arrangement.CertifiedCells) != 6 {
				t.Fatal("aligned original roofs in exact physical units", axis, order, e)
			}
			// Antiderivatives in unit chart times the independent XY Jacobian six.
			want := [2][]*big.Rat{
				{big.NewRat(28550399214829, 2063645886382080), big.NewRat(590037023506439, 6190937659146240)},
				{big.NewRat(4909741, 2063645886382080), big.NewRat(1, 31488737280), big.NewRat(160842989, 2063645886382080)},
			}[order]
			for i, v := range want {
				v.Mul(v, big.NewRat(6, 1))
				if measured.Volumes[i].Cmp(v) != 0 {
					t.Fatal("independent physical layer volume", axis, order, i, measured.Volumes[i], v)
				}
			}
			contacts := measured.Projection.Arrangement.Vertices[4:]
			for i, vertex := range contacts {
				p := new(big.Rat).Set(stations[i])
				if axis == 0 {
					p.Mul(p, big.NewRat(2, 1)).Add(p, big.NewRat(2, 1))
				} else {
					p.Mul(p, big.NewRat(3, 1)).Sub(p, big.NewRat(1, 1))
				}
				if vertex.Parameters[axis].Cmp(p) != 0 || vertex.Point[axis].Cmp(p) != 0 {
					t.Fatal("contact station is retained in physical model coordinates", i, vertex)
				}
			}
		}
	}
}
