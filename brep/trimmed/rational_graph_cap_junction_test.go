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
