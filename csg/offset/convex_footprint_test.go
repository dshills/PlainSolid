package offset

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/big"
	"slices"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/brep/trimmed"
	"plainsolid/geom"
)

func TestConvexEuclideanFootprintDilation(t *testing.T) {
	ctx := context.Background()
	frame, err := geom.NewFrame(geom.Vec3{X: 1, Y: -2, Z: 3}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	profile := []geom.Vec2{{}, {X: 4}, {Y: 3}}
	for _, height := range []float64{2, -2} {
		out, err := DilateConvexPrismFootprint(ctx, frame, profile, height, .625, 1e-9)
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Primitives) != 6 || out.AreaConstant.Cmp(big.NewRat(27, 2)) != 0 || out.VolumeConstant.Cmp(big.NewRat(27, 1)) != 0 || out.VolumePiCoefficient.Cmp(big.NewRat(25, 32)) != 0 {
			t.Fatal("incorrect exact Steiner quantities", out.Primitives, out.VolumeConstant, out.VolumePiCoefficient)
		}
		diagonal := -1
		for i, a := range profile {
			b := profile[(i+1)%len(profile)]
			if a.X != b.X && a.Y != b.Y {
				diagonal = i
			}
		}
		var found bool
		for _, p := range out.Primitives {
			if p.OriginalEdge == diagonal {
				found = true
				ends := [2]geom.Vec2{p.Start, p.End}
				a, b := geom.Vec2{X: 4.375, Y: .5}, geom.Vec2{X: .375, Y: 3.5}
				if ends != ([2]geom.Vec2{a, b}) && ends != ([2]geom.Vec2{b, a}) {
					t.Fatal("diagonal support did not move by the exact normal offset", p)
				}
			}
			if p.OriginalVertex >= 0 && p.Center != profile[p.OriginalVertex] {
				t.Fatal("rounded corner lost the original vertex", p)
			}
		}
		if !found || out.Native.Solid.Topology().EulerCharacteristic != 2 {
			t.Fatal("missing original support or closed native incidence", out.Native.Solid.Topology())
		}
		var raw bytes.Buffer
		if err = trimmed.WriteJSON(&raw, out.Native.Solid); err != nil {
			t.Fatal(err)
		}
		reloaded, err := trimmed.ReadJSONContext(ctx, &raw, trimmed.JSONLimits{})
		if err != nil {
			t.Fatal(err)
		}
		mass, err := reloaded.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6})
		if err != nil || math.Abs(mass.Volume-(27+25*math.Pi/32)) > 1e-5 {
			t.Fatal("native or analytic offset volume disagrees", mass, err)
		}
		slices.Reverse(profile)
	}
}

