package trimmed

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/trimvalidate"
	"plainsolid/geom"
)

func TestAlgebraicCapLineJunctionOwners(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	p := func(x, y int64) [3]*big.Rat { return [3]*big.Rat{q(x, 1), q(y, 1), q(1, 1)} }
	points := [4][3]*big.Rat{p(0, 0), p(2, 0), p(2, 2), p(0, 2)}
	mid := func(a, b [3]*big.Rat) [3]*big.Rat {
		return [3]*big.Rat{new(big.Rat).Quo(new(big.Rat).Add(a[0], b[0]), q(2, 1)),
			new(big.Rat).Quo(new(big.Rat).Add(a[1], b[1]), q(2, 1)), q(1, 1)}
	}
	var pieces []projectedGraphSectionPiece
	var ring trimvalidate.Loop
	for i, a := range points {
		b := points[(i+1)%len(points)]
		n := [3][3]*big.Rat{a, mid(a, b), b}
		part, e := trimvalidate.ExactQuadratic(n)
		if e != nil {
			t.Fatal(e)
		}
		ring = append(ring, part)
		pieces = append(pieces, projectedGraphSectionPiece{control: n[:], source: RationalGraphCapEdgeSource{Piece: i}})
	}
	o := RationalGraphShellRegionOptions{}
	opts, _ := o.Topology.WithDefaults()
	r, e := certifyProjectedRationalGraphRegions(ctx, [][]projectedGraphSectionPiece{pieces}, []trimvalidate.Loop{ring}, horizontalPolynomialPlane(new(big.Rat)), opts)
	if e != nil {
		t.Fatal(e)
	}
	plane := [3]geom.Vec3{{X: 1}, {X: 1, Y: 1}, {X: 1, Z: 1}}
	c, e := r.ClipAlgebraicHalfspace(ctx, plane, true, o)
	if e != nil {
		t.Fatal(e)
	}
	ex := [2]*big.Rat{q(-1, 1), q(3, 1)}
	origin := [3]*big.Rat{q(1, 1), q(0, 1), q(0, 1)}
	d := [3]*big.Rat{q(1, 1), q(2, 1), q(0, 1)}
	rootOpts := geom.ExactCurvePlaneEventOptions{}
	if q, e := c.IntersectRationalLineWithTouches(ctx, origin, d, ex, rootOpts); !errors.Is(e, geom.ErrConvergence) || len(q.Contacts) != 0 {
		t.Fatal("old API still rejects junctions", q, e)
	}
	line, e := c.IntersectRationalLineWithJunctions(ctx, origin, d, ex, rootOpts)
	if e != nil || len(line.Material) != 1 || len(line.Contacts) != 2 {
		t.Fatal("diagonal must meet the two exact clipped corners", line, e)
	}
	for i, v := range line.Contacts {
		a, e := v.Parameter.Classify(ctx, rootOpts)
		if e != nil || !v.Junction || v.Use == v.OtherUse || v.OtherCurveParameter == nil || v.CurveParameter == nil ||
			v.Touch || v.Entry != (i == 0) || a.Kind != geom.ExactRootKindRational || a.Rational.Cmp(q(int64(i), 1)) != 0 {
			t.Fatal("corner ancestry and exact parameter", i, v, a, e)
		}
	}
	d[1] = q(-2, 1)
	line, e = c.IntersectRationalLineWithJunctions(ctx, origin, d, ex, rootOpts)
	if e != nil || len(line.Contacts) != 1 || len(line.Material) != 0 || !line.Contacts[0].Touch || !line.Contacts[0].Junction {
		t.Fatal("opposing exterior rays have an isolated corner touch", line, e)
	}
	// The clipped corner is a shared seam between the inherited
	// lower edge and an exact generated edge.
	v := line.Contacts[0]
	w := c.wires[v.Wire]
	if (w[v.Use].Seam >= 0) == (w[v.OtherUse].Seam >= 0) {
		t.Fatal("the event must retain both generated and inherited coedges", v)
	}
}

