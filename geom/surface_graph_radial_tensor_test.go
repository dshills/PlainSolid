package geom_test

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func tiltedRadialGap(t *testing.T, tilt [3]float64) *geom.SplineGraphRadialContours {
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
				z := tilt[0]*x + tilt[1]*y + tilt[2]
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
	p, e := geom.CertifySplineGraphRadialContours(context.Background(), patches[0], patches[1], [2]*big.Rat{big.NewRat(3, 8), big.NewRat(5, 8)}, []*big.Rat{big.NewRat(1, 8)}, geom.GraphPointContactOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return p
}

func checkRadialTensor(t *testing.T, got geom.RadialGraphTensor, want [3][3]*big.Rat) {
	t.Helper()
	trace := new(big.Rat)
	for i := range 3 {
		trace.Add(trace, want[i][i])
	}
	for i := range 3 {
		for j := range 3 {
			if got.SecondMomentPiCoefficients[i][j].Cmp(want[i][j]) != 0 {
				t.Fatal("raw second moment", i, j, got.SecondMomentPiCoefficients[i][j], want[i][j])
			}
			inertia := new(big.Rat).Neg(want[i][j])
			if i == j {
				inertia.Add(inertia, trace)
			}
			if got.OriginInertiaPiCoefficients[i][j].Cmp(inertia) != 0 {
				t.Fatal("world-origin inertia", i, j, got.OriginInertiaPiCoefficients[i][j], inertia)
			}
		}
	}
}

func TestRadialTensorAffineShearAndTranslation(t *testing.T) {
	ctx := context.Background()
	cx, cy := big.NewRat(3, 8), big.NewRat(5, 8)
	v := big.NewRat(1, 8192)
	z := big.NewRat(1, 1572864)
	c := big.NewRat(1, 3145728)
	var q [3][3]*big.Rat
	q[0][0] = new(big.Rat).Add(new(big.Rat).Mul(new(big.Rat).Mul(cx, cx), v), c)
	q[1][1] = new(big.Rat).Add(new(big.Rat).Mul(new(big.Rat).Mul(cy, cy), v), c)
	q[0][1] = new(big.Rat).Mul(new(big.Rat).Mul(cx, cy), v)
	q[1][0] = new(big.Rat).Set(q[0][1])
	q[0][2] = new(big.Rat).Mul(cx, z)
	q[2][0] = new(big.Rat).Set(q[0][2])
	q[1][2] = new(big.Rat).Mul(cy, z)
	q[2][1] = new(big.Rat).Set(q[1][2])
	q[2][2] = big.NewRat(1, 201326592)
	first := [3]*big.Rat{new(big.Rat).Mul(cx, v), new(big.Rat).Mul(cy, v), z}
	for _, tilt := range [][3]float64{{0, 0, 0}, {.25, -.125, -2}} {
		p := tiltedRadialGap(t, tilt)
		out, e := p.MassTensorBands(ctx, []int{0}, geom.RadialGraphMomentOptions{})
		if e != nil {
			t.Fatal(e)
		}
		// Independent push-forward of the analytic parabolic cap under the
		// determinant-one world shear z'=z+alpha*x+beta*y+delta.
		var l [3][3]*big.Rat
		for i := range 3 {
			for j := range 3 {
				l[i][j] = new(big.Rat)
				if i == j {
					l[i][j].SetInt64(1)
				}
			}
		}
		l[2][0].SetFloat64(tilt[0])
		l[2][1].SetFloat64(tilt[1])
		shift := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat).SetFloat64(tilt[2])}
		var lf [3]*big.Rat
		for i := range 3 {
			lf[i] = new(big.Rat)
			for k := range 3 {
				lf[i].Add(lf[i], new(big.Rat).Mul(l[i][k], first[k]))
			}
		}
		var want [3][3]*big.Rat
		for i := range 3 {
			for j := range 3 {
				want[i][j] = new(big.Rat)
				for k := range 3 {
					for n := range 3 {
						want[i][j].Add(want[i][j], new(big.Rat).Mul(new(big.Rat).Mul(l[i][k], q[k][n]), l[j][n]))
					}
				}
				want[i][j].Add(want[i][j], new(big.Rat).Mul(lf[i], shift[j]))
				want[i][j].Add(want[i][j], new(big.Rat).Mul(shift[i], lf[j]))
				want[i][j].Add(want[i][j], new(big.Rat).Mul(new(big.Rat).Mul(shift[i], shift[j]), v))
			}
		}
		checkRadialTensor(t, out.Total, want)
		if len(out.Bands) != 1 {
			t.Fatal(out)
		}
		checkRadialTensor(t, out.Bands[0].Tensor, want)
		work := out.ExactOperations
		if _, e = p.MassTensorBands(ctx, []int{0}, geom.RadialGraphMomentOptions{MaxOperations: work}); e != nil {
			t.Fatal("exact quota", e)
		}
		if partial, e := p.MassTensorBands(ctx, []int{0}, geom.RadialGraphMomentOptions{MaxOperations: work - 1}); e == nil || partial.Total.SecondMomentPiCoefficients[0][0] != nil || len(partial.Bands) != 0 {
			t.Fatal("atomic aggregate quota", e)
		}
		out.Total.SecondMomentPiCoefficients[0][2].SetInt64(99)
		if out.Total.SecondMomentPiCoefficients[2][0].Cmp(want[2][0]) != 0 {
			t.Fatal("symmetric entries alias")
		}
		again, e := p.MassTensorBands(ctx, []int{0}, geom.RadialGraphMomentOptions{})
		if e != nil {
			t.Fatal(e)
		}
		checkRadialTensor(t, again.Total, want)
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		if partial, e := p.MassTensorBands(cancelled, []int{0}, geom.RadialGraphMomentOptions{}); e != context.Canceled || len(partial.Bands) != 0 {
			t.Fatal("cancel", e)
		}
	}
}

