package geom_test

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func TestRadialCentroidOriginalNonradialHeights(t *testing.T) {
	for _, shift := range []float64{0, -2} {
		p := nonradialRoofGapShift(t, shift)
		ctx := context.Background()
		m, e := p.MassMomentsBands(ctx, []int{0}, geom.RadialGraphMomentOptions{})
		if e != nil {
			t.Fatal(e)
		}
		// Mean of common uv/64 over a centered circle is cx*cy/64.
		// For H=r^2-q the volume-weighted mid-height is r^2/3.
		want := new(big.Rat).Add(big.NewRat(15, 4096), big.NewRat(1, 192))
		want.Add(want, new(big.Rat).SetFloat64(shift))
		if m.Height.CentroidZ.Cmp(want) != 0 || len(m.Bands) != 1 || m.Bands[0].Moment.CentroidZ.Cmp(want) != 0 {
			t.Fatal("original Z centroid", m.Height.CentroidZ, want)
		}
		if m.Height.FirstZPiCoefficient.Cmp(new(big.Rat).Mul(want, m.Planar.Total.VolumePiCoefficient)) != 0 {
			t.Fatal("Z first moment")
		}
		work := m.ExactOperations
		if _, e = p.MassMomentsBands(ctx, []int{0}, geom.RadialGraphMomentOptions{MaxOperations: work}); e != nil {
			t.Fatal("exact budget", e)
		}
		if out, e := p.MassMomentsBands(ctx, []int{0}, geom.RadialGraphMomentOptions{MaxOperations: work - 1}); e == nil || out.Height.CentroidZ != nil || len(out.Bands) != 0 {
			t.Fatal("atomic cumulative quota", e)
		}
		m.Height.CentroidZ.SetInt64(100)
		m.Bands[0].Moment.FirstZPiCoefficient.SetInt64(200)
		again, e := p.MassMomentsBands(ctx, []int{0}, geom.RadialGraphMomentOptions{})
		if e != nil || again.Height.CentroidZ.Cmp(want) != 0 {
			t.Fatal("owned outputs", e)
		}
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if out, e := p.MassMomentsBands(cancelled, []int{0}, geom.RadialGraphMomentOptions{}); e != context.Canceled || out.Height.CentroidZ != nil {
			t.Fatal("cancel", e)
		}
	}
}

func TestTripleRadialOriginalCentroid(t *testing.T) {
	rat := func(s string) *big.Rat {
		q, ok := new(big.Rat).SetString(s)
		if !ok {
			t.Fatal(s)
		}
		return q
	}
	want := rat("106610222140625/232841378390867968")
	coefficients := []*big.Rat{rat("430085395050048828125/7605903601369376408980219232256"), rat("7588958740234375/226673591177742970257408")}
	for _, reverse := range []bool{false, true} {
		p := tripleRadialVolumeProof(t, reverse)
		m, e := p.MassMomentsBands(context.Background(), []int{0, 2}, geom.RadialGraphMomentOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if m.Height.CentroidZ.Cmp(want) != 0 || len(m.Bands) != 2 {
			t.Fatal("independent polynomial-square centroid", m.Height.CentroidZ, want)
		}
		for i, c := range coefficients {
			if m.Bands[i].Moment.FirstZPiCoefficient.Cmp(c) != 0 {
				t.Fatal("independent per-component Z integral", i, m.Bands[i], c)
			}
		}
		if out, e := p.MassMomentsBands(context.Background(), []int{0, 2}, geom.RadialGraphMomentOptions{MaxCoefficientBits: 64}); e == nil || out.Height.CentroidZ != nil {
			t.Fatal("coefficient budget", e)
		}
		t.Log("original exact centroid Z", m.Height.CentroidZ, "operations", m.ExactOperations)
	}
}

func TestRadialCentroidZeroHeightSum(t *testing.T) {
	var patches [2]*geom.SurfacePatch
	knots := []float64{0, 0, 0, 1, 1, 1}
	q := [3]float64{.25, -.25, .25}
	for k := range 2 {
		var poles []geom.Vec3
		for i := range 3 {
			for j := range 3 {
				z := (.0625 - q[i] - q[j]) / 2
				if k == 1 {
					z = -z
				}
				poles = append(poles, geom.Vec3{X: float64(i) / 2, Y: float64(j) / 2, Z: z})
			}
		}
		s, e := geom.NewBSplineSurface(2, 2, 3, 3, poles, knots, knots, nil)
		if e != nil {
			t.Fatal(e)
		}
		patches[k], e = geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
		if e != nil {
			t.Fatal(e)
		}
	}
	p, e := geom.CertifySplineGraphRadialContours(context.Background(), patches[0], patches[1], [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, []*big.Rat{big.NewRat(1, 4)}, geom.GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := p.MassMomentsBands(context.Background(), []int{0}, geom.RadialGraphMomentOptions{})
	if e != nil || m.Height.CentroidZ.Sign() != 0 || m.Height.FirstZPiCoefficient.Sign() != 0 {
		t.Fatal("zero original height sum", m, e)
	}
}
