package trimmed

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"plainsolid/geom"
)

// The independent original polynomial is 9/16*(y-p(x))*Q(x,y),
// p(x)=vertexY+(x-1/2)^2/8, Q=(x-1/2)^2+(y-cy)^2-r².
func parabolaCircleCrossingsTestStock(t *testing.T, axis int, cy, vertexY, radiusSquared *big.Rat, perturb bool) [3]*geom.BSplineSurface {
	t.Helper()
	return parabolaCircleCurvatureStock(t, axis, cy, vertexY, radiusSquared, big.NewRat(1, 8), perturb)
}

func parabolaCircleCurvatureStock(t *testing.T, axis int, cy, vertexY, radiusSquared, curvature *big.Rat, perturb bool) [3]*geom.BSplineSurface {
	t.Helper()
	circleConstant := new(big.Rat).Mul(cy, cy)
	circleConstant.Add(circleConstant, big.NewRat(1, 4)).Sub(circleConstant, radiusSquared)
	circle := map[[2]int]*big.Rat{
		{0, 0}: circleConstant, {1, 0}: big.NewRat(-1, 1), {2, 0}: big.NewRat(1, 1),
		{0, 1}: new(big.Rat).Mul(cy, big.NewRat(-2, 1)), {0, 2}: big.NewRat(1, 1),
	}
	branch := map[[2]int]*big.Rat{
		{0, 0}: new(big.Rat).Sub(new(big.Rat).Quo(curvature, big.NewRat(-4, 1)), vertexY),
		{1, 0}: curvature, {2, 0}: new(big.Rat).Neg(curvature), {0, 1}: big.NewRat(1, 1),
	}
	power := make(map[[2]int]*big.Rat)
	for i, c := range circle {
		for j, p := range branch {
			var degree [2]int
			degree[axis], degree[1-axis] = i[0]+j[0], i[1]+j[1]
			if power[degree] == nil {
				power[degree] = new(big.Rat)
			}
			value := new(big.Rat).Mul(c, p)
			power[degree].Add(power[degree], value.Mul(value, big.NewRat(9, 16)))
		}
	}
	basis := func(i, degree int) *big.Rat {
		return new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(degree)), new(big.Int).Binomial(4, int64(degree)))
	}
	knots := []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
	var surfaces [3]*geom.BSplineSurface
	for face := range surfaces {
		var poles []geom.Vec3
		for i := range 5 {
			for j := range 5 {
				z := 0.
				if face > 0 {
					z = 2
				}
				if face == 2 {
					height := big.NewRat(2, 1)
					for degree, coefficient := range power {
						term := new(big.Rat).Mul(basis(i, degree[0]), basis(j, degree[1]))
						height.Add(height, term.Mul(term, coefficient))
					}
					var ok bool
					z, ok = height.Float64()
					if !ok {
						t.Fatal("test pole is not exactly representable", height)
					}
					if perturb && i == 2 && j == 3 {
						z = math.Nextafter(z, math.Inf(1))
					}
				}
				poles = append(poles, geom.Vec3{X: float64(i) / 4, Y: float64(j) / 4, Z: z})
			}
		}
		var err error
		surfaces[face], err = geom.NewBSplineSurface(4, 4, 5, 5, poles, knots, knots, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	return surfaces
}

func parabolaCircleCrossingsStock(t *testing.T, axis int) [3]*geom.BSplineSurface {
	t.Helper()
	return parabolaCircleCrossingsTestStock(t, axis, big.NewRat(49, 128), big.NewRat(3, 8), big.NewRat(1, 16), false)
}

func parabolaCircleCrossingPatches(t *testing.T, roofs [3]*geom.BSplineSurface) [2]*geom.SurfacePatch {
	t.Helper()
	var patches [2]*geom.SurfacePatch
	for i := range patches {
		var err error
		patches[i], err = geom.NewSurfacePatch(roofs[i+1], geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}
	return patches
}

func TestParabolaCircleCrossingsSource(t *testing.T) {
	ctx := context.Background()
	opts := geom.GraphPointContactOptions{}
	for axis := range 2 {
		patch := parabolaCircleCrossingPatches(t, parabolaCircleCrossingsStock(t, axis))
		for order := range 2 {
			proof, err := geom.DiscoverSplineGraphParabolaCircleCrossings(ctx, patch[order], patch[1-order], axis, opts)
			if err != nil {
				t.Fatal(axis, order, err)
			}
			wantSigns := [4]int{1, -1, -1, 1}
			if order == 1 {
				wantSigns = [4]int{-1, 1, 1, -1}
			}
			if proof.FreeAxis != axis || proof.RegionSides != wantSigns || proof.ExactOperations == 0 ||
				proof.CircleRadius.Cmp(big.NewRat(1, 4)) != 0 || proof.RadiusSquared.Cmp(big.NewRat(1, 16)) != 0 ||
				proof.CircleCenter[axis].Cmp(big.NewRat(1, 2)) != 0 || proof.CircleCenter[1-axis].Cmp(big.NewRat(49, 128)) != 0 {
				t.Fatalf("changed exact factor coverage for %d/%d: %+v", axis, order, proof)
			}
			for i, exit := range proof.ParabolaExits {
				if exit[axis].Cmp(big.NewRat(int64(i), 1)) != 0 || exit[1-axis].Cmp(big.NewRat(13, 32)) != 0 ||
					exit[2].Cmp(big.NewRat(2, 1)) != 0 {
					t.Fatal("changed original stock exits", exit)
				}
			}
			for i, node := range proof.Nodes {
				station := big.NewRat(int64(2*i+1), 4)
				if node.Parameters[axis].Cmp(station) != 0 || node.Parameters[1-axis].Cmp(big.NewRat(49, 128)) != 0 ||
					node.Point[axis].Cmp(station) != 0 || node.Point[1-axis].Cmp(node.Parameters[1-axis]) != 0 ||
					node.Point[2].Cmp(big.NewRat(2, 1)) != 0 {
					t.Fatal("lost exact original node", node)
				}
				sectors := [2][2][4]int{{{3, 1, 0, 2}, {1, 3, 2, 0}}, {{2, 0, 1, 3}, {0, 2, 3, 1}}}[axis][i]
				slope := big.NewRat(int64(2*i-1), 16)
				if node.SectorRegions != sectors || node.Rays[0][axis].Cmp(big.NewRat(1, 1)) != 0 ||
					node.Rays[0][1-axis].Cmp(slope) != 0 ||
					node.Rays[1][axis].Sign() != 0 || node.Rays[1][1-axis].Cmp(big.NewRat(int64(1-2*axis), 1)) != 0 {
					t.Fatal("changed original branches or cyclic sector order", node)
				}
				for j, ray := range node.Rays {
					if ray[2].Sign() != 0 {
						t.Fatal("changed original flat tangent plane", ray)
					}
					next := node.Rays[(j+1)%4]
					det := new(big.Rat).Mul(ray[0], next[1])
					det.Sub(det, new(big.Rat).Mul(ray[1], next[0]))
					if det.Sign() <= 0 || proof.RegionSides[node.SectorRegions[j]] == proof.RegionSides[node.SectorRegions[(j+1)%4]] {
						t.Fatal("expected alternating transverse sectors in physical XY", j, node)
					}
				}
			}
			for i, uv := range proof.RegionWitnesses {
				a, err := patch[order].EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
				if err != nil {
					t.Fatal(err)
				}
				b, err := patch[1-order].EvaluateRational(ctx, uv[0], uv[1], geom.SurfacePatchEvaluationOptions{})
				if err != nil || a.Point[2].Cmp(b.Point[2]) != wantSigns[i] {
					t.Fatalf("region %d has the wrong sign on the original roofs: %v", i, err)
				}
			}
		}
	}
}

func TestParabolaCircleCrossingsChangedRadius(t *testing.T) {
	ctx := context.Background()
	patches := parabolaCircleCrossingPatches(t, parabolaCircleCrossingsTestStock(t, 0,
		big.NewRat(193, 512), big.NewRat(3, 8), big.NewRat(1, 64), false))
	proof, err := geom.DiscoverSplineGraphParabolaCircleCrossings(ctx, patches[0], patches[1], 0, geom.GraphPointContactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i, node := range proof.Nodes {
		if node.Parameters[0].Cmp(big.NewRat(int64(3+2*i), 8)) != 0 ||
			node.Parameters[1].Cmp(big.NewRat(193, 512)) != 0 ||
			node.Rays[0][1].Cmp(big.NewRat(int64(2*i-1), 32)) != 0 {
			t.Fatal("changed factors did not produce their own original nodes", node)
		}
	}
}

func TestParabolaCircleCrossingsSourceRefusals(t *testing.T) {
	ctx := context.Background()
	vertex, r2 := big.NewRat(3, 8), big.NewRat(1, 16)
	for _, c := range []struct {
		name           string
		cy, vertex, r2 *big.Rat
		changed        bool
		axis, work     int
		want           error
	}{
		{name: "changed coefficient", cy: big.NewRat(49, 128), vertex: vertex, r2: r2, changed: true, want: geom.ErrConvergence},
		{name: "wrong chart axis", cy: big.NewRat(49, 128), vertex: vertex, r2: r2, axis: 1, want: geom.ErrConvergence},
		{name: "separate", cy: big.NewRat(3, 4), vertex: vertex, r2: big.NewRat(1, 64), want: geom.ErrConvergence},
		{name: "tangent", cy: big.NewRat(5, 8), vertex: vertex, r2: r2, want: geom.ErrConvergence},
		{name: "off diameter", cy: big.NewRat(25, 64), vertex: vertex, r2: r2, want: geom.ErrConvergence},
		{name: "stock wall", cy: big.NewRat(1, 4), vertex: big.NewRat(31, 128), r2: r2, want: geom.ErrConvergence},
		{name: "irrational radius", cy: big.NewRat(97, 256), vertex: vertex, r2: big.NewRat(1, 32), want: geom.ErrUnrepresentable},
		{name: "short budget", cy: big.NewRat(49, 128), vertex: vertex, r2: r2, work: 1, want: geom.ErrConvergence},
	} {
		t.Run(c.name, func(t *testing.T) {
			patches := parabolaCircleCrossingPatches(t, parabolaCircleCrossingsTestStock(t, 0, c.cy, c.vertex, c.r2, c.changed))
			proof, err := geom.DiscoverSplineGraphParabolaCircleCrossings(ctx, patches[0], patches[1], c.axis, geom.GraphPointContactOptions{MaxOperations: c.work})
			if proof != nil || !errors.Is(err, c.want) {
				t.Fatalf("unsupported source published an arrangement: %+v, %v", proof, err)
			}
		})
	}
	patches := parabolaCircleCrossingPatches(t, parabolaCircleCrossingsStock(t, 0))
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if proof, err := geom.DiscoverSplineGraphParabolaCircleCrossings(cancelled, patches[0], patches[1], 0, geom.GraphPointContactOptions{}); proof != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled source published an arrangement", proof, err)
	}
}