func TestRadialTensorBilinearRoof(t *testing.T) {
	p := nonradialRoofGap(t)
	m, e := p.MassTensorBands(context.Background(), []int{0}, geom.RadialGraphMomentOptions{})
	if e != nil {
		t.Fatal(e)
	}
	cx, cy := big.NewRat(3, 8), big.NewRat(5, 8)
	v := big.NewRat(1, 8192)
	c := big.NewRat(1, 3145728)
	r6, r8 := big.NewRat(1, 262144), big.NewRat(1, 16777216)
	cx2, cy2 := new(big.Rat).Mul(cx, cx), new(big.Rat).Mul(cy, cy)
	var q [3][3]*big.Rat
	q[0][0] = new(big.Rat).Add(new(big.Rat).Mul(cx2, v), c)
	q[1][1] = new(big.Rat).Add(new(big.Rat).Mul(cy2, v), c)
	q[0][1] = new(big.Rat).Mul(new(big.Rat).Mul(cx, cy), v)
	q[1][0] = new(big.Rat).Set(q[0][1])
	q[0][2] = new(big.Rat).Add(new(big.Rat).Quo(new(big.Rat).Mul(cy, q[0][0]), big.NewRat(64, 1)), new(big.Rat).Quo(new(big.Rat).Mul(cx, r6), big.NewRat(6, 1)))
	q[2][0] = new(big.Rat).Set(q[0][2])
	q[1][2] = new(big.Rat).Add(new(big.Rat).Quo(new(big.Rat).Mul(cx, q[1][1]), big.NewRat(64, 1)), new(big.Rat).Quo(new(big.Rat).Mul(cy, r6), big.NewRat(6, 1)))
	q[2][1] = new(big.Rat).Set(q[1][2])
	// Integral of (xy/64)^2 H + (xy/64)H^2 + H^3/3.
	q[2][2] = new(big.Rat).Mul(new(big.Rat).Mul(cx2, cy2), v)
	q[2][2].Add(q[2][2], new(big.Rat).Mul(new(big.Rat).Add(cx2, cy2), c))
	q[2][2].Add(q[2][2], new(big.Rat).Quo(r8, big.NewRat(96, 1)))
	q[2][2].Quo(q[2][2], big.NewRat(4096, 1))
	q[2][2].Add(q[2][2], new(big.Rat).Quo(new(big.Rat).Mul(new(big.Rat).Mul(cx, cy), r6), big.NewRat(192, 1)))
	q[2][2].Add(q[2][2], new(big.Rat).Quo(r8, big.NewRat(12, 1)))
	checkRadialTensor(t, m.Total, q)
}

func TestTripleRadialTensorPolynomialCube(t *testing.T) {
	rat := func(s string) *big.Rat {
		q, ok := new(big.Rat).SetString(s)
		if !ok {
			t.Fatal(s)
		}
		return q
	}
	wantZZ := rat("8952130846239958703517913818359375/401413413573551216547642928907670507107647488")
	bands := []*big.Rat{rat("2739486239958703517913818359375/401413413573551216547642928907670507107647488"), rat("65115392208099365234375/2920666982925840541048404185186304")}
	for _, reverse := range []bool{false, true} {
		p := tripleRadialVolumeProof(t, reverse)
		m, e := p.MassTensorBands(context.Background(), []int{0, 2}, geom.RadialGraphMomentOptions{})
		if e != nil {
			t.Fatal(e)
		}
		if m.Total.SecondMomentPiCoefficients[2][2].Cmp(wantZZ) != 0 || len(m.Bands) != 2 {
			t.Fatal("independent H^3/3 integral", m.Total.SecondMomentPiCoefficients[2][2], wantZZ)
		}
		for i, b := range bands {
			if m.Bands[i].Tensor.SecondMomentPiCoefficients[2][2].Cmp(b) != 0 {
				t.Fatal("component cube", i)
			}
		}
		for axis := range 2 {
			if m.Total.SecondMomentPiCoefficients[axis][2].Cmp(new(big.Rat).Quo(m.Mass.Height.FirstZPiCoefficient, big.NewRat(2, 1))) != 0 {
				t.Fatal("radial XZ/YZ symmetry")
			}
		}
		if m.Total.OriginInertiaPiCoefficients[0][0].Cmp(m.Total.OriginInertiaPiCoefficients[1][1]) != 0 {
			t.Fatal("world inertia radial symmetry")
		}
		if out, e := p.MassTensorBands(context.Background(), []int{0, 2}, geom.RadialGraphMomentOptions{MaxCoefficientBits: 64}); e == nil || len(out.Bands) != 0 {
			t.Fatal("bit limit", e)
		}
		t.Log("triple exact tensor operations", m.ExactOperations)
	}
}

