package geom

import (
	"context"
	"errors"
	"math/big"
	"slices"
	"testing"
)

func TestVerifiedNormalOffsetSweepIntersectsRationalAffineLine(t *testing.T) {
	ctx := context.Background()
	sweep, world := sweepBoundarySlabFixture(t)
	q := big.NewRat
	for _, c := range []struct {
		name               string
		p, d               [3]*big.Rat
		lo, hi             *big.Rat
		pinned, start, end []NormalOffsetSweepSide
	}{
		{"original side crossing", [3]*big.Rat{q(-1, 1), q(1, 1), q(-1, 9)}, [3]*big.Rat{q(2, 1), q(1, 2), q(1, 5)},
			q(1, 2), q(3, 2), nil, []NormalOffsetSweepSide{NormalOffsetSweepUMin}, []NormalOffsetSweepSide{NormalOffsetSweepUMax}},
		{"isolated original corner", [3]*big.Rat{q(0, 1), q(0, 1), q(-1, 3)}, [3]*big.Rat{q(1, 1), q(-1, 1), q(1, 20)},
			q(0, 1), q(0, 1), []NormalOffsetSweepSide{NormalOffsetSweepUMin, NormalOffsetSweepVMin, NormalOffsetSweepLower},
			[]NormalOffsetSweepSide{NormalOffsetSweepUMin, NormalOffsetSweepVMin, NormalOffsetSweepLower},
			[]NormalOffsetSweepSide{NormalOffsetSweepUMin, NormalOffsetSweepVMin, NormalOffsetSweepLower}},
		{"coincident original cap edge", [3]*big.Rat{q(-1, 1), q(0, 1), q(2, 7)}, [3]*big.Rat{q(3, 2), q(0, 1), q(0, 1)},
			q(2, 3), q(2, 1), []NormalOffsetSweepSide{NormalOffsetSweepVMin, NormalOffsetSweepUpper},
			[]NormalOffsetSweepSide{NormalOffsetSweepUMin, NormalOffsetSweepVMin, NormalOffsetSweepUpper},
			[]NormalOffsetSweepSide{NormalOffsetSweepUMax, NormalOffsetSweepVMin, NormalOffsetSweepUpper}},
		{"nonbinary layer crossing", [3]*big.Rat{q(2, 3), q(7, 5), q(0, 1)}, [3]*big.Rat{q(0, 1), q(0, 1), q(3, 5)},
			q(-5, 9), q(10, 21), nil, []NormalOffsetSweepSide{NormalOffsetSweepLower}, []NormalOffsetSweepSide{NormalOffsetSweepUpper}},
		{"proved miss", [3]*big.Rat{q(-1, 1), q(4, 1), q(0, 1)}, [3]*big.Rat{q(1, 1), q(0, 1), q(1, 9)},
			nil, nil, nil, nil, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			p, d := placedSweepLine(world, c.p, c.d)
			r, err := sweep.IntersectRationalLine(ctx, p, d, NormalOffsetSweepLineOptions{})
			if err != nil || !r.Complete {
				t.Fatal("exact placed affine query should finish", r, err)
			}
			if c.lo == nil {
				if r.Span != nil {
					t.Fatal("disjoint exact line reported material", r)
				}
				return
			}
			span := r.Span
			if span == nil || span.Start.Parameter.Cmp(c.lo) != 0 || span.End.Parameter.Cmp(c.hi) != 0 ||
				!slices.Equal(span.Start.Faces, c.start) || !slices.Equal(span.End.Faces, c.end) ||
				!slices.Equal(span.BoundaryFaces, c.pinned) {
				t.Fatal("exact clipping lost analytic endpoints or original side pattern", span)
			}
			checkSweepLineEndpoint(t, sweep, p, d, span.Start)
			checkSweepLineEndpoint(t, sweep, p, d, span.End)
			t.Logf("original line interval [%s,%s], boundary span faces %v, exact work %d",
				span.Start.Parameter, span.End.Parameter, span.BoundaryFaces, r.ExactOperations)
			short, err := sweep.IntersectRationalLine(ctx, p, d,
				NormalOffsetSweepLineOptions{Patch: SurfacePatchOptions{MaxOperations: r.ExactOperations - 1}})
			if !errors.Is(err, ErrConvergence) || short.Complete || short.Span != nil {
				t.Fatal("short exact work published incomplete material", short, err)
			}
		})
	}
}

