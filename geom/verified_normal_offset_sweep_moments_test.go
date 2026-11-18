package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestVerifiedNormalOffsetSweepMomentsExactRationalSlab(t *testing.T) {
	ctx := context.Background()
	for _, placed := range []bool{false, true} {
		r := [3][3]*big.Rat{
			{big.NewRat(1, 1), new(big.Rat), new(big.Rat)},
			{new(big.Rat), big.NewRat(1, 1), new(big.Rat)},
			{new(big.Rat), new(big.Rat), big.NewRat(1, 1)},
		}
		origin := [3]*big.Rat{new(big.Rat), new(big.Rat), new(big.Rat)}
		if placed {
			r[0][0], r[0][1], r[1][0], r[1][1] = big.NewRat(4, 5), big.NewRat(-3, 5), big.NewRat(3, 5), big.NewRat(4, 5)
			origin = [3]*big.Rat{big.NewRat(5, 4), big.NewRat(-7, 5), big.NewRat(11, 9)}
		}
		transform := func(local [3]*big.Rat) (world [3]*big.Rat) {
			for i := range r {
				world[i] = new(big.Rat).Set(origin[i])
				for j := range r[i] {
					world[i].Add(world[i], new(big.Rat).Mul(r[i][j], local[j]))
				}
			}
			return world
		}
		var poles [][4]*big.Rat
		for _, x := range []int64{0, 2} {
			for _, y := range []int64{0, 3} {
				v := transform([3]*big.Rat{big.NewRat(x, 1), big.NewRat(y, 1), new(big.Rat)})
				poles = append(poles, [4]*big.Rat{v[0], v[1], v[2], big.NewRat(1, 1)})
			}
		}
		p, err := NewExactHomogeneousBezierSurface(ctx, 1, 1, poles, SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		s, _, err := NewVerifiedNormalOffsetSweep(ctx, p, big.NewRat(-1, 3), big.NewRat(2, 7),
			NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}, NormalOffsetCoverageOptions{})
		if err != nil {
			t.Fatal(err)
		}
		tol := big.NewRat(1, 100000)
		o := NormalOffsetSweepMomentOptions{VolumeTolerance: tol, CentroidTolerance: tol,
			SecondMomentTolerance: tol, InertiaTolerance: tol, MaxCells: 1, Patch: SurfacePatchOptions{MaxCoefficientBits: 64}}
		got, err := s.MassMomentsRational(ctx, o)
		if err != nil || !got.Complete || got.VisitedCells != 1 || got.ActiveCells != 1 {
			t.Fatal("rational slab must have exact moments after one cell", placed, got, err)
		}
		eq := func(interval ExactParameterInterval, q *big.Rat) {
			t.Helper()
			if interval.Lower.Cmp(q) != 0 || interval.Upper.Cmp(q) != 0 {
				t.Fatal("translated and rotated slab moment differs from independent tensor", placed, interval, q)
			}
		}
		volume := big.NewRat(26, 7)
		eq(got.Volume, volume)
		center := transform([3]*big.Rat{big.NewRat(1, 1), big.NewRat(3, 2), big.NewRat(-1, 42)})
		localVariance := [3]*big.Rat{big.NewRat(1, 3), big.NewRat(3, 4), big.NewRat(169, 5292)}
		var second, central [3][3]*big.Rat
		for i := range center {
			eq(got.Centroid[i], center[i])
			eq(got.FirstMoment[i], new(big.Rat).Mul(volume, center[i]))
			for j := range center {
				central[i][j] = new(big.Rat)
				for k, v := range localVariance {
					q := new(big.Rat).Mul(r[i][k], r[j][k])
					central[i][j].Add(central[i][j], q.Mul(q, v))
				}
				central[i][j].Mul(central[i][j], volume)
				q := new(big.Rat).Mul(center[i], center[j])
				second[i][j] = new(big.Rat).Add(central[i][j], q.Mul(q, volume))
				eq(got.SecondMoment[i][j], second[i][j])
			}
		}
		for i := range second {
			for j := range second[i] {
				if i == j {
					k, l := (i+1)%3, (i+2)%3
					eq(got.InertiaOrigin[i][j], new(big.Rat).Add(second[k][k], second[l][l]))
					eq(got.InertiaCentroid[i][j], new(big.Rat).Add(central[k][k], central[l][l]))
				} else {
					eq(got.InertiaOrigin[i][j], new(big.Rat).Neg(second[i][j]))
					eq(got.InertiaCentroid[i][j], new(big.Rat).Neg(central[i][j]))
				}
			}
		}
		short := o
		short.Patch.MaxOperations = got.ExactOperations - 1
		limited, err := s.MassMomentsRational(ctx, short)
		if !errors.Is(err, ErrConvergence) || limited.Complete {
			t.Fatal("one-short exact operation limit published completion", limited, err)
		}
		if limited.Volume.Lower != nil {
			eq(limited.Volume, volume)
		}
		got.Centroid[0].Lower.SetInt64(700)
		got.InertiaOrigin[0][1].Lower.SetInt64(700)
		if got.Centroid[0].Upper.Cmp(center[0]) != 0 ||
			got.InertiaOrigin[1][0].Lower.Cmp(new(big.Rat).Neg(second[0][1])) != 0 {
			t.Fatal("published moment bounds share mutable rational values")
		}
		t.Logf("placed=%t exact volume %s with %d exact operations", placed, volume, got.ExactOperations)
	}
}