func TestRadialTensorMaximumDegree(t *testing.T) {
	ctx := context.Background()
	radii := []*big.Rat{big.NewRat(1, 16), big.NewRat(1, 8), big.NewRat(3, 16), big.NewRat(1, 4)}
	// Four exact radial factors have degree8. Adding u^8*v^8 to BOTH
	// original heights forces degree16 per axis in the squared height and
	// degree20 in the q-weighted ZZ integrand, exercising the explicit cap.
	power := map[[2]int]*big.Rat{{0, 0}: big.NewRat(1, 1)}
	for _, r := range radii {
		f := map[[2]int]*big.Rat{{0, 0}: new(big.Rat).Sub(big.NewRat(1, 2), new(big.Rat).Mul(r, r)), {1, 0}: big.NewRat(-1, 1), {0, 1}: big.NewRat(-1, 1), {2, 0}: big.NewRat(1, 1), {0, 2}: big.NewRat(1, 1)}
		next := map[[2]int]*big.Rat{}
		for a, x := range power {
			for c, y := range f {
				k := [2]int{a[0] + c[0], a[1] + c[1]}
				if next[k] == nil {
					next[k] = new(big.Rat)
				}
				next[k].Add(next[k], new(big.Rat).Mul(x, y))
			}
		}
		power = next
	}
	build := func(common bool) *geom.SplineGraphRadialContours {
		var patches [2]*geom.SurfacePatch
		for owner := range 2 {
			var net [][4]*big.Rat
			for i := 0; i <= 8; i++ {
				for j := 0; j <= 8; j++ {
					z := new(big.Rat)
					if owner == 1 {
						for key, c := range power {
							if key[0] > i || key[1] > j {
								continue
							}
							x := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(key[0])), new(big.Int).Binomial(8, int64(key[0])))
							y := new(big.Rat).SetFrac(new(big.Int).Binomial(int64(j), int64(key[1])), new(big.Int).Binomial(8, int64(key[1])))
							z.Add(z, new(big.Rat).Mul(new(big.Rat).Mul(c, x), y))
						}
					}
					if common && i == 8 && j == 8 {
						z.Add(z, big.NewRat(1, 1))
					}
					net = append(net, [4]*big.Rat{big.NewRat(int64(i), 8), big.NewRat(int64(j), 8), z, big.NewRat(1, 1)})
				}
			}
			var e error
			patches[owner], e = geom.NewExactHomogeneousBezierSurface(ctx, 8, 8, net, geom.SurfacePatchOptions{})
			if e != nil {
				t.Fatal(e)
			}
		}
		p, e := geom.CertifySplineGraphRadialContours(ctx, patches[0], patches[1], [2]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2)}, radii, geom.GraphPointContactOptions{})
		if e != nil {
			t.Fatal(e)
		}
		return p
	}
	base, e := build(false).MassTensorBands(ctx, []int{0, 2}, geom.RadialGraphMomentOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p := build(true)
	out, e := p.MassTensorBands(ctx, []int{0, 2}, geom.RadialGraphMomentOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if out.Total.SecondMomentPiCoefficients[2][2].Cmp(base.Total.SecondMomentPiCoefficients[2][2]) <= 0 || out.Mass.Height.CentroidZ.Cmp(base.Mass.Height.CentroidZ) <= 0 {
		t.Fatal("nonnegative common original height must increase Z moments")
	}
	if out.Total.SecondMomentPiCoefficients[0][2].Cmp(out.Total.SecondMomentPiCoefficients[1][2]) != 0 {
		t.Fatal("independent axis-exchange symmetry")
	}
	if out.ExactOperations > 100000 {
		t.Fatal("unexpected unbounded work", out.ExactOperations)
	}
	t.Log("maximum supported-degree tensor operations", out.ExactOperations)
}