func TestVerifiedNormalOffsetSweepIntersectsOriginalCurvedRuling(t *testing.T) {
	ctx := context.Background()
	sweep := sweepLineCurvedFixture(t, false)
	q := big.NewRat
	world := func(x [3]*big.Rat) [3]*big.Rat { return obliqueSweepTestTransform(x[0], x[1], x[2]) }
	atHeight := func(y, ny, nz, t *big.Rat) [3]*big.Rat {
		z := new(big.Rat).Mul(q(3, 10), new(big.Rat).Mul(y, y))
		return [3]*big.Rat{q(17, 8), new(big.Rat).Add(y, new(big.Rat).Mul(t, ny)),
			z.Add(z, new(big.Rat).Mul(t, nz))}
	}
	ruling := [3]*big.Rat{q(-2, 1), q(0, 1), q(0, 1)}
	for _, c := range []struct {
		name           string
		point          [3]*big.Rat
		seed, actualUV *big.Rat
		displacement   *big.Rat
		boundary       []NormalOffsetSweepSide
	}{
		{"nonbinary original transverse and displacement stations", atHeight(q(3, 8), q(-9, 41), q(40, 41), q(1, 17)),
			q(11, 34), q(11, 34), q(1, 17), nil},
		{"automatic nonbinary rational recovery", atHeight(q(3, 8), q(-9, 41), q(40, 41), q(1, 17)),
			nil, q(11, 34), q(1, 17), nil},
		{"automatic source midpoint", atHeight(q(1, 10), q(0, 1), q(0, 1), q(0, 1)),
			nil, q(0, 1), q(0, 1), nil},
		{"original curved cap ruling", atHeight(q(0, 1), q(0, 1), q(1, 1), q(2, 13)),
			q(-1, 10), q(-1, 10), q(2, 13), []NormalOffsetSweepSide{NormalOffsetSweepUpper}},
	} {
		t.Run(c.name, func(t *testing.T) {
			p, d := placedSweepLine(world, c.point, ruling)
			r, err := sweep.IntersectRationalLine(ctx, p, d, NormalOffsetSweepLineOptions{ProfileParameter: c.seed})
			if err != nil || !r.Complete || r.Span == nil || r.Span.AlgebraicStation != nil {
				t.Fatal("exact original curved ruling intersection should finish", r, err)
			}
			span := r.Span
			if span.Start.Parameter.Cmp(q(1, 4)) != 0 || span.End.Parameter.Cmp(q(3, 4)) != 0 ||
				span.Start.Station[0].Cmp(q(3, 1)) != 0 || span.End.Station[0].Cmp(q(2, 1)) != 0 ||
				span.Start.Faces[0] != NormalOffsetSweepUMax || span.End.Faces[0] != NormalOffsetSweepUMin ||
				!slices.Equal(span.BoundaryFaces, c.boundary) {
				t.Fatal("negative ruling direction lost exact entry, exit or whole-boundary ownership", span)
			}
			for _, end := range [2]NormalOffsetSweepLineEndpoint{span.Start, span.End} {
				if end.Station[1].Cmp(c.actualUV) != 0 || end.Station[2].Cmp(c.displacement) != 0 {
					t.Fatal("lost original projective profile clock or nonbinary displacement", end, c)
				}
				checkSweepLineEndpoint(t, sweep, p, d, end)
			}
			t.Logf("original profile %s, signed displacement %s, ruled face span %v, exact work %d",
				c.actualUV, c.displacement, span.BoundaryFaces, r.ExactOperations)
		})
	}
	base := atHeight(q(3, 8), q(-9, 41), q(40, 41), q(1, 17))
	p, d := placedSweepLine(world, base, ruling)
	refuse := func(ctx context.Context, s *VerifiedNormalOffsetSweep, d [3]*big.Rat, o NormalOffsetSweepLineOptions, want error) {
		t.Helper()
		r, err := s.IntersectRationalLine(ctx, p, d, o)
		if !errors.Is(err, want) || r.Complete || r.Span != nil {
			t.Fatal("unsupported or incomplete line invented a material interval", r, err, want)
		}
	}
	refuse(ctx, sweep, d, NormalOffsetSweepLineOptions{ProfileParameter: q(0, 1)}, ErrConvergence)
	refuse(ctx, sweep, d, NormalOffsetSweepLineOptions{ProfileParameter: q(3, 1)}, ErrDomain)
	refuse(ctx, sweepLineCurvedFixture(t, true), d, NormalOffsetSweepLineOptions{ProfileParameter: q(11, 34)}, ErrDomain)
	_, nonparallel := placedSweepLine(world, base, [3]*big.Rat{q(-2, 1), q(1, 10), q(0, 1)})
	refuse(ctx, sweep, nonparallel, NormalOffsetSweepLineOptions{}, ErrDomain)
	refuse(ctx, sweep, d, NormalOffsetSweepLineOptions{Patch: SurfacePatchOptions{MaxOperations: 1}}, ErrConvergence)
	stopped, cancel := context.WithCancel(ctx)
	cancel()
	refuse(stopped, sweep, d, NormalOffsetSweepLineOptions{}, context.Canceled)

	far := base
	far[1] = q(3, 1)
	mp, md := placedSweepLine(world, far, ruling)
	miss, err := sweep.IntersectRationalLine(ctx, mp, md, NormalOffsetSweepLineOptions{})
	if err != nil || !miss.Complete || miss.Span != nil {
		t.Fatal("transverse rational hull should certify a full curved miss", miss, err)
	}
	checkIndependentAlgebraicRuling(t, sweep, world, ruling)
}

