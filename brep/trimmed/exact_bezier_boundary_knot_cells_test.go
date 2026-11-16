package trimmed

import (
	"context"
	"errors"
	"math/big"
	"slices"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

func TestExactBezierBoundaryCuspidalCubicCrease(t *testing.T) {
	ctx := context.Background()
	src := cuspidalCubicKnotStock(t)
	common, err := CommonCuspidalCubicSplineGraphs(ctx, src[0], src[1], src[2], 0, 1e-9, CuspidalCubicCommonOptions{})
	if err != nil {
		t.Fatal(err)
	}
	s := common.Boundary()
	if b, err := s.ExtractExactBezierBoundary(ctx, edgecheck.Options{}); !errors.Is(err, geom.ErrConvergence) || len(b.Edges) != 0 {
		t.Fatal("ordinary extraction must keep its one-span gate", err)
	}
	b, err := s.ExtractExactBezierBoundaryAtRationalKnots(ctx, edgecheck.Options{})
	if err != nil || len(b.Faces) != len(s.faces) || len(b.Edges) != len(s.edges)+4 || len(b.Vertices) != len(s.vertices)+4 ||
		!slices.Equal(b.Vertices[:len(s.vertices)], s.vertices) {
		t.Fatal("exact extraction did not retain native faces, vertices and four knot crossings", err)
	}
	for i, face := range common.History().RoofFaces {
		if b.Faces[face].Spline != src[1+common.History().RoofOwners[i]] {
			t.Fatal("neutral extraction replaced an original knotted chart")
		}
	}
	for i := len(s.vertices); i < len(b.Vertices); i++ {
		if b.Vertices[i].X != 21.0/16 {
			t.Fatal("neutral joint left its physical crease", i, b.Vertices[i])
		}
		// Each actual model crease adds exactly 1/16 to dZ/dX.
		// The ratio of endpoint control differences gives the
		// native derivative quotient independently of fragment clock.
		var derivatives [2]*big.Rat
		for _, e := range b.Edges {
			var a, z [4]*big.Rat
			net := e.Model.Control
			side := 0
			switch {
			case e.Start == i:
				a, z = net[0], net[1]
				side = 1
			case e.End == i:
				a, z = net[len(net)-1], net[len(net)-2]
			default:
				continue
			}
			d := func(axis int) *big.Rat {
				return new(big.Rat).Sub(new(big.Rat).Quo(z[axis], z[3]), new(big.Rat).Quo(a[axis], a[3]))
			}
			dx := d(0)
			if dx.Sign() == 0 || derivatives[side] != nil {
				t.Fatal("neutral joint lost its two directed owner fragments", i)
			}
			derivatives[side] = new(big.Rat).Quo(d(2), dx)
		}
		if derivatives[0] == nil || derivatives[1] == nil {
			t.Fatal("neutral joint did not retain both incident fragments", i)
		}
		jump := new(big.Rat).Sub(derivatives[1], derivatives[0])
		if jump.Cmp(big.NewRat(1, 16)) != 0 && jump.Cmp(big.NewRat(-1, 16)) != 0 {
			t.Fatal("source crease was smoothed or changed", i, jump)
		}
	}
	incidence := make([]int, len(b.Edges))
	for _, face := range b.Faces {
		for _, loop := range face.Loops {
			for i, use := range loop {
				next := loop[(i+1)%len(loop)]
				end, start := b.Edges[use.Edge].End, b.Edges[next.Edge].Start
				if use.Reversed {
					end = b.Edges[use.Edge].Start
				}
				if next.Reversed {
					start = b.Edges[next.Edge].End
				}
				if end != start {
					t.Fatal("knot split broke an original directed loop")
				}
				incidence[use.Edge]++
			}
		}
	}
	for i, count := range incidence {
		if count != 2 {
			t.Fatal("neutral edge did not retain both original uses", i, count)
		}
	}
}

func TestExactBezierBoundaryRejectsUnrepresentedKnotCells(t *testing.T) {
	ctx := context.Background()
	src := cuspidalCubicKnotStock(t)
	for _, test := range []struct {
		u [4]*big.Rat
		v [4]*big.Rat
	}{
		// u=t² crosses 7/16 at an irrational t.
		{[4]*big.Rat{big.NewRat(0, 1), big.NewRat(0, 1), big.NewRat(1, 3), big.NewRat(1, 1)},
			[4]*big.Rat{big.NewRat(1, 2), big.NewRat(1, 2), big.NewRat(1, 2), big.NewRat(1, 2)}},
		// u=t crosses at 7/16 but v=t(1−t)/5 has a nonbinary model Y.
		{[4]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 3), big.NewRat(2, 3), big.NewRat(1, 1)},
			[4]*big.Rat{big.NewRat(0, 1), big.NewRat(1, 15), big.NewRat(1, 15), big.NewRat(0, 1)}},
	} {
		net := make([][3]*big.Rat, 4)
		for i := range net {
			net[i] = [3]*big.Rat{test.u[i], test.v[i], big.NewRat(1, 1)}
		}
		q, err := NewExactBezierUV(net)
		if err != nil {
			t.Fatal(err)
		}
		var end [2]geom.Vec3
		eval := exactGraphVertexEvaluator(ctx, geom.GraphPointContactOptions{})
		for i, p := range q.ends {
			end[i], err = eval(src[1], openSectionPiece{a: p})
			if err != nil {
				t.Fatal(err)
			}
		}
		edge := parametric.Edge{Start: 0, End: 1, Curve: surfaceTrimCurve{src[1], q.ends[0], q.ends[1], q}, Range: parametric.Interval{Max: 1}}
		b := ExactBezierBoundary{Vertices: end[:], Edges: make([]ExactBezierBoundaryEdge, 1)}
		if parts, err := b.splitRationalKnotEdge(ctx, 0, edge, edgecheck.Options{}); !errors.Is(err, geom.ErrUnrepresentable) || len(parts) != 0 {
			t.Fatal("irreducible station or newly unrepresented vertex passed exact splitting", err)
		}
	}
}
