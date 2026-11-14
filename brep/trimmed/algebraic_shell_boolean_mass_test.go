package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestAlgebraicShellMassBoundsSignedTranslation(t *testing.T) {
	ctx := context.Background()
	// A translated warped slab has unit volume. Its bottom is at
	// z=1+xy/8 and its height is one above each (x,y) in [-2,-1]×[2,3].
	a, _ := makeOpenSplineBox(t, [3]float64{-2, 2, 1}, [3]float64{-1, 3, 2}, .125)
	b, _ := makeOpenSplineBox(t, [3]float64{2, -4, -3}, [3]float64{3, -2, -2}, 0)
	section, e := SectionSplineShellsAlgebraic(ctx, a, b, AlgebraicShellSectionOptions{})
	if e != nil {
		t.Fatal(e)
	}
	u, e := section.CombineAlgebraic(ctx, parametric.Union, AlgebraicShellBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := u.BoundMassProperties(ctx, AlgebraicShellMeasureOptions{})
	if e != nil {
		t.Fatal(e)
	}
	contains := func(b geom.ExactParameterInterval, q *big.Rat) bool {
		return b.Lower != nil && b.Upper != nil && b.Lower.Cmp(q) <= 0 && b.Upper.Cmp(q) >= 0
	}
	if m.Volume.Lower.Sign() <= 0 || !contains(m.Volume, big.NewRat(3, 1)) {
		t.Fatal("independent disjoint volume", m.Volume)
	}
	// First moments of the volume-one slab and disjoint volume-two box.
	// Their x moments have opposite signs; the total y and z moments
	// are negative even though the integration origin is on the slab.
	for axis, q := range []*big.Rat{big.NewRat(7, 2), big.NewRat(-7, 2), big.NewRat(-127, 32)} {
		if !contains(m.FirstMoment[axis], q) || !contains(m.Centroid[axis], new(big.Rat).Quo(q, big.NewRat(3, 1))) {
			t.Fatal("independent world first moment and centroid", axis, m.FirstMoment[axis], m.Centroid[axis], q)
		}
	}
	// Product integrals use z=1+t+xy/8 for the slab and independent
	// Cartesian intervals for the box. Ixx/Iyy combine two raw squares;
	// negative off-diagonal products test the world translation signs.
	wantInertia := [3][3]*big.Rat{
		{big.NewRat(22363, 576), big.NewRat(75, 4), big.NewRat(673, 48)},
		{big.NewRat(75, 4), big.NewRat(16603, 576), big.NewRat(-281, 16)},
		{big.NewRat(673, 48), big.NewRat(-281, 16), big.NewRat(40, 1)},
	}
	for i, row := range wantInertia {
		for j, want := range row {
			if x := m.Inertia[i][j]; !contains(x, want) || x.Lower.Cmp(m.Inertia[j][i].Lower) != 0 || x.Upper.Cmp(m.Inertia[j][i].Upper) != 0 {
				t.Fatal("independent symmetric inertia value", i, j, x, want)
			}
		}
	}
	if limited, e := u.BoundMassProperties(ctx, AlgebraicShellMeasureOptions{MaxCells: 1}); !errors.Is(e, geom.ErrConvergence) || limited.Volume.Lower != nil || limited.FirstMoment[0].Lower != nil || limited.Inertia[0][0].Lower != nil {
		t.Fatal("budget must not publish partial measures", limited, e)
	}
}

func TestAlgebraicShellMassBoundsClippedLens(t *testing.T) {
	ctx := context.Background()
	a, b, hints := transverseSplineCaps(t)
	o := AlgebraicShellSectionOptions{Seeds: []AlgebraicShellSectionSeed{{Faces: [2]int{0, 1}, Tubes: hints}}}
	o.Trace.Coverage.Isolation.SeparatingProjections = true
	o.Trace.Coverage.Isolation.ParameterTolerance = .25
	o.Trace.Coverage.Cell.MaxOperations = 16000000
	section, e := SectionSplineShellsAlgebraic(ctx, a, b, o)
	if e != nil {
		t.Fatal(e)
	}
	u, e := section.CombineAlgebraic(ctx, parametric.Intersection, AlgebraicShellBooleanOptions{})
	if e != nil {
		t.Fatal(e)
	}
	m, e := u.BoundMassProperties(ctx, AlgebraicShellMeasureOptions{MaxDepth: 10, TubeSubdivisions: 8})
	if e != nil {
		t.Fatal(e)
	}
	// Disk integration gives zbar=r²/3. Reflection symmetry gives
	// xbar=ybar=1/2. For this radius the positive Z moment is tiny;
	// its enclosure must still include an independent volume bracket.
	for axis, q := range []*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2), big.NewRat(25, 768)} {
		mlo := new(big.Rat).Mul(big.NewRat(14980281131695, 1e15), q)
		mhi := new(big.Rat).Mul(big.NewRat(14980281131696, 1e15), q)
		interval := m.FirstMoment[axis]
		if interval.Lower.Cmp(mlo) > 0 || interval.Upper.Cmp(mhi) < 0 || m.AmbiguousCells == 0 {
			t.Fatal("clipped first moment excludes analytic value", axis, interval)
		}
		centroid := m.Centroid[axis]
		if centroid.Lower != nil && (centroid.Lower.Cmp(q) > 0 || centroid.Upper.Cmp(q) < 0) {
			t.Fatal("clipped centroid excludes analytic value", axis, centroid)
		}
	}
}
