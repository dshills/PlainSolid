package offset

import (
	"context"
	"errors"
	"math/big"
	"slices"
	"testing"

	"plainsolid/geom"
)

func convexLineFixture(t *testing.T) ConvexFootprintDilation {
	t.Helper()
	frame, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	result, err := DilateConvexPrismFootprint(context.Background(), frame, []geom.Vec2{{}, {X: 4}, {Y: 3}}, 2, .625, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestConvexFootprintLineContacts(t *testing.T) {
	ctx := context.Background()
	q := convexLineFixture(t)
	r := big.NewRat
	bounds := [2]*big.Rat{r(-1, 1), r(5, 1)}
	width := r(1, 1000000000000)
	got, err := q.ClipLineRational(ctx, [2]*big.Rat{r(0, 1), r(-1, 2)}, [2]*big.Rat{r(2, 1), r(0, 1)}, bounds[0], bounds[1], width)
	if err != nil || len(got.Hits) != 2 || len(got.Interior) != 1 || len(got.Boundary) != 0 {
		t.Fatal("exact two-corner secant", got, err)
	}
	for i, want := range []*big.Rat{r(-3, 16), r(35, 16)} {
		h := got.Hits[i]
		if h.Min.Cmp(want) != 0 || h.Max.Cmp(want) != 0 || !slices.Equal(h.Corners, []int{i}) || len(h.Edges) != 0 {
			t.Fatal("lost original contact parameter or arc owner", h, want)
		}
	}
	got, err = q.ClipLineRational(ctx, [2]*big.Rat{r(0, 1), r(-1, 4)}, [2]*big.Rat{r(1, 1), r(0, 1)}, bounds[0], bounds[1], width)
	if err != nil || len(got.Hits) != 2 || len(got.Interior) != 1 {
		t.Fatal("irrational two-corner secant", got, err)
	}
	for i, hit := range got.Hits {
		if !slices.Equal(hit.Corners, []int{i}) || hit.Min.Cmp(hit.Max) >= 0 || sub(hit.Max, hit.Min).Cmp(width) > 0 {
			t.Fatal("failed exact algebraic bracket", hit)
		}
		poly := func(p *big.Rat) *big.Rat {
			x := sub(p, r(int64(4*i), 1))
			return sub(mul(x, x), r(21, 64))
		}
		if poly(hit.Min).Sign()*poly(hit.Max).Sign() != -1 {
			t.Fatal("isolated endpoints fail independent quadratic sign check", hit)
		}
	}
	inside, err := q.ClipLineRational(ctx, [2]*big.Rat{r(1, 1), r(1, 1)}, [2]*big.Rat{r(1, 1), r(0, 1)}, r(0, 1), r(1, 2), width)
	if err != nil || len(inside.Hits) != 0 || len(inside.Interior) != 1 || inside.Interior[0].End.Max.Cmp(r(1, 2)) != 0 {
		t.Fatal("finite clip contained by rounded footprint", inside, err)
	}
}

func TestConvexFootprintLineTangentAndCoincidence(t *testing.T) {
	ctx := context.Background()
	q := convexLineFixture(t)
	r := big.NewRat
	width := r(1, 1000)
	got, err := q.ClipLineRational(ctx, [2]*big.Rat{r(-3, 8), r(-1, 2)}, [2]*big.Rat{r(4, 1), r(-3, 1)}, r(-1, 1), r(1, 1), width)
	if err != nil || len(got.Hits) != 1 || len(got.Interior)+len(got.Boundary) != 0 || got.Hits[0].Min.Sign() != 0 || !slices.Equal(got.Hits[0].Corners, []int{0}) {
		t.Fatal("single rational corner tangent", got, err)
	}
	got, err = q.ClipLineRational(ctx, [2]*big.Rat{r(0, 1), r(-5, 8)}, [2]*big.Rat{r(1, 1), r(0, 1)}, r(-1, 1), r(5, 1), width)
	if err != nil || len(got.Interior) != 0 || len(got.Hits) != 2 || len(got.Boundary) != 1 {
		t.Fatal("straight supporting edge must be a boundary span", got, err)
	}
	span := got.Boundary[0]
	if span.Start.Min.Sign() != 0 || span.End.Min.Cmp(r(4, 1)) != 0 || !slices.Equal(span.Edges, []int{0}) ||
		!slices.Equal(span.Start.Corners, []int{0}) || !slices.Equal(span.End.Corners, []int{1}) {
		t.Fatal("coincident edge lost its tangent corner owners", span)
	}
	bad := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 1050))
	if denied, err := q.ClipLineRational(ctx, [2]*big.Rat{r(0, 1), r(-1, 4)}, [2]*big.Rat{r(1, 1), r(0, 1)}, r(-1, 1), r(5, 1), bad); !errors.Is(err, ErrLimit) || len(denied.Hits) != 0 {
		t.Fatal("exhausted algebraic bracket limit leaked contacts", denied, err)
	}
}
