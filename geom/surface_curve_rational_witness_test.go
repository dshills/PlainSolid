package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestComposedEventRationalWitness(t *testing.T) {
	ctx := context.Background()
	cell := junctionLinearCell(t, 1, 0)
	q := big.NewRat(1, 3)
	root, _, _ := NewExactRationalRoot(ctx, q, ExactCurvePlaneEventOptions{})
	event, e := cell.BindRoot(ctx, root, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	p, e := event.EvaluateRationalWitness(ctx, q, ExactCurvePlaneEventOptions{})
	if e != nil || p.Point[0].Cmp(big.NewRat(1, 3)) != 0 || p.Point[2].Cmp(big.NewRat(-1, 6)) != 0 {
		t.Fatal(p, e)
	}
	near := new(big.Rat).Add(q, new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100)))
	p, e = event.EvaluateRationalWitness(ctx, near, ExactCurvePlaneEventOptions{})
	if e != ErrDomain || p.Point[0] != nil {
		t.Fatal("near witness", p, e)
	}
	p, e = event.EvaluateRationalWitness(ctx, q, ExactCurvePlaneEventOptions{MaxOperations: 1})
	if e == nil || p.Point[0] != nil {
		t.Fatal("atomic", p, e)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = event.EvaluateRationalWitness(cancelCtx, q, ExactCurvePlaneEventOptions{}); e != context.Canceled {
		t.Fatal(e)
	}
}
func TestComposedEventIrrationalRejectsRationalWitness(t *testing.T) {
	ctx := context.Background()
	cell := composedTouchFixture(t, false)
	hits, e := cell.IntersectRawPlane(ctx, [4]*big.Rat{nil, nil, big.NewRat(1, 1), nil}, ExactCurvePlaneEventOptions{})
	if e != nil {
		t.Fatal(e)
	}
	event := hits.Events[0]
	bounds := event.Interval()
	mid := new(big.Rat).Quo(new(big.Rat).Add(bounds.Lower, bounds.Upper), big.NewRat(2, 1))
	p, e := event.EvaluateRationalWitness(ctx, mid, ExactCurvePlaneEventOptions{})
	if e != ErrDomain || p.Point[0] != nil {
		t.Fatal("bracket midpoint is not event", p, e)
	}
}
