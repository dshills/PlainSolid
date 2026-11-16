package parabolacircle

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"plainsolid/brep/edgecheck"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestPrepareNormalBridge(t *testing.T) {
	makeBoundary := func(power int) trimmed.ExactBezierBoundary {
		// Model law in the original edge clock is (t, 1-t, t^power).
		var b trimmed.ExactBezierBoundary
		b.Edges = make([]trimmed.ExactBezierBoundaryEdge, 1)
		c := make([][4]*big.Rat, 9)
		for i := range c {
			c[i] = [4]*big.Rat{
				big.NewRat(int64(i), 8),
				big.NewRat(int64(8-i), 8),
				new(big.Rat).SetFrac(binomial(i, power), binomial(8, power)),
				big.NewRat(1, 1),
			}
		}
		b.Edges[0].Model = geom.ExactBezierSpan{Start: 0, End: 1, Control: c}
		return b
	}
	decimal := func(q *big.Rat) (string, error) {
		d := new(big.Int).Set(q.Denom())
		d.Rsh(d, d.TrailingZeroBits())
		for new(big.Int).Rem(d, big.NewInt(5)).Sign() == 0 {
			d.Quo(d, big.NewInt(5))
		}
		if d.Cmp(big.NewInt(1)) != 0 {
			return "", geom.ErrUnrepresentable
		}
		return q.RatString(), nil
	}

	ctx := context.Background()
	quartic := makeBoundary(4)
	if err := PrepareNormalBridge(ctx, quartic, 0, edgecheck.Options{}, decimal); err != nil {
		t.Fatal(err)
	}
	out := quartic.Edges[0].Model
	if out.Start != 0 || out.End != 1 || len(out.Control) != 6 {
		t.Fatal("changed the exact clock or failed to derive the degree-five net", out)
	}
	for i, pole := range out.Control {
		want := [4]*big.Rat{
			big.NewRat(int64(i), 5), big.NewRat(int64(5-i), 5),
			new(big.Rat).SetFrac(binomial(i, 4), big.NewInt(5)), big.NewRat(1, 1),
		}
		for j, q := range pole {
			if q.Cmp(want[j]) != 0 {
				t.Fatal("changed the original law", i, j, q, want[j])
			}
		}
	}
	for _, c := range []struct {
		power int
		proof edgecheck.Options
		want  error
	}{
		{6, edgecheck.Options{}, geom.ErrUnrepresentable},
		{4, edgecheck.Options{MaxDegree: 4}, geom.ErrConvergence},
		{4, edgecheck.Options{MaxBits: 2}, geom.ErrConvergence},
	} {
		b := makeBoundary(c.power)
		before := b.Edges[0].Model.Control
		if err := PrepareNormalBridge(ctx, b, 0, c.proof, decimal); !errors.Is(err, c.want) {
			t.Fatal(c.power, c.proof, err)
		}
		if len(b.Edges[0].Model.Control) != len(before) || b.Edges[0].Model.Control[0][0] != before[0][0] {
			t.Fatal("failed candidate changed the source edge")
		}
	}
	affine := makeBoundary(1)
	before := affine.Edges[0].Model.Control
	if err := PrepareNormalBridge(ctx, affine, 0, edgecheck.Options{}, decimal); err != nil ||
		&affine.Edges[0].Model.Control[0] != &before[0] {
		t.Fatal("changed an already representable bridge", err)
	}
}