func TestAlgebraicCapBoundaryRunsRetainIrrationalSeam(t *testing.T) {
	ctx := context.Background()
	stock := makeWarpedSquareRationalGraphTube(t, 1./8)
	const z = .5 - 11./64
	planes := [2][3]geom.Vec3{
		{{Z: .5}, {Y: 1, Z: .5}, {X: 1, Z: .5}},
		{{Z: z}, {X: 1, Z: z}, {Y: 1, Z: z + 1./8}},
	}
	pair, err := stock.ClipAlgebraicGraphCapPair(ctx, planes,
		RationalGraphShellSectionOptions{ComposeSectionCurves: true}, RationalGraphShellRegionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	q := big.NewRat
	dir := [3]*big.Rat{q(-1, 1), q(0, 1), q(0, 1)}
	point := [3]*big.Rat{q(3, 1), q(11, 8), q(1, 2)}
	extent := [2]*big.Rat{q(9, 8), q(7, 4)}
	opts := geom.ExactCurvePlaneEventOptions{}
	runs, err := pair.caps[0].IntersectRationalBoundaryRuns(ctx, point, dir, extent, opts)
	if err != nil || len(runs.Runs) != 1 {
		t.Fatal("negative direction must clip only one true seam", runs, err)
	}
	run := runs.Runs[0]
	// X = sqrt(3) is the original inner-wall intersection, so
	// t = 3-sqrt(3) is the root in (1,3/2) of t²−6t+6.
	edge := run.Extent[1]
	s, err := edge.SignPolynomial(ctx, []*big.Rat{q(6, 1), q(-6, 1), q(1, 1)}, opts)
	if err != nil || s.Sign != 0 {
		t.Fatal("expected exact irrational clipped end", s, err)
	}
	i, err := edge.Enclose(ctx, q(1, 64), opts)
	if err != nil || !i.Converged || i.Interval.Lower.Cmp(q(1, 1)) <= 0 || i.Interval.Upper.Cmp(q(3, 2)) >= 0 {
		t.Fatal("must isolate the correct reflected root", i, err)
	}
	lo, err := run.Extent[0].Classify(ctx, opts)
	if err != nil || lo.Kind != geom.ExactRootKindRational || lo.Rational.Cmp(extent[0]) != 0 {
		t.Fatal("caller limit must not move", lo, err)
	}
	u := pair.caps[0].wires[run.Wire][run.Use]
	if u.Seam < 0 {
		t.Fatal("expected generated seam ancestry", run)
	}
	sameParameter, err := run.CurveParameters[0].Compare(ctx, run.CurveParameters[1], opts)
	if err != nil || !sameParameter.Proven || sameParameter.Order == 0 || run.AlongWire != ((sameParameter.Order < 0) != u.Reversed) {
		t.Fatal("original endpoint order and wire direction were lost", run, sameParameter, err)
	}
	bad, err := pair.caps[0].IntersectRationalBoundaryRuns(ctx, point, dir, extent, geom.ExactCurvePlaneEventOptions{MaxOperations: 1})
	if !errors.Is(err, geom.ErrConvergence) || len(bad.Runs) != 0 {
		t.Fatal("a partial overlap cannot escape a depleted budget", bad, err)
	}
}

func TestAlgebraicCapLineRegularOddTangency(t *testing.T) {
	ctx := context.Background()
	q := big.NewRat
	p := func(x, y *big.Rat) [3]*big.Rat { return [3]*big.Rat{x, y, q(1, 1)} }
	corners := [4][3]*big.Rat{p(q(0, 1), q(-1, 8)), p(q(1, 1), q(1, 8)), p(q(1, 1), q(1, 1)), p(q(0, 1), q(1, 1))}
	// The lower boundary is the regular graph y=(x-1/2)^3. On
	// y=0 the line exits material at the cubic stationary crossing.
	controls := [][][3]*big.Rat{{corners[0], p(q(1, 3), q(1, 8)), p(q(2, 3), q(-1, 8)), corners[1]}}
	for i := 1; i < 4; i++ {
		a, z := corners[i], corners[(i+1)%4]
		mid := func(k int) *big.Rat { return new(big.Rat).Quo(new(big.Rat).Add(a[k], z[k]), q(2, 1)) }
		controls = append(controls, [][3]*big.Rat{a, p(mid(0), mid(1)), z})
	}
	var pieces []projectedGraphSectionPiece
	var ring trimvalidate.Loop
	for i, control := range controls {
		segment, err := trimvalidate.ExactBezier(control)
		if err != nil {
			t.Fatal(err)
		}
		ring = append(ring, segment)
		pieces = append(pieces, projectedGraphSectionPiece{control: control, source: RationalGraphCapEdgeSource{Piece: i}})
	}
	options := RationalGraphShellRegionOptions{}
	topo, _ := options.Topology.WithDefaults()
	model, err := certifyProjectedRationalGraphRegions(ctx, [][]projectedGraphSectionPiece{pieces}, []trimvalidate.Loop{ring}, horizontalPolynomialPlane(q(0, 1)), topo)
	if err != nil {
		t.Fatal(err)
	}
	clip, err := model.ClipAlgebraicHalfspace(ctx, [3]geom.Vec3{{X: .25}, {X: .25, Y: 1}, {X: .25, Z: 1}}, true, options)
	if err != nil {
		t.Fatal(err)
	}
	a := [3]*big.Rat{q(0, 1), q(0, 1), q(0, 1)}
	d := [3]*big.Rat{q(1, 1), q(0, 1), q(0, 1)}
	extent := [2]*big.Rat{q(0, 1), q(1, 1)}
	opts := geom.ExactCurvePlaneEventOptions{}
	if no, err := clip.IntersectRationalLineWithTouches(ctx, a, d, extent, opts); !errors.Is(err, geom.ErrConvergence) || len(no.Contacts) != 0 {
		t.Fatal("old touch query retains its original domain", no, err)
	}
	for i := 0; i < 2; i++ {
		line, err := clip.IntersectRationalLineWithJunctions(ctx, a, d, extent, opts)
		if err != nil || len(line.Contacts) != 2 || len(line.Material) != 1 {
			t.Fatal("regular cubic tangent crossing must toggle parity", i, line, err)
		}
		want := [2]*big.Rat{q(1, 4), q(1, 2)}
		if i == 1 {
			want = [2]*big.Rat{q(1, 2), q(3, 4)}
		}
		for k, hit := range line.Contacts {
			root, err := hit.Parameter.Classify(ctx, opts)
			if err != nil || root.Kind != geom.ExactRootKindRational || root.Rational.Cmp(want[k]) != 0 ||
				hit.Entry != (k == 0) || hit.Touch || hit.Junction {
				t.Fatal("exact regular crossing order", i, k, hit, root, err)
			}
		}
		a[0], d[0] = q(1, 1), q(-1, 1)
	}
}
