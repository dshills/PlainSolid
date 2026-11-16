package geom

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func TestProjectiveSurfaceGraphOverlapDiscoversOriginalPartialMap(t *testing.T) {
	ctx := context.Background()
	for _, rotated := range []bool{false, true} {
		axes := [2]int{0, 1}
		if rotated {
			axes = [2]int{2, 0}
		}
		a := arcTestPatch(t, func(u, v float64) Vec3 {
			if rotated {
				return Vec3{v, u * v, u}
			}
			return Vec3{u, v, u * v}
		})
		b := projectiveOverlapTestChart(t, rotated, false)
		r, err := CertifyProjectiveSurfaceGraphOverlap(ctx, a, b, axes, SurfacePatchOptions{})
		if err != nil || !r.Coincident || r.NoArea || !r.SwapB || !r.ReverseBU || r.ReverseBV ||
			r.Orientation != 1 || r.ProjectionOrientation != [2]int{1, 1} || r.ComparedCells != 4 {
			t.Fatal("did not discover the complete projective overlap under the original chart map", rotated, r, err)
		}
		controls := []struct {
			interval ExactParameterInterval
			lo, hi   *big.Rat
		}{
			{r.Projection[0], new(big.Rat), big.NewRat(1, 1)},
			{r.Projection[1], big.NewRat(1, 2), big.NewRat(1, 1)},
			{r.Areas[0].U, new(big.Rat), big.NewRat(1, 1)},
			{r.Areas[0].V, big.NewRat(1, 2), big.NewRat(1, 1)},
			{r.Areas[1].U, big.NewRat(-2, 1), big.NewRat(2, 7)},
			{r.Areas[1].V, big.NewRat(64, 13), big.NewRat(128, 17)},
		}
		for i, c := range controls {
			if c.interval.Lower.Cmp(c.lo) != 0 || c.interval.Upper.Cmp(c.hi) != 0 {
				t.Fatal("overlap clipped an original or model interval incorrectly", i, c)
			}
		}
		if r.BStationRatios[0].Cmp(big.NewRat(13, 17)) != 0 || r.BStationRatios[1].Cmp(big.NewRat(7, 9)) != 0 {
			t.Fatal("wrong original local station ratios", r.BStationRatios)
		}
		perturbed, err := CertifyProjectiveSurfaceGraphOverlap(ctx, a, projectiveOverlapTestChart(t, rotated, true), axes, SurfacePatchOptions{})
		if !errors.Is(err, ErrConvergence) || perturbed.Coincident || perturbed.NoArea || perturbed.ComparedCells != 1 ||
			perturbed.Areas[1].V.Upper.Cmp(r.Areas[1].V.Upper) != 0 {
			t.Fatal("a common projection must not certify a perturbed original lift", perturbed, err)
		}
	}
}

func TestProjectiveSurfaceGraphOverlapRejectsAProjectionThatOnlyMatchesSamples(t *testing.T) {
	ctx := context.Background()
	a := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{u, v, 0} })
	var controls [][4]*big.Rat
	for u := int64(0); u <= 1; u++ {
		for j, bump := range [3]*big.Rat{new(big.Rat), big.NewRat(1, 2), new(big.Rat)} {
			// x varies linearly at v=1/2, and looks constant during
			// the endpoint probes on u=1/2. It is not separable.
			x := new(big.Rat).Add(big.NewRat(u, 1), new(big.Rat).Mul(bump, big.NewRat(2*u-1, 200)))
			controls = append(controls, [4]*big.Rat{x, big.NewRat(int64(j), 2), new(big.Rat), big.NewRat(1, 1)})
		}
	}
	b, err := NewExactHomogeneousBezierSurface(ctx, 1, 2, controls, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := CertifyProjectiveSurfaceGraphOverlap(ctx, a, b, [2]int{0, 1}, SurfacePatchOptions{})
	if !errors.Is(err, ErrDomain) || r.Coincident || r.NoArea {
		t.Fatal("sampling alone must not authenticate a separable projection", r, err)
	}
	contact := arcTestPatch(t, func(u, v float64) Vec3 { return Vec3{1 + u, v, 0} })
	r, err = CertifyProjectiveSurfaceGraphOverlap(ctx, a, contact, [2]int{0, 1}, SurfacePatchOptions{})
	if err != nil || r.Coincident || !r.NoArea {
		t.Fatal("edge contact has no positive projected area", r, err)
	}
	r, err = CertifyProjectiveSurfaceGraphOverlap(ctx, a, a, [2]int{0, 1}, SurfacePatchOptions{MaxOperations: 16})
	if !errors.Is(err, ErrConvergence) || r.Coincident || r.NoArea {
		t.Fatal("short proof work must remain unresolved", r, err)
	}
}

func projectiveOverlapTestChart(t *testing.T, rotated, perturb bool) *SurfacePatch {
	t.Helper()
	// x=5/4−(3/2)φ_{2/3}(v), y=1/2+(3/4)φ_{3/2}(u), z=xy.
	// Evaluate the homogeneous bilinear numerators independently at
	// the two original internal knots (physical U=0 and V=7).
	p := &SurfacePatch{degreeU: 1, degreeV: 1, countU: 3, countV: 3,
		knotsU: []float64{-2, -2, 0, 2, 2}, knotsV: []float64{4, 4, 7, 8, 8}}
	one := big.NewRat(1, 1)
	shift := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	for _, u := range [3]*big.Rat{new(big.Rat), big.NewRat(1, 2), one} {
		for _, v := range [3]*big.Rat{new(big.Rat), big.NewRat(3, 4), one} {
			xw := new(big.Rat).Sub(one, new(big.Rat).Quo(v, big.NewRat(3, 1)))
			yw := new(big.Rat).Add(one, new(big.Rat).Quo(u, big.NewRat(2, 1)))
			x := new(big.Rat).Sub(new(big.Rat).Mul(big.NewRat(5, 4), xw), v)
			y := new(big.Rat).Add(new(big.Rat).Quo(yw, big.NewRat(2, 1)), new(big.Rat).Mul(big.NewRat(9, 8), u))
			w := new(big.Rat).Mul(xw, yw)
			z := new(big.Rat).Mul(x, y)
			if perturb {
				z.Add(z, new(big.Rat).Mul(shift, w))
			}
			p2 := exactHomogeneous{new(big.Rat).Mul(x, yw), new(big.Rat).Mul(y, xw), z, w}
			if rotated {
				p2[0], p2[1], p2[2] = p2[1], p2[2], p2[0]
			}
			p.net = append(p.net, p2)
		}
	}
	p.options, _ = patchOptions(SurfacePatchOptions{})
	if err := p.computeBounds(&polynomialBudget{maxOperations: p.options.MaxOperations, maxBits: p.options.MaxCoefficientBits}); err != nil {
		t.Fatal(err)
	}
	return p
}