func TestVerifiedNormalOffsetSweepMomentsCurvedAndSigned(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for _, negative := range []bool{false, true} {
		name := "ordinary"
		if negative {
			name = "rotated negative Jacobian"
		}
		t.Run(name, func(t *testing.T) {
			xWidth := .25
			uv := ParameterRectangle{.375, .625, 0, 1}
			lower, upper := big.NewRat(-1, 20), big.NewRat(1, 10)
			view := NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}
			o := NormalOffsetSweepMomentOptions{
				VolumeTolerance: big.NewRat(1, 300), CentroidTolerance: big.NewRat(3, 40),
				SecondMomentTolerance: big.NewRat(1, 125), InertiaTolerance: big.NewRat(1, 125),
			}
			if negative {
				xWidth = .03125
				uv = ParameterRectangle{.484375, .515625, 0, 1}
				lower, upper = big.NewRat(7, 10), big.NewRat(4, 5)
				view = NormalOffsetSweepProjection{Axes: [3]int{1, 0, 2}, Reversed: [3]bool{true, true}}
				o.VolumeTolerance, o.SecondMomentTolerance, o.InertiaTolerance = big.NewRat(1, 10000), big.NewRat(1, 1300), big.NewRat(1, 1300)
			}
			p, err := normalOffsetParabolicCylinder(t, negative).Trim(uv)
			if err != nil {
				t.Fatal(err)
			}
			sweep, _, err := NewVerifiedNormalOffsetSweep(ctx, p, lower, upper, view, NormalOffsetCoverageOptions{})
			if err != nil || (sweep.Orientation() < 0) != negative {
				t.Fatal("expected independent original parabolic sweep", sweep, err)
			}
			lo, _ := lower.Float64()
			hi, _ := upper.Float64()
			want := referenceParabolicSweepMoments(xWidth, lo, hi, negative)
			got, err := sweep.MassMomentsRational(ctx, o)
			verifySweepMomentControl(t, got, err, o, want)
			if !negative {
				short := o
				short.MaxCells = 1
				coarse, err := sweep.MassMomentsRational(ctx, short)
				if !errors.Is(err, ErrConvergence) || coarse.Complete || coarse.ActiveCells != 1 ||
					coarse.Volume.Lower == nil || coarse.Volume.Lower.Sign() <= 0 ||
					coarse.Volume.Lower.Cmp(got.Volume.Lower) > 0 || coarse.Volume.Upper.Cmp(got.Volume.Upper) < 0 {
					t.Fatal("short cell limit lost the coarse positive material result", coarse, err)
				}
				short.Patch.MaxOperations = 1
				empty, err := sweep.MassMomentsRational(ctx, short)
				if !errors.Is(err, ErrConvergence) || empty.Complete || empty.Volume.Lower != nil {
					t.Fatal("exact work refusal published unresolved numerical data", empty, err)
				}
				cancelled, cancel := context.WithCancel(ctx)
				cancel()
				empty, err = sweep.MassMomentsRational(cancelled, o)
				if !errors.Is(err, context.Canceled) || empty.Complete || empty.Volume.Lower != nil {
					t.Fatal("cancellation published numerical data", empty, err)
				}
			}
		})
	}
}

