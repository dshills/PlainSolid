package geom

import (
	"context"
	"math/big"
	"testing"
)

func TestFiniteOffAxisSphereCylinder(t *testing.T) {
	r := big.NewRat
	cases := []struct {
		lo, hi *big.Rat
		count  int
		closed bool
	}{
		{r(-2, 1), r(2, 1), 1, true},
		{r(1, 4), r(1, 2), 2, false},
		{r(-1, 2), r(1, 2), 2, false},
		{r(1, 1), r(2, 1), 1, false},
		{r(2, 1), r(3, 1), 0, false},
	}
	for _, c := range cases {
		q, e := IntersectFiniteOffAxisSphereCylinder(context.Background(), Vec3{X: 2}, 2, Vec3{}, Vec3{Z: 1}, 1, [2]*big.Rat{c.lo, c.hi}, SurfacePatchEvaluationOptions{})
		if e != nil || !q.DomainsComplete || !q.ConnectivityComplete || len(q.Components) != c.count {
			t.Fatal(c, q, e)
		}
		for _, component := range q.Components {
			if component.Closed != c.closed {
				t.Fatal(c, component)
			}
		}
	}
	// Rational window endpoints remain independently owned cap events.
	q, e := IntersectFiniteOffAxisSphereCylinder(context.Background(), Vec3{X: 2}, 2, Vec3{}, Vec3{Z: -3}, 1, [2]*big.Rat{r(-1, 3), r(-1, 6)}, SurfacePatchEvaluationOptions{})
	if e != nil || !q.ConnectivityComplete || len(q.Components) != 2 || len(q.CapEvents[0].Points) != 2 || q.CapEvents[0].CapParameter.Cmp(r(-1, 3)) != 0 {
		t.Fatal(q, e)
	}
	// Two disjoint support loops remain two closed components when fully kept.
	q, e = IntersectFiniteOffAxisSphereCylinder(context.Background(), Vec3{X: 2}, 4, Vec3{}, Vec3{Z: 1}, 1, [2]*big.Rat{r(-5, 1), r(5, 1)}, SurfacePatchEvaluationOptions{})
	if e != nil || len(q.Components) != 2 || !q.Components[0].Closed || !q.Components[1].Closed {
		t.Fatal(q, e)
	}
	// Singular node and a zero-angular-width cap contact are explicit domains,
	// not falsely regular component graphs.
	q, e = IntersectFiniteOffAxisSphereCylinder(context.Background(), Vec3{X: 2}, 3, Vec3{}, Vec3{Z: 1}, 1, [2]*big.Rat{r(-4, 1), r(4, 1)}, SurfacePatchEvaluationOptions{})
	if e != nil || !q.DomainsComplete || q.ConnectivityComplete {
		t.Fatal(q, e)
	}
	q, e = IntersectFiniteOffAxisSphereCylinder(context.Background(), Vec3{X: 2}, 2, Vec3{}, Vec3{Z: 1}, 1, [2]*big.Rat{r(0, 1), r(1, 1)}, SurfacePatchEvaluationOptions{})
	if e != nil || !q.DomainsComplete || q.ConnectivityComplete {
		t.Fatal(q, e)
	}
	q, e = IntersectFiniteOffAxisSphereCylinder(context.Background(), Vec3{X: 2}, 2, Vec3{}, Vec3{Z: 1}, 1, [2]*big.Rat{r(-2, 1), r(2, 1)}, SurfacePatchEvaluationOptions{MaxOperations: 1})
	if e == nil || q.DomainsComplete || q.ConnectivityComplete {
		t.Fatal(q, e)
	}
}
