package trimmed

import (
	"context"
	"math/big"
	"testing"

	"plainsolid/geom"
)

func TestMappedRoofPairwiseTurnUsesExactRationalGradient(t *testing.T) {
	ctx := context.Background()
	// In original chart coordinates x=u and y=v. The weighted height is
	// u*(b+c*v)/(1+v). The endpoint source normals turn .3477... radians
	// while the centroid axis sits more than .175 radians from one endpoint.
	b := big.NewRat(3, 16)
	c := new(big.Rat).Sub(big.NewRat(2115, 1792), b)
	var controls [][4]*big.Rat
	for u := int64(0); u <= 1; u++ {
		for j := int64(0); j <= 2; j++ {
			v := big.NewRat(j, 2)
			w := new(big.Rat).Add(big.NewRat(1, 1), v)
			y := new(big.Rat).Set(v)
			if j == 2 {
				y.Add(y, big.NewRat(1, 1))
			}
			z := new(big.Rat).Add(b, new(big.Rat).Mul(c, v))
			controls = append(controls, [4]*big.Rat{
				new(big.Rat).Mul(big.NewRat(u, 1), w), y,
				z.Mul(z, big.NewRat(u, 1)), w,
			})
		}
	}
	p, err := geom.NewExactHomogeneousBezierSurface(ctx, 1, 2, controls, geom.SurfacePatchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	n, err := newMappedRoofNormals(ctx, p, .175, new(big.Rat))
	if err != nil {
		t.Fatal(err)
	}
	if err := n.enableTurn(ctx, p, .35); err != nil {
		t.Fatal(err)
	}
	uv := [3][2]*big.Rat{
		{new(big.Rat), new(big.Rat)}, {new(big.Rat), big.NewRat(1, 1)},
		{big.NewRat(1, 100000), big.NewRat(1, 4)},
	}
	centroid := [2]*big.Rat{big.NewRat(1, 300000), big.NewRat(5, 12)}
	middle, err := p.EvaluateRational(ctx, centroid[0], centroid[1], geom.SurfacePatchEvaluationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	end, err := p.EvaluateRational(ctx, uv[0][0], uv[0][1], geom.SurfacePatchEvaluationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	center := ddCross(middle.Du, middle.Dv)
	endpoint := ddCross(end.Du, end.Dv)
	q, positive := ddTanSquared(center, endpoint)
	if !positive || q.Cmp(n.limit) <= 0 {
		t.Fatal("the fixture must challenge the single analytic-center half-angle cone", q)
	}
	var slopes roofNormalSlopeRange
	if ok, err := n.extendBernsteinGraphSlopes(ctx, p, n.normalRectangle(p, uv), &slopes); err != nil || !ok {
		t.Fatal("exact Bernstein slope enclosure failed", err)
	}
	turn, err := n.boundPairwiseTurn(ctx, p, uv)
	if err != nil || turn == nil || turn.Cmp(n.turnLimit) > 0 || slopes.turnBound(mappedNormalTurnLimit(.34)) != nil {
		t.Fatal("full pairwise bound must certify .35 radians with the tighter negative", turn, err)
	}
	for _, point := range uv {
		j, err := p.EvaluateRational(ctx, point[0], point[1], geom.SurfacePatchEvaluationOptions{})
		if err != nil {
			t.Fatal(err)
		}
		normal := ddCross(j.Du, j.Dv)
		for i := range slopes.lo {
			q := new(big.Rat).Quo(normal[i], new(big.Rat).Abs(normal[2]))
			if q.Cmp(slopes.lo[i]) < 0 || q.Cmp(slopes.hi[i]) > 0 {
				t.Fatal("exact authored graph normal falls outside its coefficient hull", i, q)
			}
		}
	}
}