type referenceSweepMoments struct {
	volume float64
	first  [3]float64
	second [3][3]float64
}

func referenceParabolicSweepMoments(width, lower, upper float64, rotated bool) (result referenceSweepMoments) {
	const nx, nt = 1024, 128
	dx, dt := width*2/nx, (upper-lower)/nt
	for i := 0; i < nx; i++ {
		x := -width + dx*(float64(i)+.5)
		g := 1 + 4*x*x
		length := math.Sqrt(g)
		for j := 0; j < nt; j++ {
			t := lower + dt*(float64(j)+.5)
			weight := math.Abs(length-2*t/g) * dx * dt
			xMoved, zMoved := x-2*x*t/length, x*x+t/length
			center, uniform := [3]float64{xMoved, .5, zMoved}, 1
			if rotated {
				center, uniform = [3]float64{-.5, xMoved, zMoved}, 0
			}
			result.volume += weight
			for k, m := range center {
				result.first[k] += m * weight
				for l, n := range center {
					product := m * n
					if k == uniform && l == uniform {
						product = 1.0 / 3
					}
					result.second[k][l] += product * weight
				}
			}
		}
	}
	return result
}

func verifySweepMomentControl(t *testing.T, got NormalOffsetSweepMassMoments, err error,
	o NormalOffsetSweepMomentOptions, want referenceSweepMoments) {
	t.Helper()
	if err != nil || !got.Complete || got.Volume.Lower == nil || got.Volume.Lower.Sign() <= 0 {
		t.Fatal("curved integration should qualify at the requested widths", got.Volume, got.ActiveCells, got.VisitedCells, got.ExactOperations, err)
	}
	check := func(q ExactParameterInterval, expected float64, tolerance *big.Rat) {
		t.Helper()
		lo, _ := q.Lower.Float64()
		hi, _ := q.Upper.Float64()
		if expected < lo || expected > hi ||
			(tolerance != nil && new(big.Rat).Sub(q.Upper, q.Lower).Cmp(tolerance) > 0) {
			t.Fatalf("independent original integral %g outside [%g,%g] at width %v", expected, lo, hi, tolerance)
		}
	}
	check(got.Volume, want.volume, o.VolumeTolerance)
	for i := range want.first {
		check(got.FirstMoment[i], want.first[i], nil)
		check(got.Centroid[i], want.first[i]/want.volume, o.CentroidTolerance)
		for j := range want.first {
			check(got.SecondMoment[i][j], want.second[i][j], o.SecondMomentTolerance)
			origin := -want.second[i][j]
			center := origin + want.first[i]*want.first[j]/want.volume
			if i == j {
				k, l := (i+1)%3, (i+2)%3
				origin = want.second[k][k] + want.second[l][l]
				center = origin - (want.first[k]*want.first[k]+want.first[l]*want.first[l])/want.volume
			}
			check(got.InertiaOrigin[i][j], origin, o.InertiaTolerance)
			check(got.InertiaCentroid[i][j], center, o.InertiaTolerance)
		}
	}
	t.Logf("independent volume %.13g, centroid [%.10g %.10g %.10g], %d visited cells, %d exact operations",
		want.volume, want.first[0]/want.volume, want.first[1]/want.volume, want.first[2]/want.volume,
		got.VisitedCells, got.ExactOperations)
}

