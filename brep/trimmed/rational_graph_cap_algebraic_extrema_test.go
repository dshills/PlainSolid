//go:build research

package trimmed

import (
	"context"
	"errors"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/placement"
	"plainsolid/geom"
)

func TestAlgebraicStockDirectionalBounds(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	stock := reparameterizedGraphSleeveForFlux(t)
	planes := [2][3]geom.Vec3{
		{{Z: .25}, {Y: 1, Z: .25}, {X: 1, Z: .25}},
		{{Z: .75}, {X: 1, Z: .75}, {Y: 1, Z: .75}},
	}
	section := RationalGraphShellSectionOptions{ComposeSectionCurves: true}
	p, err := stock.ClipAlgebraicGraphBand(ctx, planes, section, RationalGraphShellRegionOptions{})
	if err != nil {
		t.Fatal(err)
	}
	opts := geom.ExactCurvePlaneEventOptions{MaxOperations: 12000000}
	stockModel, err := p.PrepareVerticalMaterial(ctx, section, opts)
	if err != nil {
		t.Fatal(err)
	}
	q := big.NewRat
	// The sleeve has 0 <= X <= 3 and X²/8 <= Y <= 3+X²/8.
	// For 2Y-3X/4, the lower envelope X²/4-3X/4
	// reaches -9/16 at X=3/2; the upper envelope has maximum 6.
	// Both remain true on either cap, with the inner void irrelevant.
	d := [3]*big.Rat{q(-3, 4), q(2, 1), q(0, 1)}
	r, err := stockModel.BoundDirectionRational(ctx, d, opts)
	if err != nil {
		t.Fatal(err)
	}
	assertValue := func(v AlgebraicGraphDirectionalWitness, want *big.Rat) {
		t.Helper()
		c, err := v.Value.Classify(ctx, opts)
		if err != nil || c.Kind != geom.ExactRootKindRational || c.Rational.Cmp(want) != 0 {
			t.Fatal("wrong exact bound", c, want, err)
		}
	}
	assertValue(r.Min, q(-9, 16))
	assertValue(r.Max, q(6, 1))
	if r.Min.ConstantUse || r.Min.Cap != 0 || r.Min.CurveParameter == nil {
		t.Fatal("a true interior stationary minimum with stable original owner is required", r.Min)
	}
	localPoint, err := stockModel.ResolveDirectionalWitness(ctx, r.Min, q(1, 1<<20), opts)
	if err != nil {
		t.Fatal(err)
	}
	expected := [3]*big.Rat{q(3, 2), q(9, 32), q(1, 4)}
	for i, c := range localPoint.Coordinates {
		assertValue(AlgebraicGraphDirectionalWitness{Value: c}, expected[i])
	}
	tampered := r.Min
	tampered.Use++
	if bad, err := stockModel.ResolveDirectionalWitness(ctx, tampered, q(1, 64), opts); !errors.Is(err, geom.ErrInvalid) || bad.Coordinates[0] != nil {
		t.Fatal("mutated source owner must not publish a different support point", bad, err)
	}
	u := p.caps[r.Min.Cap].wires[r.Min.Wire][r.Min.Use]
	lo, _ := u.Cell.EndpointRoot(0)
	hi, _ := u.Cell.EndpointRoot(1)
	for _, check := range []struct {
		r     *geom.ExactAlgebraicRoot
		order int
	}{{lo, 1}, {hi, -1}} {
		c, err := r.Min.CurveParameter.Compare(ctx, check.r, opts)
		if err != nil || !c.Proven || c.Order != check.order {
			t.Fatal("witness must lie strictly inside its original cell", c, err)
		}
	}
	vertical, err := stockModel.BoundDirectionRational(ctx, [3]*big.Rat{q(0, 1), q(0, 1), q(1, 1)}, opts)
	if err != nil || !vertical.Min.ConstantUse || !vertical.Max.ConstantUse || vertical.Min.Cap != 0 || vertical.Max.Cap != 1 {
		t.Fatal("constant cap bounds and ownership", vertical, err)
	}
	assertValue(vertical.Min, q(1, 4))
	assertValue(vertical.Max, q(3, 4))

	matrix := [3][4]*big.Rat{
		{q(3, 5), q(0, 1), q(4, 5), q(1, 7)},
		{q(0, 1), q(1, 1), q(0, 1), q(-1, 3)},
		{q(-4, 5), q(0, 1), q(3, 5), q(1, 11)},
	}
	pose, err := placement.NewRationalRigid(matrix, 4096)
	if err != nil {
		t.Fatal(err)
	}
	world, err := p.PlaceVerticalMaterialRigid(ctx, pose, section, opts)
	if err != nil {
		t.Fatal(err)
	}
	dw, err := pose.TransformVector(ctx, d, false, 4096)
	if err != nil {
		t.Fatal(err)
	}
	out, err := world.BoundDirectionRational(ctx, dw, opts)
	if err != nil {
		t.Fatal(err)
	}
	offset := new(big.Rat).Sub(new(big.Rat).Sub(q(3, 55), q(9, 140)), q(2, 3))
	assertValue(out.Min, new(big.Rat).Add(q(-9, 16), offset))
	assertValue(out.Max, new(big.Rat).Add(q(6, 1), offset))
	x := out.Min
	if x.Cap != r.Min.Cap || x.Wire != r.Min.Wire || x.Use != r.Min.Use {
		t.Fatal("exact placement may not change original owners", r.Min, out.Min)
	}
	pworld, err := world.ResolveDirectionalWitness(ctx, x, q(1, 1<<20), opts)
	if err != nil {
		t.Fatal(err)
	}
	target, err := pose.TransformPoint(ctx, expected, false, 4096)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range pworld.Coordinates {
		assertValue(AlgebraicGraphDirectionalWitness{Value: c}, target[i])
		if pworld.Bounds[i].Lower.Cmp(target[i]) > 0 || pworld.Bounds[i].Upper.Cmp(target[i]) < 0 {
			t.Fatal("world coordinate enclosure", pworld.Bounds[i], target[i])
		}
	}
	width := q(1, 256)
	localBox, err := stockModel.BoundBoxRational(ctx, width, opts)
	if err != nil {
		t.Fatal(err)
	}
	// A second proper tilt leaves world X=(3X-4Y)/5, whose upper
	// envelope reaches its maximum at X=3. The transformed local
	// box has an empty corner at X=3,Y=0; using it would give 9/5.
	tilt, err := placement.NewRationalRigid([3][4]*big.Rat{
		{q(3, 5), q(-4, 5), q(0, 1), q(1, 7)},
		{q(4, 5), q(3, 5), q(0, 1), q(-1, 3)},
		{q(0, 1), q(0, 1), q(1, 1), q(1, 11)},
	}, 4096)
	if err != nil {
		t.Fatal(err)
	}
	tw, err := p.PlaceVerticalMaterialRigid(ctx, tilt, section, opts)
	if err != nil {
		t.Fatal(err)
	}
	worldBox, err := tw.BoundBoxRational(ctx, width, opts)
	if err != nil {
		t.Fatal(err)
	}
	expectedLo := [3]*big.Rat{q(-12, 5), q(0, 1), q(1, 4)}
	expectedHi := [3]*big.Rat{q(9, 10), q(39, 8), q(3, 4)}
	for i := range localBox.Bounds {
		assertValue(worldBox.Exact[i].Min, new(big.Rat).Add(expectedLo[i], [3]*big.Rat{q(1, 7), q(-1, 3), q(1, 11)}[i]))
		assertValue(worldBox.Exact[i].Max, new(big.Rat).Add(expectedHi[i], [3]*big.Rat{q(1, 7), q(-1, 3), q(1, 11)}[i]))
		if localBox.Bounds[i].Lower == nil || worldBox.Bounds[i].Upper == nil {
			t.Fatal("every axis must have an outward rational bound", localBox, worldBox)
		}
	}
	if bad, err := tw.BoundBoxRational(ctx, width, geom.ExactCurvePlaneEventOptions{MaxOperations: worldBox.Exact[0].ExactOperations + 2}); !errors.Is(err, geom.ErrConvergence) || bad.Exact[0].Min.Value != nil {
		t.Fatal("all axes share one atomic work budget", bad, err)
	}
	if failed, err := world.BoundDirectionRational(ctx, dw, geom.ExactCurvePlaneEventOptions{MaxOperations: 1}); !errors.Is(err, geom.ErrConvergence) || failed.Min.Value != nil || failed.Max.Value != nil {
		t.Fatal("bounded failure must be atomic", failed, err)
	}
}
