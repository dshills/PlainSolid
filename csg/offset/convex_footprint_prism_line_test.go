package offset

import (
	"context"
	"math/big"
	"slices"
	"testing"

	"plainsolid/geom"
)

func TestConvexPrismLinePlacedCapsAndGenerators(t *testing.T) {
	ctx := context.Background()
	frame, err := geom.NewFrame(geom.Vec3{X: 1, Y: -2, Z: 3}, geom.Vec3{Y: 1}, geom.Vec3{X: 1})
	if err != nil {
		t.Fatal(err)
	}
	shape, err := DilateConvexPrismFootprint(ctx, frame, []geom.Vec2{{}, {X: 4}, {Y: 3}}, -2, .625, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	r := big.NewRat
	width := r(1, 1000000)
	dir := [3]*big.Rat{r(2, 1), r(0, 1), r(0, 1)}
	full, err := shape.ClipPrismLineRational(ctx, [3]*big.Rat{r(2, 1), r(-1, 1), r(4, 1)}, dir, r(-2, 1), r(1, 1), width)
	if err != nil || len(full.Hits) != 2 || len(full.Interior) != 1 || len(full.Boundary) != 0 {
		t.Fatal("exact placed axial material", full, err)
	}
	for i, expected := range []*big.Rat{r(-3, 2), r(-1, 2)} {
		if hit := full.Hits[i]; hit.Min.Cmp(expected) != 0 || hit.Max.Cmp(expected) != 0 || !slices.Equal(hit.Caps, []int{i}) || len(hit.Corners)+len(hit.Edges) != 0 {
			t.Fatal("lost placed axial parameter or cap ownership", i, hit)
		}
	}
	one, err := shape.ClipPrismLineRational(ctx, [3]*big.Rat{r(2, 1), r(-1, 1), r(4, 1)}, dir, r(-2, 1), r(-3, 2), width)
	if err != nil || len(one.Hits) != 1 || len(one.Interior)+len(one.Boundary) != 0 || !slices.Equal(one.Hits[0].Caps, []int{0}) {
		t.Fatal("single axial interval endpoint must remain a cap hit", one, err)
	}
	generator, err := shape.ClipPrismLineRational(ctx, [3]*big.Rat{r(2, 1), r(-19, 8), r(5, 2)}, dir, r(-2, 1), r(1, 1), width)
	if err != nil || len(generator.Hits) != 2 || len(generator.Boundary) != 1 || len(generator.Interior) != 0 || !slices.Equal(generator.Boundary[0].Corners, []int{0}) {
		t.Fatal("exact circular corner generator", generator, err)
	}
	for i, hit := range generator.Hits {
		if !slices.Equal(hit.Caps, []int{i}) || !slices.Equal(hit.Corners, []int{0}) {
			t.Fatal("lost cap/corner rim tie", hit)
		}
	}
	cap, err := shape.ClipPrismLineRational(ctx, [3]*big.Rat{r(1, 1), r(-2, 1), r(5, 2)}, [3]*big.Rat{r(0, 1), r(2, 1), r(0, 1)}, r(-1, 1), r(3, 1), width)
	if err != nil || len(cap.Hits) != 2 || len(cap.Boundary) != 1 || len(cap.Interior) != 0 || !slices.Equal(cap.Boundary[0].Caps, []int{1}) ||
		cap.Boundary[0].Start.Min.Cmp(r(-3, 16)) != 0 || cap.Boundary[0].End.Max.Cmp(r(35, 16)) != 0 {
		t.Fatal("coplanar cap intersection must be boundary only", cap, err)
	}
	oblique, err := shape.ClipPrismLineRational(ctx, [3]*big.Rat{r(0, 1), r(-2, 1), r(5, 2)}, [3]*big.Rat{r(1, 1), r(1, 1), r(0, 1)}, r(-2, 1), r(3, 1), width)
	if err != nil || len(oblique.Hits) != 2 || len(oblique.Interior) != 1 || len(oblique.Boundary) != 0 ||
		oblique.Hits[0].Min.Cmp(r(-3, 8)) != 0 || oblique.Hits[1].Min.Cmp(r(1, 1)) != 0 ||
		!slices.Equal(oblique.Hits[0].Corners, []int{0}) || !slices.Equal(oblique.Hits[1].Caps, []int{1}) {
		t.Fatal("oblique corner and cap exit retain original world parameter", oblique, err)
	}
}
