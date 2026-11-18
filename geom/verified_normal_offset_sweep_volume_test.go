package geom

import (
	"context"
	"errors"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"
)

func TestVerifiedNormalOffsetSweepVolumeExactSlab(t *testing.T) {
	ctx := context.Background()
	var poles [][4]*big.Rat
	for _, x := range []int64{0, 2} {
		for _, y := range []int64{0, 3} {
			poles = append(poles, [4]*big.Rat{big.NewRat(x, 1), big.NewRat(y, 1), new(big.Rat), big.NewRat(1, 1)})
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
	v, err := s.VolumeRational(ctx, NormalOffsetSweepVolumeOptions{AbsoluteError: big.NewRat(1, 10000), MaxCells: 1})
	if err != nil || !v.Complete || v.VisitedCells != 1 || v.ActiveCells != 1 ||
		v.Bounds.Lower.Cmp(big.NewRat(26, 7)) != 0 || v.Bounds.Upper.Cmp(big.NewRat(26, 7)) != 0 {
		t.Fatal("exact rational slab volume was not recovered", v, err)
	}
}

func TestVerifiedNormalOffsetSweepVolumeParabolicStrip(t *testing.T) {
	ctx := context.Background()
	for _, c := range []struct {
		name        string
		umin, umax  float64
		lo, hi, tol *big.Rat
		orientation int
	}{
		{"ordinary", .375, .625, big.NewRat(-1, 20), big.NewRat(1, 10), big.NewRat(1, 350), 1},
		{"negative Jacobian", .484375, .515625, big.NewRat(7, 10), big.NewRat(4, 5), big.NewRat(1, 10000), -1},
	} {
		t.Run(c.name, func(t *testing.T) {
			p, err := normalOffsetParabolicCylinder(t, false).Trim(ParameterRectangle{c.umin, c.umax, 0, 1})
			if err != nil {
				t.Fatal(err)
			}
			s, _, err := NewVerifiedNormalOffsetSweep(ctx, p, c.lo, c.hi,
				NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}, Reversed: [3]bool{c.orientation < 0}}, NormalOffsetCoverageOptions{})
			if err != nil || s.Orientation() != c.orientation {
				t.Fatal("could not verify original strip sweep orientation", s, err)
			}
			a := 2*c.umax - 1
			lo, _ := c.lo.Float64()
			hi, _ := c.hi.Float64()
			// Integral of area sqrt(1+4*x²), and of its signed
			// curvature term 2/(1+4*x²), across the exact strip.
			area := a*math.Sqrt(1+4*a*a) + math.Asinh(2*a)/2
			want := math.Abs((hi-lo)*area - (hi*hi-lo*lo)*math.Atan(2*a))
			v, err := s.VolumeRational(ctx, NormalOffsetSweepVolumeOptions{AbsoluteError: c.tol})
			checkSweepVolumeControl(t, v, err, c.tol, want)
			if c.orientation > 0 {
				limited, err := s.VolumeRational(ctx, NormalOffsetSweepVolumeOptions{AbsoluteError: c.tol, MaxCells: 1})
				if !errors.Is(err, ErrConvergence) || limited.Complete || limited.ActiveCells != 1 ||
					limited.Bounds.Lower.Cmp(v.Bounds.Lower) > 0 || limited.Bounds.Upper.Cmp(v.Bounds.Upper) < 0 {
					t.Fatal("short budget lost the coarse certified bound", limited, err)
				}
			}
		})
	}
}

func TestVerifiedNormalOffsetSweepVolumeSphericalSector(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	a := big.NewRat(1, 16)
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
	s, _, err := NewVerifiedNormalOffsetSweep(ctx, p, big.NewRat(1, 4), big.NewRat(1, 2),
		NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}, NormalOffsetCoverageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// This exact stereographic chart lies on the unit sphere. Its
	// radial normal gives a quadratic (1+t)² volume factor, so omitting
	// the second displacement moment cannot satisfy this control.
	// Independently bound the stereographic solid angle with alternating
	// polynomial sums for 4/(1+x²+y²)², integrating each monomial exactly.
	var moment [6]*big.Rat
	power := new(big.Rat).Set(a)
	for i := range moment {
		moment[i] = new(big.Rat).Mul(power, big.NewRat(2, int64(2*i+1)))
		power.Mul(power, a2)
	}
	var lower, upper *big.Rat
	area := new(big.Rat)
	for k := int64(0); k <= 5; k++ {
		term := new(big.Rat)
		for i := int64(0); i <= k; i++ {
			q := new(big.Rat).Mul(moment[i], moment[k-i])
			term.Add(term, q.Mul(q, new(big.Rat).SetInt(new(big.Int).Binomial(k, i))))
		}
		term.Mul(term, big.NewRat(4*(k+1), 1))
		if k%2 == 0 {
			area.Add(area, term)
			upper = new(big.Rat).Set(area)
		} else {
			area.Sub(area, term)
			lower = new(big.Rat).Set(area)
		}
	}
	for _, q := range [2]*big.Rat{lower, upper} {
		q.Mul(q, big.NewRat(91, 192))
	}
	want, _ := lower.Float64()
	tol := big.NewRat(1, 1800)
	v, err := s.VolumeRational(ctx, NormalOffsetSweepVolumeOptions{AbsoluteError: tol})
	checkSweepVolumeControl(t, v, err, tol, want)
	if v.Bounds.Lower.Cmp(lower) > 0 || v.Bounds.Upper.Cmp(upper) < 0 {
		t.Fatal("missed the independent exact sphere-sector enclosure", v, lower, upper)
	}
}

func checkSweepVolumeControl(t *testing.T, v NormalOffsetSweepVolume, err error, tol *big.Rat, want float64) {
	t.Helper()
	if v.Bounds.Lower == nil || v.Bounds.Upper == nil {
		t.Fatal("missing volume enclosure", v, err)
	}
	l, _ := v.Bounds.Lower.Float64()
	h, _ := v.Bounds.Upper.Float64()
	if err != nil || !v.Complete || l <= 0 || l > want || h < want || new(big.Rat).Sub(v.Bounds.Upper, v.Bounds.Lower).Cmp(tol) > 0 {
		t.Fatalf("volume %g outside completed enclosure [%g,%g]; cells %d, work %d: %v", want, l, h, v.VisitedCells, v.ExactOperations, err)
	}
	t.Logf("volume %g enclosed by [%g,%g] after %d cells", want, l, h, v.VisitedCells)
}