func TestVerifiedNormalOffsetSweepMomentsSphericalSector(t *testing.T) {
	ctx := context.Background()
	a := big.NewRat(1, 32)
	a2 := new(big.Rat).Mul(a, a)
	lin := [3]*big.Rat{new(big.Rat).Neg(a), new(big.Rat), a}
	sq := [3]*big.Rat{a2, new(big.Rat).Neg(a2), a2}
	var poles [][4]*big.Rat
	for i := range lin {
		for j := range lin {
			r2 := new(big.Rat).Add(sq[i], sq[j])
			poles = append(poles, [4]*big.Rat{new(big.Rat).Add(lin[i], lin[i]), new(big.Rat).Add(lin[j], lin[j]),
				new(big.Rat).Sub(big.NewRat(1, 1), r2), new(big.Rat).Add(big.NewRat(1, 1), r2)})
		}
	}
	p, err := NewExactHomogeneousBezierSurface(ctx, 2, 2, poles, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sweep, _, err := NewVerifiedNormalOffsetSweep(ctx, p, big.NewRat(1, 4), big.NewRat(1, 2),
		NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}, NormalOffsetCoverageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	o := NormalOffsetSweepMomentOptions{
		VolumeTolerance: big.NewRat(1, 3000), CentroidTolerance: big.NewRat(1, 12),
		SecondMomentTolerance: big.NewRat(1, 1000), InertiaTolerance: big.NewRat(1, 1000),
	}
	m, err := sweep.MassMomentsRational(ctx, o)
	// The displacement coordinate is the radial length minus one.
	// Integrating R², R³ and R⁴ gives independent radial weights.
	// The remaining numerical integral checks the original spherical
	// surface using its exact stereographic area law.
	var want referenceSweepMoments
	const n = 256
	const step = (2.0 / 32) / n
	weight := func(degree int) float64 {
		return (math.Pow(1.5, float64(degree+1)) - math.Pow(1.25, float64(degree+1))) / float64(degree+1)
	}
	radial := [3]float64{weight(2), weight(3), weight(4)}
	for i := 0; i < n; i++ {
		x := -1.0/32 + step*(float64(i)+.5)
		for j := 0; j < n; j++ {
			y := -1.0/32 + step*(float64(j)+.5)
			q := x*x + y*y
			d := 1 + q
			point := [3]float64{2 * x / d, 2 * y / d, (1 - q) / d}
			area := 4 * step * step / (d * d)
			want.volume += area * radial[0]
			for i, p := range point {
				want.first[i] += area * radial[1] * p
				for j, s := range point {
					want.second[i][j] += area * radial[2] * p * s
				}
			}
		}
	}
	verifySweepMomentControl(t, m, err, o, want)
	exact := referenceSphereAxialSecond(a)
	if m.SecondMoment[2][2].Lower.Cmp(exact.Lower) > 0 || m.SecondMoment[2][2].Upper.Cmp(exact.Upper) < 0 {
		t.Fatal("axial second moment missed independently integrated alternating-series bounds", m.SecondMoment[2][2], exact)
	}
}

func referenceSphereAxialSecond(a *big.Rat) ExactParameterInterval {
	// The angular factor is 4*(1-r)²/(1+r)^4 for r=x²+y².
	// On this domain successive alternating terms of (1+r)^-4
	// decrease. Both truncations integrate original monomials exactly.
	var m [10]*big.Rat
	a2 := new(big.Rat).Mul(a, a)
	p := new(big.Rat).Set(a)
	for k := range m {
		m[k] = new(big.Rat).Mul(p, big.NewRat(2, int64(2*k+1)))
		p.Mul(p, a2)
	}
	angularPower := func(k int) *big.Rat {
		sum := new(big.Rat)
		for j := 0; j <= k; j++ {
			term := new(big.Rat).Mul(m[j], m[k-j])
			term.Mul(term, new(big.Rat).SetInt(new(big.Int).Binomial(int64(k), int64(j))))
			sum.Add(sum, term)
		}
		return sum
	}
	var result ExactParameterInterval
	sum := new(big.Rat)
	for k := 0; k < 8; k++ {
		term := angularPower(k)
		term.Sub(term, new(big.Rat).Mul(big.NewRat(2, 1), angularPower(k+1)))
		term.Add(term, angularPower(k+2))
		term.Mul(term, new(big.Rat).SetInt(new(big.Int).Binomial(int64(k+3), 3)))
		if k%2 == 0 {
			sum.Add(sum, term)
			result.Upper = new(big.Rat).Set(sum)
		} else {
			sum.Sub(sum, term)
			result.Lower = new(big.Rat).Set(sum)
		}
	}
	// 4 * integral of R^4 over the exact radial range [5/4,3/2].
	factor := big.NewRat(4651, 1280)
	result.Lower.Mul(result.Lower, factor)
	result.Upper.Mul(result.Upper, factor)
	return result
}
