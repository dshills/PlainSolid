package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/geom"
	"plainsolid/intersect"
)

func TestProjectedLensMassOnUnequalExactRectangles(t *testing.T) {
	slowtest.Skip(t)
	checkProjectedLensMass(t, false)
}

func TestProjectedLensMassOnConvexTrims(t *testing.T) {
	slowtest.Skip(t)
	checkProjectedLensMass(t, true)
}

func checkProjectedLensMass(t *testing.T, convex bool) {
	t.Helper()
	ctx := context.Background()
	l := projectedExactRoofLens(t, convex)
	m, e := l.BoundMassProperties(ctx, AlgebraicTubeLensVolumeOptions{MaxDepth: 8})
	if e != nil {
		t.Fatal(e)
	}
	// Integrate z from 0 to R²-r² over the disk. The exact volume is
	// πR⁴/2, the centroid height is R²/3, and symmetry sets X and Y.
	// 333/106 and 355/113 give a rational enclosure of π.
	scale := big.NewRat(625, 131072)
	low := new(big.Rat).Mul(scale, big.NewRat(333, 106))
	high := new(big.Rat).Mul(scale, big.NewRat(355, 113))
	if m.Volume.Lower.Sign() <= 0 || m.Volume.Lower.Cmp(low) > 0 || m.Volume.Upper.Cmp(high) < 0 ||
		new(big.Rat).Sub(m.Volume.Upper, m.Volume.Lower).Cmp(high) >= 0 {
		t.Fatal("independent disk integral", m.Volume, low, high)
	}
	for i, want := range [3]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2), big.NewRat(25, 768)} {
		if v := m.Centroid[i]; v.Lower == nil || v.Upper == nil || v.Lower.Cmp(want) > 0 || v.Upper.Cmp(want) < 0 {
			t.Fatal("independent centroid", i, v, want)
		}
	}
	// Ixy/V=-1/4 by centered disk symmetry. Izz/V is 1/2 from
	// translation plus R²/3 from the radial second moment.
	for _, q := range []struct {
		interval geom.ExactParameterInterval
		ratio    *big.Rat
	}{{m.Inertia[0][1], big.NewRat(-1, 4)}, {m.Inertia[2][2], big.NewRat(409, 768)}} {
		a, b := new(big.Rat).Mul(low, q.ratio), new(big.Rat).Mul(high, q.ratio)
		if a.Cmp(b) > 0 {
			a, b = b, a
		}
		if q.interval.Lower.Cmp(a) > 0 || q.interval.Upper.Cmp(b) < 0 {
			t.Fatal("independent inertia integral", q.interval, a, b)
		}
	}
	var limited geom.SurfacePatchOptions
	limited.MaxOperations = 512
	m, e = l.BoundMassProperties(ctx, AlgebraicTubeLensVolumeOptions{Patch: limited})
	if !errors.Is(e, geom.ErrConvergence) || m.ExactOperations > limited.MaxOperations ||
		m.Volume.Lower != nil || m.Centroid[0].Lower != nil || m.Inertia[0][0].Lower != nil {
		t.Fatal("budget failure exposed partial mass properties", m, e)
	}
}

func projectedExactRoofLens(t *testing.T, convex bool) AlgebraicProjectedTubeLens {
	t.Helper()
	ctx := context.Background()
	a, b, seeds := transverseSplineCaps(t)
	b = exactRoofForLensMesh(t, b)
	faces := [2]Face{a.Faces()[0], b.Faces()[1]}
	if convex {
		// A's oblique trim x+4y≥1 and B's x+y≥1/2 meet at a
		// nonbinary corner (1/3,1/6). Neither cuts the retained disk.
		poly := [2][]geom.Vec2{
			{{X: 1}, {X: 1, Y: 1}, {Y: 1}, {Y: .25}},
			{{X: .375, Y: .125}, {X: .625, Y: .125}, {X: .875, Y: .375}, {X: .875, Y: .625},
				{X: .625, Y: .875}, {X: .375, Y: .875}, {X: .125, Y: .625}, {X: .125, Y: .375}},
		}
		for i, points := range poly {
			faces[i].Outer = make([]Use, len(points))
			for j, p := range points {
				faces[i].Outer[j] = Use{Edge: j, UVStart: p, UVEnd: points[(j+1)%len(points)]}
			}
		}
		id := [2][3]*big.Rat{
			{big.NewRat(1, 1), new(big.Rat), new(big.Rat)},
			{new(big.Rat), big.NewRat(1, 1), new(big.Rat)},
		}
		check := func(*big.Rat) error { return nil }
		footprint, e := commonLensMassPolygon(faces, id, check)
		if e != nil {
			t.Fatal(e)
		}
		area, e := footprint.area(check)
		// From the target octagon's area 7/16, subtract the small
		// clipped triangle with base 1/8 and height 1/24.
		if e != nil || area.Cmp(big.NewRat(167, 384)) != 0 {
			t.Fatal("exact projected trim footprint", area, e)
		}
		found := false
		for _, p := range footprint {
			found = found || p[0].Cmp(big.NewRat(1, 3)) == 0 && p[1].Cmp(big.NewRat(1, 6)) == 0
		}
		if !found {
			t.Fatal("lost the nonbinary corner of the common trim")
		}
	}
	var patches [2]intersect.Patch
	for i, f := range faces {
		d, e := faceUVBounds(f)
		if e != nil {
			t.Fatal(e)
		}
		patches[i], e = algebraicShellTracePatch(f, d)
		if e != nil {
			t.Fatal(e)
		}
	}
	if patches[0].Min == patches[1].Min || patches[0].Max == patches[1].Max {
		t.Fatal("fixture must have different outer rectangles")
	}
	opts := AlgebraicTrimmedPairSectionOptions{}
	axes := [2]int{0, 1}
	opts.Clip.Projection = &axes
	opts.Trace.Coverage.Isolation.SeparatingProjections = true
	opts.Trace.Coverage.Isolation.ParameterTolerance = .25
	opts.Trace.Coverage.Cell.MaxOperations = 16000000
	s, e := SectionTrimmedSplinePair(ctx, patches, faces, &seeds, opts)
	if e != nil || !s.Complete {
		t.Fatal(s.Trace.Issues, e)
	}
	l, e := s.SealProjectedAlgebraicTubeLens(ctx, AlgebraicTubeLensOptions{})
	if e != nil || l.Topology().EulerCharacteristic != 2 || l.Faces()[1].SourceSurface != faces[1].Surface {
		t.Fatal(l.Topology(), e)
	}
	return l
}
