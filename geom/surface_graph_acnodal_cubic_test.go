package geom

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"
)

func TestDiscoverProjectedGraphAcnodalCubicWithSeparateArc(t *testing.T) {
	ctx := context.Background()
	for _, axes := range [][2]int{{0, 1}, {2, 0}} {
		src := cubicGraphSourceFixture(t, big.NewRat(13, 16), big.NewRat(1, 2), big.NewRat(-1, 8), big.NewRat(1, 1), false)
		src[1] = independentCubicGraphOwner(t, src[1])
		vertical := 2
		if axes == ([2]int{2, 0}) {
			vertical = 1
			for i, p := range src {
				var err error
				src[i], err = permuteGraphPatch(p, [3]int{0, 2, 1}, &polynomialBudget{maxOperations: 8000000, maxBits: 16384})
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		for order := range 2 {
			t.Run(fmt.Sprintf("axes_%v/order_%d", axes, order), func(t *testing.T) {
				owners := [2]*SurfacePatch{src[order], src[1-order]}
				result, err := DiscoverProjectedGraphAcnodalCubic(ctx, owners[0], owners[1], axes, order, GraphPointContactOptions{})
				if err != nil {
					t.Fatal(err)
				}
				p := result.Isolated
				orientation := 1
				if axes[0] == 2 {
					orientation = -1
				}
				if p.Boundary != ([2]uint8{}) || p.Point[vertical].RatString() != "629/256" ||
					result.IsolatedTangent[0][vertical].RatString() != "9/16" || result.IsolatedTangent[1][vertical].RatString() != "13/128" ||
					result.SourceDirection != 1-2*order || result.ProjectionOrientation != orientation || result.Gap.RatString() != "1/8" ||
					len(result.Regions) != 2 || result.Regions[0].HeightSide != 1-2*order || result.Regions[1].HeightSide != 2*order-1 {
					t.Fatal("incorrect isolated point, chart tangent or regions", result)
				}
				arc := result.Arc
				if arc == nil || arc.ModelJet[0][0].Sign() != 0 || arc.ModelJet[0][3-vertical].RatString() != "1/8" {
					t.Fatal("separate branch has no regular tangent at its turn", arc)
				}
				for i, v := range arc.Vertices {
					wantA := [3][2]string{{"15/16", "1/2"}, {"1", "29/64"}, {"1", "35/64"}}[i]
					wantB := [3][2]string{{"3", "-11/4"}, {"93/32", "-3"}, {"99/32", "-3"}}[i]
					for axis := range 2 {
						if v.UV[order][axis].RatString() != wantA[axis] || v.UV[1-order][axis].RatString() != wantB[axis] {
							t.Fatal("lost independently authored chart binding", i, v)
						}
					}
				}
				for i, edge := range arc.Edges {
					if [2]int{edge.Start, edge.End} != ([2][2]int{{1, 0}, {0, 2}})[i] {
						t.Fatal("isolated point was placed on the regular branch", edge)
					}
					mid := new(big.Rat).Mul(big.NewRat(1, 2), new(big.Rat).Add(edge.Range[0], edge.Range[1]))
					for _, station := range []*big.Rat{edge.Range[0], mid, edge.Range[1]} {
						x := new(big.Rat).Add(big.NewRat(1, 8), new(big.Rat).Mul(station, station))
						y := new(big.Rat).Mul(x, station)
						var points [2][3]*big.Rat
						for owner, patch := range owners {
							var uv [2]*big.Rat
							for axis, row := range result.ChartMaps[owner] {
								uv[axis] = new(big.Rat).Add(row[2], new(big.Rat).Add(new(big.Rat).Mul(row[0], x), new(big.Rat).Mul(row[1], y)))
							}
							p, err := patch.EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
							if err != nil {
								t.Fatal(err)
							}
							points[owner] = p.Point
						}
						for axis := range 3 {
							if points[0][axis].Cmp(points[1][axis]) != 0 {
								t.Fatal("regular branch off one original roof", station, points)
							}
						}
					}
				}
			})
		}
	}
}

func TestDiscoverProjectedGraphAcnodalCubicIsolatedAndRefusals(t *testing.T) {
	ctx := context.Background()
	axes := [2]int{0, 1}
	r, s, k := big.NewRat(13, 16), big.NewRat(1, 2), big.NewRat(1, 1)
	src := cubicGraphSourceFixture(t, r, s, big.NewRat(-1, 4), k, false)
	out, err := DiscoverProjectedGraphAcnodalCubic(ctx, src[0], src[1], axes, 0, GraphPointContactOptions{})
	if err != nil || out.Arc != nil || len(out.Regions) != 1 || out.Regions[0].HeightSide != 1 ||
		out.Isolated.UV[0][0].Cmp(r) != 0 || out.Isolated.UV[0][1].Cmp(s) != 0 {
		t.Fatal("expected only the interior double point", out, err)
	}
	for _, c := range []struct {
		name       string
		depth, mid *big.Rat
		changed    bool
		err        error
	}{
		{"cusp", new(big.Rat), s, false, ErrDomain},
		{"ordinary real node", big.NewRat(1, 16), s, false, ErrDomain},
		{"boundary turning point", big.NewRat(-3, 16), s, false, ErrDomain},
		{"boundary corner", big.NewRat(-1, 8), big.NewRat(3, 64), false, ErrDomain},
		{"irrational exit", big.NewRat(-1, 16), s, false, ErrUnrepresentable},
		{"changed residual", big.NewRat(-1, 8), s, true, ErrConvergence},
	} {
		t.Run(c.name, func(t *testing.T) {
			a := cubicGraphSourceFixture(t, r, c.mid, c.depth, k, c.changed)
			if res, err := DiscoverProjectedGraphAcnodalCubic(ctx, a[0], a[1], axes, 0, GraphPointContactOptions{}); !errors.Is(err, c.err) || res != nil {
				t.Fatal("unsupported section was published", res, err)
			}
		})
	}
	if res, err := DiscoverProjectedGraphAcnodalCubic(ctx, src[0], src[1], axes, 0,
		GraphPointContactOptions{MaxOperations: out.ExactOperations - 1}); !errors.Is(err, ErrConvergence) || res != nil {
		t.Fatal("work truncation published a result", res, err)
	}
}