func TestConvexEuclideanFootprintDilationAtomic(t *testing.T) {
	ctx := context.Background()
	frame, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	irrational := []geom.Vec2{{}, {X: 2}, {Y: 2}}
	if got, err := DilateConvexPrismFootprint(ctx, frame, irrational, 2, .25, 1e-9); !errors.Is(err, geom.ErrUnrepresentable) || len(got.Primitives) != 0 {
		t.Fatal("inexact diagonal offset leaked geometry", err)
	}
	if got, err := DilateConvexPrismFootprint(ctx, frame, irrational, 2, 0, 1e-9); err != nil || got.VolumeConstant.Cmp(big.NewRat(4, 1)) != 0 || got.VolumePiCoefficient.Sign() != 0 {
		t.Fatal("zero offset must preserve the original prism", got.VolumeConstant, err)
	}
	concave := []geom.Vec2{{}, {X: 3}, {X: 2, Y: 1}, {X: 3, Y: 3}, {Y: 3}}
	if got, err := DilateConvexPrismFootprint(ctx, frame, concave, 2, .25, 1e-9); !errors.Is(err, ErrInvalid) || len(got.Primitives) != 0 {
		t.Fatal("unproved nonconvex footprint", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := DilateConvexPrismFootprint(cancelled, frame, irrational, 2, 0, 1e-9); !errors.Is(err, context.Canceled) || len(got.Primitives) != 0 {
		t.Fatal("cancelled dilation returned geometry", err)
	}
}

func TestConvexDilationExactPointOwners(t *testing.T) {
	ctx := context.Background()
	frame, err := geom.NewFrame(geom.Vec3{}, geom.Vec3{X: 1}, geom.Vec3{Z: 1})
	if err != nil {
		t.Fatal(err)
	}
	profile := []geom.Vec2{{}, {X: 4}, {Y: 3}}
	shape, err := DilateConvexPrismFootprint(ctx, frame, profile, 2, .625, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	r := big.NewRat
	for _, test := range []struct {
		point          [2]*big.Rat
		edges, corners []int
	}{
		{[2]*big.Rat{r(-3, 8), r(-1, 2)}, nil, []int{0}},
		{[2]*big.Rat{r(0, 1), r(-5, 8)}, []int{0}, []int{0}},
		{[2]*big.Rat{r(19, 8), r(2, 1)}, []int{1}, nil},
	} {
		got, err := shape.LocateRational(ctx, test.point)
		if err != nil || got.Location != parametric.Boundary || got.SquaredDistanceToProfile.Cmp(r(25, 64)) != 0 ||
			!slices.Equal(got.Edges, test.edges) || !slices.Equal(got.Corners, test.corners) {
			t.Fatal("incorrect original tangent, corner or edge ownership", got, test, err)
		}
	}
	turn := r(-3, 8)
	change := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100))
	inside, err := shape.LocateRational(ctx, [2]*big.Rat{new(big.Rat).Add(turn, change), r(-1, 2)})
	if err != nil || inside.Location != parametric.Inside {
		t.Fatal("exact interior point near a circular corner", inside, err)
	}
	outside, err := shape.LocateRational(ctx, [2]*big.Rat{new(big.Rat).Sub(turn, change), r(-1, 2)})
	if err != nil || outside.Location != parametric.Outside {
		t.Fatal("exact exterior point near a circular corner", outside, err)
	}
	shape.AreaPiCoefficient.SetInt64(0)
	shape.Primitives[0].Start.X = 100
	profile[0] = geom.Vec2{X: 100}
	again, err := shape.LocateRational(ctx, [2]*big.Rat{turn, r(-1, 2)})
	if err != nil || again.Location != parametric.Boundary || !slices.Equal(again.Corners, []int{0}) {
		t.Fatal("result or caller mutation changed the source proof", again, err)
	}
}

func TestConvexDilationExactRationalNativeSolid(t *testing.T) {
	ctx := context.Background()
	frame, err := geom.NewFrame(geom.Vec3{X: 1, Y: -2, Z: 3}, geom.Vec3{Y: 1}, geom.Vec3{X: 1})
	if err != nil {
		t.Fatal(err)
	}
	shape, err := DilateConvexPrismFootprint(ctx, frame, []geom.Vec2{{}, {X: 4}, {Y: 3}}, -2, .625, 1e-9)
	if err != nil {
		t.Fatal(err)
	}
	body, err := shape.ExactSolid(ctx, trimmed.ExactCircularLoopOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if top := body.Topology(); top.EulerCharacteristic != 2 || top.Faces != 8 || top.Edges != 18 {
		t.Fatal("exact support graph must retain one wall per original primitive", top)
	}
	var circularArcs, nonbinary int
	for i, f := range body.Faces()[2:] {
		base, _ := parametric.SurfaceMapping(f.Surface)
		patch, ok := base.(*geom.ExactBezierSurface)
		if !ok {
			t.Fatal("no exact circular support on generated wall", f)
		}
		recipe, err := patch.ExactPatch().ExportExactBezierRecipe()
		if err != nil {
			t.Fatal(err)
		}
		if recipe.DegreeU == 2 {
			primitive := shape.Primitives[i]
			for _, u := range []*big.Rat{big.NewRat(1, 3), big.NewRat(5, 7)} {
				jet, err := patch.ExactPatch().EvaluateRational(ctx, u, big.NewRat(2, 7), geom.SurfacePatchEvaluationOptions{})
				if err != nil {
					t.Fatal(err)
				}
				x, y := sub(jet.Point[0], num(primitive.Center.X)), sub(jet.Point[1], num(primitive.Center.Y))
				if add(mul(x, x), mul(y, y)).Cmp(big.NewRat(25, 64)) != 0 {
					t.Fatal("retained rational patch leaves the exact circle", i, u, jet.Point)
				}
			}
			circularArcs++
			if _, err := patch.ExactPatch().ToBSplineSurface(); errors.Is(err, geom.ErrUnrepresentable) {
				nonbinary++
			}
		}
	}
	if circularArcs != 3 || nonbinary == 0 {
		t.Fatal("fixture must exercise nonbinary homogeneous circular arc controls", circularArcs, nonbinary)
	}
	var raw bytes.Buffer
	if err := trimmed.WriteJSON(&raw, body); err != nil {
		t.Fatal(err)
	}
	body, err = trimmed.ReadJSONContext(ctx, &raw, trimmed.JSONLimits{})
	if err != nil {
		t.Fatal(err)
	}
	mass, err := body.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-6})
	if err != nil || math.Abs(mass.Volume-(27+25*math.Pi/32)) > 1e-5 {
		t.Fatal("reloaded exact circle geometry must retain independent Steiner volume", mass, err)
	}
}
