package geom_test

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func nonradialRoofGap(t *testing.T) *geom.SplineGraphRadialContours {
	return nonradialRoofGapShift(t, 0)
}

func nonradialRoofGapShift(t *testing.T, shift float64) *geom.SplineGraphRadialContours {
	t.Helper()
	cx, cy, r := .375, .625, .125
	knots := []float64{0, 0, 0, 1, 1, 1}
	var patches [2]*geom.SurfacePatch
	qx := [3]float64{cx * cx, cx*cx - cx, (1 - cx) * (1 - cx)}
	qy := [3]float64{cy * cy, cy*cy - cy, (1 - cy) * (1 - cy)}
	for k := range 2 {
		var poles []geom.Vec3
		for i := range 3 {
			for j := range 3 {
				x, y := float64(i)/2, float64(j)/2
				z := x*y/64 + shift
				if k == 1 {
					z += r*r - qx[i] - qy[j]
				}
				poles = append(poles, geom.Vec3{X: x, Y: y, Z: z})
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
	proof, e := geom.CertifySplineGraphRadialContours(context.Background(), patches[0], patches[1], [2]*big.Rat{big.NewRat(3, 8), big.NewRat(5, 8)}, []*big.Rat{big.NewRat(1, 8)}, geom.GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return proof
}

func TestRadialPlanarMomentsNonradialRoofs(t *testing.T) {
	ctx := context.Background()
	p := nonradialRoofGap(t)
	m, e := p.PlanarMomentsBands(ctx, []int{0}, geom.RadialGraphMomentOptions{})
	if e != nil {
		t.Fatal(e)
	}
	// Independently, H=r^2-q gives V/pi=r^4/2 and each central
	// horizontal second moment /pi = r^6/12.
	v := big.NewRat(1, 8192)
	central := big.NewRat(1, 3145728)
	center := [2]*big.Rat{big.NewRat(3, 8), big.NewRat(5, 8)}
	if len(m.Bands) != 1 || m.Bands[0].HeightSign != -1 || m.Total.VolumePiCoefficient.Cmp(v) != 0 || m.Total.ProjectedAreaPiCoefficient.Cmp(big.NewRat(1, 64)) != 0 {
		t.Fatal(m)
	}
	for i, c := range center {
		if m.Total.CentroidXY[i].Cmp(c) != 0 || m.Total.FirstMomentPiCoefficients[i].Cmp(new(big.Rat).Mul(c, v)) != 0 {
			t.Fatal("first moment", i)
		}
	}
	for i, axes := range [][2]int{{0, 0}, {0, 1}, {1, 1}} {
		want := new(big.Rat).Mul(center[axes[0]], center[axes[1]])
		want.Mul(want, v)
		if axes[0] == axes[1] {
			want.Add(want, central)
		}
		if m.Total.SecondMomentPiCoefficients[i].Cmp(want) != 0 {
			t.Fatal("second moment", i, m.Total.SecondMomentPiCoefficients[i], want)
		}
	}
	work := m.ExactOperations
	if _, e = p.PlanarMomentsBands(ctx, []int{0}, geom.RadialGraphMomentOptions{MaxOperations: work}); e != nil {
		t.Fatal("exact budget", e)
	}
	if out, e := p.PlanarMomentsBands(ctx, []int{0}, geom.RadialGraphMomentOptions{MaxOperations: work - 1}); e == nil || out.Total.VolumePiCoefficient != nil || len(out.Bands) > 0 {
		t.Fatal("atomic budget", out, e)
	}
	m.Total.CentroidXY[0].SetInt64(20)
	m.Bands[0].Moment.VolumePiCoefficient.SetInt64(99)
	again, e := p.PlanarMomentsBands(ctx, []int{0}, geom.RadialGraphMomentOptions{})
	if e != nil || again.Total.CentroidXY[0].Cmp(center[0]) != 0 || again.Total.VolumePiCoefficient.Cmp(v) != 0 {
		t.Fatal("mutated proof", e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if out, e := p.PlanarMomentsBands(cancelled, []int{0}, geom.RadialGraphMomentOptions{}); e != context.Canceled || len(out.Bands) != 0 {
		t.Fatal("cancel", e)
	}
	for _, bands := range [][]int{nil, {1}, {0, 0}} {
		if out, e := p.PlanarMomentsBands(ctx, bands, geom.RadialGraphMomentOptions{}); e == nil || len(out.Bands) != 0 {
			t.Fatal("invalid bands", bands, e)
		}
	}
}

func TestTripleRadialPlanarMoments(t *testing.T) {
	rat := func(s string) *big.Rat {
		q, ok := new(big.Rat).SetString(s)
		if !ok {
			t.Fatal(s)
		}
		return q
	}
	q := new(big.Rat).Add(rat("4983740234375/3541774862152233910272"), rat("212939453125/27021597764222976"))
	for _, reverse := range []bool{false, true} {
		p := tripleRadialVolumeProof(t, reverse)
		m, e := p.PlanarMomentsBands(context.Background(), []int{0, 2}, geom.RadialGraphMomentOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if len(m.Bands) != 2 || m.Total.ProjectedAreaPiCoefficient.Cmp(big.NewRat(425, 4096)) != 0 || m.Total.VolumePiCoefficient.Cmp(big.NewRat(63333935546875, 864691128455135232)) != 0 {
			t.Fatal(m)
		}
		centered := new(big.Rat).Sub(m.Total.SecondMomentPiCoefficients[0], new(big.Rat).Quo(m.Total.VolumePiCoefficient, big.NewRat(4, 1)))
		centered.Mul(centered, big.NewRat(2, 1))
		if centered.Cmp(q) != 0 {
			t.Fatal("independent q-weighted integral", centered, q)
		}
		if m.Total.SecondMomentPiCoefficients[0].Cmp(m.Total.SecondMomentPiCoefficients[2]) != 0 || m.Total.SecondMomentPiCoefficients[1].Cmp(new(big.Rat).Quo(m.Total.VolumePiCoefficient, big.NewRat(4, 1))) != 0 {
			t.Fatal("radial tensor symmetry")
		}
		for _, o := range []geom.RadialGraphMomentOptions{{MaxCoefficientBits: 63}, {MaxCoefficientBits: 64}, {MaxOperations: 1}} {
			if out, e := p.PlanarMomentsBands(context.Background(), []int{0, 2}, o); e == nil || out.Total.VolumePiCoefficient != nil || len(out.Bands) != 0 {
				t.Fatal("atomic arithmetic limit", out, e)
			}
		}
		t.Log("exact planar moments work", m.ExactOperations)
	}
}
