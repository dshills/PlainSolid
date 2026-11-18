package trimmed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"plainsolid/internal/slowtest"
	"testing"

	"plainsolid/brep/parametric"
	"plainsolid/geom"
)

// The independently authored power net is
// 9/16*(v-1/4-u²/8)*((u-1/2)²+(v-cy)²-1/64).
func parabolaCircleGraphStock(t *testing.T, axis int, cy, radiusSquared *big.Rat, perturb bool) [3]*geom.BSplineSurface {
	t.Helper()
	power := map[[2]int]*big.Rat{}
	constant := new(big.Rat).Add(new(big.Rat).Mul(cy, cy), big.NewRat(1, 4))
	constant.Sub(constant, radiusSquared)
	circle := map[[2]int]*big.Rat{
		{0, 0}: constant, {1, 0}: big.NewRat(-1, 1), {2, 0}: big.NewRat(1, 1),
		{0, 1}: new(big.Rat).Mul(cy, big.NewRat(-2, 1)), {0, 2}: big.NewRat(1, 1),
	}
	branch := map[[2]int]*big.Rat{{0, 1}: big.NewRat(1, 1), {0, 0}: big.NewRat(-1, 4), {2, 0}: big.NewRat(-1, 8)}
	for i, a := range circle {
		for j, b := range branch {
			ij := [2]int{i[0] + j[0], i[1] + j[1]}
			if axis == 1 {
				ij[0], ij[1] = ij[1], ij[0]
			}
			if power[ij] == nil {
				power[ij] = new(big.Rat)
			}
			term := new(big.Rat).Mul(a, b)
			power[ij].Add(power[ij], term.Mul(term, big.NewRat(9, 16)))
		}
	}
	basis := func(i, j int) *big.Rat {
		return new(big.Rat).SetFrac(new(big.Int).Binomial(int64(i), int64(j)), new(big.Int).Binomial(4, int64(j)))
	}
	var surfaces [3]*geom.BSplineSurface
	knots := []float64{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
	for surface := range surfaces {
		var poles []geom.Vec3
		for i := range 5 {
			for j := range 5 {
				z := 0.0
				if surface > 0 {
					z = 2
				}
				if surface == 2 {
					exact := big.NewRat(2, 1)
					for k, x := range power {
						term := new(big.Rat).Mul(basis(i, k[0]), basis(j, k[1]))
						exact.Add(exact, term.Mul(term, x))
					}
					var ok bool
					z, ok = exact.Float64()
					if !ok {
						t.Fatal("original control is not exactly representable", exact)
					}
					if perturb && i == 3 && j == 2 {
						z = math.Nextafter(z, math.Inf(1))
					}
				}
				poles = append(poles, geom.Vec3{X: float64(i) / 4, Y: float64(j) / 4, Z: z})
			}
		}
		var err error
		surfaces[surface], err = geom.NewBSplineSurface(4, 4, 5, 5, poles, knots, knots, nil)
		if err != nil {
			t.Fatal(err)
		}
	}
	return surfaces
}

func TestParabolaCircleGraphProof(t *testing.T) {
	ctx := context.Background()
	for axis := range 2 {
		for order := range 2 {
			for _, circle := range [][2]*big.Rat{{big.NewRat(3, 4), big.NewRat(1, 64)}, {big.NewRat(1, 8), big.NewRat(1, 256)}} {
				center, radiusSquared := circle[0], circle[1]
				src := parabolaCircleGraphStock(t, axis, center, radiusSquared, false)
				a, err := geom.NewSurfacePatch(src[1+order], geom.SurfacePatchOptions{})
				if err != nil {
					t.Fatal(err)
				}
				b, err := geom.NewSurfacePatch(src[2-order], geom.SurfacePatchOptions{})
				if err != nil {
					t.Fatal(err)
				}
				r, err := geom.DiscoverSplineGraphParabolaCircle(ctx, a, b, axis, geom.GraphPointContactOptions{})
				if err != nil {
					t.Fatal(err)
				}
				above := center.Cmp(big.NewRat(1, 2)) > 0
				sides := [3]int{1, -1, -1}
				if above {
					sides[2] = 1
				}
				if order == 1 {
					for i := range sides {
						sides[i] = -sides[i]
					}
				}
				if r.CircleAbove != above || r.RegionSides != sides || r.CircleCenter[axis].Cmp(big.NewRat(1, 2)) != 0 ||
					r.CircleCenter[1-axis].Cmp(center) != 0 || r.RadiusSquared.Cmp(radiusSquared) != 0 {
					t.Fatalf("axis %d order %d recovered wrong complete arrangement: %+v", axis, order, r)
				}
				for i, y := range [3]*big.Rat{big.NewRat(1, 4), big.NewRat(1, 4), big.NewRat(3, 8)} {
					if r.ParabolaControls[i][axis].Cmp(big.NewRat(int64(i), 2)) != 0 || r.ParabolaControls[i][1-axis].Cmp(y) != 0 {
						t.Fatal("wrong parabola controls", r.ParabolaControls)
					}
				}
			}
		}
	}
	for _, trial := range []struct {
		center  *big.Rat
		changed bool
		axis    int
		work    int
	}{
		{center: big.NewRat(3, 4), changed: true},
		{center: big.NewRat(3, 8)},
		{center: big.NewRat(7, 8)},
		{center: big.NewRat(3, 4), axis: 1},
		{center: big.NewRat(3, 4), work: 1},
	} {
		src := parabolaCircleGraphStock(t, 0, trial.center, big.NewRat(1, 64), trial.changed)
		a, err := geom.NewSurfacePatch(src[1], geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		b, err := geom.NewSurfacePatch(src[2], geom.SurfacePatchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if r, err := geom.DiscoverSplineGraphParabolaCircle(ctx, a, b, trial.axis, geom.GraphPointContactOptions{MaxOperations: trial.work}); !errors.Is(err, geom.ErrConvergence) || r != nil {
			t.Fatalf("unsupported source returned an arrangement: %+v %v", r, err)
		}
	}
}

func TestNativeParabolaCircleGraphEnvelope(t *testing.T) {
	slowtest.Skip(t)
	ctx := context.Background()
	for axis := range 2 {
		src := parabolaCircleGraphStock(t, axis, big.NewRat(3, 4), big.NewRat(1, 64), false)
		for order := range 2 {
			for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
				t.Run(fmt.Sprintf("%d/%d/%d", axis, order, op), func(t *testing.T) {
					r, err := CombineParabolaCircleSplineGraphs(ctx, src[0], src[1+order], src[2-order], axis, op, 1e-9, ParabolaCircleGraphOptions{})
					if err != nil {
						t.Fatal(err)
					}
					s, h := r.Boundary(), r.History()
					if s.Topology().InnerWires != 1 || len(s.faces[h.RoofFaces[1]].Holes) != 1 {
						t.Fatal("circle did not become an interior trim", h)
					}
					for _, face := range h.WallFaces {
						wall := s.faces[face]
						base, hasBase := graphWallSourceTrim(wall.Outer[0].Curve, false, false)
						roof, hasRoof := graphWallSourceTrim(wall.Outer[2].Curve, true, false)
						if !hasBase || !hasRoof || base.surface != src[0] || (roof.surface != src[1] && roof.surface != src[2]) {
							t.Fatal("wall lost original roof or base edge owner", face)
						}
					}
					mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
					want := 54715159.0/27525120 - math.Pi*4317/134217728
					if op == parametric.Union {
						want = 55418249.0/27525120 + math.Pi*4317/134217728
					}
					if err != nil || !(math.Abs(mass.Volume-want) <= 1e-8) {
						t.Fatal("mixed native envelope disagrees with independent signed integrals", mass, want, err)
					}
					t.Logf("volume %.16g area %.16g", mass.Volume, mass.Area)
					checkParabolaCircleGraphReplay(t, ctx, r)
				})
			}
		}
	}
}

func TestNativeLowerParabolaCircleGraphEnvelope(t *testing.T) {
	ctx := context.Background()
	src := parabolaCircleGraphStock(t, 1, big.NewRat(1, 8), big.NewRat(1, 256), false)
	for _, op := range []parametric.BooleanOp{parametric.Intersection, parametric.Union} {
		r, err := CombineParabolaCircleSplineGraphs(ctx, src[0], src[2], src[1], 1, op, 1e-9, ParabolaCircleGraphOptions{})
		if err != nil {
			t.Fatal(err)
		}
		h := r.History()
		s := r.Boundary()
		if len(s.faces[h.RoofFaces[0]].Holes) != 1 || len(s.faces[h.RoofFaces[1]].Holes) != 0 {
			t.Fatal("circle trim was assigned to the wrong side of the parabola", h)
		}
		mass, err := s.MassProperties(ctx, parametric.IntegrationOptions{RelativeTolerance: 1e-8})
		want := 109978571.0/55050240 - math.Pi*5763/8589934592
		if op == parametric.Union {
			want = 22818689.0/11010048 + math.Pi*5763/8589934592
		}
		if err != nil || !(math.Abs(mass.Volume-want) <= 1e-8) {
			t.Fatal("wrong lower-circle material", mass, want, err)
		}
	}
}

func checkParabolaCircleGraphReplay(t *testing.T, ctx context.Context, r *ParabolaCircleGraphEnvelope) {
	t.Helper()
	var raw, again bytes.Buffer
	if err := r.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	reloaded, err := ReadParabolaCircleGraphEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{}, ParabolaCircleGraphOptions{})
	if err != nil || reloaded.History() != r.History() {
		t.Fatal("source recipe replay changed mixed boundary owners", err)
	}
	for i, src := range r.sources {
		if !geom.SameSplineSurface(src, reloaded.sources[i]) {
			t.Fatal("source recipe replay changed an original spline", i)
		}
	}
	if err := reloaded.WriteJSON(ctx, &again); err != nil || !bytes.Equal(raw.Bytes(), again.Bytes()) {
		t.Fatal("source recipe replay changed canonical bytes", err)
	}
}

func TestParabolaCircleGraphReplayRefusals(t *testing.T) {
	ctx := context.Background()
	opts := ParabolaCircleGraphOptions{}
	src := parabolaCircleGraphStock(t, 0, big.NewRat(3, 4), big.NewRat(1, 64), false)
	r, err := CombineParabolaCircleSplineGraphs(ctx, src[0], src[1], src[2], 0, parametric.Intersection, 1e-9, opts)
	if err != nil {
		t.Fatal(err)
	}
	var raw bytes.Buffer
	if err := r.WriteJSON(ctx, &raw); err != nil {
		t.Fatal(err)
	}
	if got, err := ReadParabolaCircleGraphEnvelopeJSON(ctx, bytes.NewReader(raw.Bytes()), JSONLimits{MaxEntities: 1}, opts); !errors.Is(err, geom.ErrConvergence) || got != nil {
		t.Fatal("short entity budget published a shell", err)
	}
	var recipe parabolaCircleEnvelopeRecipe
	if err := json.Unmarshal(raw.Bytes(), &recipe); err != nil {
		t.Fatal(err)
	}
	recipe.Ledger.CircleEdges[2] = *recipe.Ledger.ParabolaEdge
	forged, err := json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ReadParabolaCircleGraphEnvelopeJSON(ctx, bytes.NewReader(forged), JSONLimits{}, opts); !errors.Is(err, geom.ErrConvergence) || got != nil {
		t.Fatal("forged circle ledger published a shell", err)
	}
	changed := parabolaCircleGraphStock(t, 0, big.NewRat(3, 4), big.NewRat(1, 64), true)
	recipe.Ledger = parabolaCircleRecipeLedger(r.History())
	recipe.Sources, err = encodePolynomialGraphSupports(ctx, changed)
	if err != nil {
		t.Fatal(err)
	}
	forged, err = json.Marshal(recipe)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := ReadParabolaCircleGraphEnvelopeJSON(ctx, bytes.NewReader(forged), JSONLimits{}, opts); !errors.Is(err, geom.ErrConvergence) || got != nil {
		t.Fatal("changed original roof published the earlier shell", err)
	}
	irrational := parabolaCircleGraphStock(t, 0, big.NewRat(3, 4), big.NewRat(1, 32), false)
	if got, err := CombineParabolaCircleSplineGraphs(ctx, irrational[0], irrational[1], irrational[2], 0, parametric.Intersection, 1e-9, opts); !errors.Is(err, geom.ErrUnrepresentable) || got != nil {
		t.Fatal("irrational circle produced a rounded native vertex", err)
	}
}