func sweepLineCurvedFixture(t *testing.T, shear bool) *VerifiedNormalOffsetSweep {
	t.Helper()
	ctx := context.Background()
	recipe := ExactBezierSurfaceRecipe{Version: 1, DegreeU: 1, DegreeV: 2, Domain: ParameterRectangle{2, 3, -.5, .5}}
	for u := int64(0); u < 2; u++ {
		for v := int64(0); v < 3; v++ {
			x, y := big.NewRat(5+8*u, 8), big.NewRat(v-1, 2)
			z := big.NewRat([3]int64{3, -3, 3}[v], 40)
			if shear {
				x.Add(x, new(big.Rat).Quo(y, big.NewRat(4, 1)))
			}
			p := obliqueSweepTestTransform(x, y, z)
			weight := [3]*big.Rat{big.NewRat(1, 1), big.NewRat(3, 2), big.NewRat(9, 4)}[v]
			recipe.Controls = append(recipe.Controls, [4]string{
				p[0].Mul(p[0], weight).RatString(), p[1].Mul(p[1], weight).RatString(),
				p[2].Mul(p[2], weight).RatString(), weight.RatString(),
			})
		}
	}
	p, err := ImportExactBezierSurfaceRecipe(ctx, recipe, SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sweep, proof, err := NewVerifiedNormalOffsetSweep(ctx, p, big.NewRat(-1, 11), big.NewRat(2, 13),
		NormalOffsetSweepProjection{Axes: [3]int{0, 1, 2}}, NormalOffsetCoverageOptions{Patch: SurfacePatchOptions{MaxOperations: 8_000_000}})
	if err != nil || !proof.Complete {
		t.Fatal("the independently authored placed curved sweep must verify", shear, len(proof.Proven), len(proof.Unresolved), proof.ExactOperations, err)
	}
	return sweep
}

func placedSweepLine(world func([3]*big.Rat) [3]*big.Rat, point, direction [3]*big.Rat) ([3]*big.Rat, [3]*big.Rat) {
	p := world(point)
	var sum [3]*big.Rat
	for i := range sum {
		sum[i] = new(big.Rat).Add(point[i], direction[i])
	}
	d := world(sum)
	for i := range d {
		d[i].Sub(d[i], p[i])
	}
	return p, d
}

func checkSweepLineEndpoint(t *testing.T, sweep *VerifiedNormalOffsetSweep, p, d [3]*big.Rat,
	end NormalOffsetSweepLineEndpoint) {
	t.Helper()
	uv, span := sweep.Domain()
	domain := normalSweepParameterIntervals(uv, exactInterval{span.Lower, span.Upper})
	var owners []NormalOffsetSweepSide
	for i, raw := range end.Station {
		q := domain[i]
		switch {
		case raw.Cmp(q.lo) == 0:
			owners = append(owners, NormalOffsetSweepSide(i*2))
		case raw.Cmp(q.hi) == 0:
			owners = append(owners, NormalOffsetSweepSide(i*2+1))
		}
	}
	if !slices.Equal(owners, end.Faces) {
		t.Fatal("endpoint omitted or changed an original boundary face", end, owners)
	}
	point, err := sweep.EvaluateRational(context.Background(), end.Station[0], end.Station[1], end.Station[2],
		SurfacePatchEvaluationOptions{}, SurfaceNormalOffsetEvaluationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i, actual := range end.Point {
		linePoint := new(big.Rat).Add(p[i], new(big.Rat).Mul(end.Parameter, d[i]))
		if actual.Cmp(linePoint) != 0 || actual.Cmp(point.Lower[i]) != 0 || actual.Cmp(point.Upper[i]) != 0 {
			t.Fatal("endpoint did not exactly replay between the line and original sweep", i, end, point)
		}
	}
}

func TestVerifiedNormalOffsetSweepIntersectsNegativeJacobianVRule(t *testing.T) {
	ctx := context.Background()
	p, err := normalOffsetParabolicCylinder(t, true).Trim(ParameterRectangle{.484375, .515625, 0, 1})
	if err != nil {
		t.Fatal(err)
	}
	sweep, proof, err := NewVerifiedNormalOffsetSweep(ctx, p, big.NewRat(7, 10), big.NewRat(4, 5),
		NormalOffsetSweepProjection{Axes: [3]int{1, 0, 2}, Reversed: [3]bool{true, true}}, NormalOffsetCoverageOptions{})
	if err != nil || !proof.Complete || sweep.Orientation() != -1 {
		t.Fatal("expected the independently verified negative-Jacobian curved strip", proof, err)
	}
	q := big.NewRat
	p0 := [3]*big.Rat{q(1, 3), q(0, 1), q(3, 4)}
	d := [3]*big.Rat{q(-2, 1), q(0, 1), q(0, 1)}
	r, err := sweep.IntersectRationalLine(ctx, p0, d, NormalOffsetSweepLineOptions{})
	if err != nil || !r.Complete || r.Span == nil ||
		r.Span.Start.Parameter.Cmp(q(1, 6)) != 0 || r.Span.End.Parameter.Cmp(q(2, 3)) != 0 ||
		!slices.Equal(r.Span.Start.Faces, []NormalOffsetSweepSide{NormalOffsetSweepVMin}) ||
		!slices.Equal(r.Span.End.Faces, []NormalOffsetSweepSide{NormalOffsetSweepVMax}) ||
		len(r.Span.BoundaryFaces) != 0 {
		t.Fatal("negative origin Jacobian lost the original V direction or endpoint identities", r, err)
	}
	checkSweepLineEndpoint(t, sweep, p0, d, r.Span.Start)
	checkSweepLineEndpoint(t, sweep, p0, d, r.Span.End)
	t.Logf("negative-Jacobian V ruling line interval [%s,%s], exact work %d",
		r.Span.Start.Parameter, r.Span.End.Parameter, r.ExactOperations)
}
