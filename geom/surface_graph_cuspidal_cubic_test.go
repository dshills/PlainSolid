package geom

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"testing"
)

func TestDiscoverProjectedGraphCuspidalCubic(t *testing.T) {
	ctx := context.Background()
	for _, axes := range [][2]int{{0, 1}, {0, 2}, {2, 0}} {
		src := cubicGraphSourceFixture(t, big.NewRat(1, 4), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 3), false)
		src[1] = independentCubicGraphOwner(t, src[1])
		projection := [3]int{0, 1, 2}
		orientation := 1
		if axes != ([2]int{0, 1}) {
			projection = [3]int{0, 2, 1}
			if axes[0] == 2 {
				orientation = -1
			}
			for i := range src {
				var err error
				src[i], err = permuteGraphPatch(src[i], projection, &polynomialBudget{maxOperations: 8000000, maxBits: 16384})
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		for order := range 2 {
			t.Run(fmt.Sprintf("axes%d%d/order%d", axes[0], axes[1], order), func(t *testing.T) {
				owners := [2]*SurfacePatch{src[order], src[1-order]}
				p, err := DiscoverProjectedGraphCuspidalCubic(ctx, owners[0], owners[1], axes, order, GraphPointContactOptions{})
				if err != nil {
					t.Fatal(err)
				}
				if p.ProjectionAxes != axes || p.ProjectionOrientation != orientation || p.FreeAxis != order ||
					p.SourceDirection != 1-2*order || p.Scale.RatString() != "1/3" ||
					p.Regions[0].HeightSide != 1-2*order || p.Regions[1].HeightSide != 2*order-1 {
					t.Fatal("wrong signed canonical cusp or whole-domain regions", p)
				}
				wantUV := [2][3][2]string{
					{{"1/4", "1/2"}, {"1", "1/8"}, {"1", "7/8"}},
					{{"3", "0"}, {"9/4", "-3"}, {"15/4", "-3"}},
				}
				for event, v := range p.Vertices {
					for i := range owners {
						j := i ^ order
						for axis := range 2 {
							if v.UV[i][axis].RatString() != wantUV[j][event][axis] {
								t.Fatal("wrong original parameter of cusp or exit", i, event, v)
							}
						}
						want := uint8(0)
						if event > 0 {
							want = [2]uint8{2, 4}[j]
						}
						if v.Boundary[i] != want {
							t.Fatal("wrong original exit side", i, event, v)
						}
					}
				}
				if p.Vertices[0].Point[projection[2]].RatString() != "137/64" {
					t.Fatal("wrong original cusp height", p.Vertices[0].Point)
				}
				checkCuspidalCubicGerms(t, p, owners, projection, order)
			})
		}
	}
}

func checkCuspidalCubicGerms(t *testing.T, p *CuspidalCubicSection, owners [2]*SurfacePatch, projection [3]int, order int) {
	t.Helper()
	ctx := context.Background()
	times := [3]string{"-1/2", "0", "1/2"}
	for i, g := range p.Germs {
		if g.Range[0].RatString() != times[i] || g.Range[1].RatString() != times[i+1] {
			t.Fatal("finite cusp restrictions do not cover the original chart", g.Range)
		}
		wantSecond := [3]string{"6", "0", "27/8"}
		wantThird := ([2][3]string{{"0", "-18", "-9/16"}, {"0", "18", "9/16"}})[i]
		wantTerminal := ([2][3]string{{"3", "-9/4", "81/64"}, {"3", "9/4", "135/64"}})[i]
		for axis, world := range projection {
			if g.ModelSecond[world].RatString() != wantSecond[axis] ||
				g.ModelThird[world].RatString() != wantThird[axis] ||
				g.TerminalModel[world].RatString() != wantTerminal[axis] {
				t.Fatal("wrong exact model cusp or exit jet", i, g)
			}
		}
		if g.UVSecond[order][0].RatString() != "6" || g.UVSecond[1-order][1].RatString() != "-24" ||
			g.UVThird[order][1].RatString() != wantThird[1] ||
			g.TerminalUV[1-order][1].RatString() != "-12" {
			t.Fatal("canonical jets were used instead of original owner jets", g)
		}
		// These are independent rational evaluations of the original
		// two graph heights and parameter maps along both open germs.
		for _, q := range []*big.Rat{g.Range[0], big.NewRat(int64(2*i-1), 3), g.Range[1]} {
			u := new(big.Rat).Add(big.NewRat(1, 4), new(big.Rat).Mul(big.NewRat(3, 1), new(big.Rat).Mul(q, q)))
			v := new(big.Rat).Add(big.NewRat(1, 2), new(big.Rat).Mul(big.NewRat(3, 1), new(big.Rat).Mul(q, new(big.Rat).Mul(q, q))))
			bu := new(big.Rat).Add(big.NewRat(2, 1), new(big.Rat).Mul(big.NewRat(2, 1), v))
			bv := new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).Mul(big.NewRat(4, 1), u))
			uv := [2][2]*big.Rat{{u, v}, {bu, bv}}
			var world [2][3]*big.Rat
			for owner, source := range owners {
				params := uv[owner^order]
				jet, err := source.EvaluateRational(ctx, params[0], params[1], SurfacePatchEvaluationOptions{})
				if err != nil {
					t.Fatal(err)
				}
				world[owner] = jet.Point
			}
			for axis, a := range world[0] {
				if a.Cmp(world[1][axis]) != 0 {
					t.Fatal("original supports differ on an open germ", q, world)
				}
			}
		}
	}
	// Independent area from 2*sqrt(k)*x^(3/2) on the longitudinal interval.
	// For positive t at the exit, it equals 4*x_exit²*t_exit/5.
	far := new(big.Rat).Sub(p.Vertices[2].Point[projection[0]], p.Vertices[0].Point[projection[0]])
	area := new(big.Rat).Mul(big.NewRat(4, 5), new(big.Rat).Mul(far, far))
	area.Mul(area, p.Germs[1].Range[1])
	if area.RatString() != "9/40" {
		t.Fatal("complete between-germs area", area)
	}
}

func TestDiscoverProjectedGraphCuspidalCubicNonbinarySourceAndBudget(t *testing.T) {
	ctx := context.Background()
	axes := [2]int{0, 1}
	src := cubicGraphSourceFixture(t, big.NewRat(5, 9), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 1), false)
	p, err := DiscoverProjectedGraphCuspidalCubic(ctx, src[0], src[1], axes, 0, GraphPointContactOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Vertices[0].Point[0].RatString() != "5/9" || p.Vertices[1].Point[1].RatString() != "11/54" ||
		p.Vertices[2].Point[1].RatString() != "43/54" || p.Germs[1].Range[1].RatString() != "2/3" {
		t.Fatal("nonbinary originals or the changed source law were rounded", p.Vertices, p.Germs)
	}
	if q, err := DiscoverProjectedGraphCuspidalCubic(ctx, src[0], src[1], axes, 0,
		GraphPointContactOptions{MaxOperations: p.ExactOperations}); err != nil || q.ExactOperations != p.ExactOperations {
		t.Fatal("the recorded exact work does not reproduce", err)
	}
	if q, err := DiscoverProjectedGraphCuspidalCubic(ctx, src[0], src[1], axes, 0,
		GraphPointContactOptions{MaxOperations: p.ExactOperations - 1}); err == nil || q != nil {
		t.Fatal("truncating the final original jet must publish no cusp", err)
	}
}

func TestDiscoverProjectedGraphCuspidalCubicRefusals(t *testing.T) {
	ctx := context.Background()
	axes := [2]int{0, 1}
	for _, tc := range []struct {
		name           string
		r, s, depth, k *big.Rat
		perturb        bool
		want           error
	}{
		{"ordinary real node", big.NewRat(13, 16), big.NewRat(1, 2), big.NewRat(1, 16), big.NewRat(1, 1), false, ErrConvergence},
		{"earlier side exit", big.NewRat(1, 4), big.NewRat(1, 8), new(big.Rat), big.NewRat(1, 3), false, ErrDomain},
		{"corner exit", big.NewRat(1, 4), big.NewRat(3, 8), new(big.Rat), big.NewRat(1, 3), false, ErrDomain},
		{"boundary cusp", new(big.Rat), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 16), false, ErrDomain},
		{"irrational exit", big.NewRat(1, 4), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 8), false, ErrUnrepresentable},
		{"mixed coefficient perturbation", big.NewRat(1, 4), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 3), true, ErrConvergence},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := cubicGraphSourceFixture(t, tc.r, tc.s, tc.depth, tc.k, tc.perturb)
			if p, err := DiscoverProjectedGraphCuspidalCubic(ctx, src[0], src[1], axes, 0, GraphPointContactOptions{}); !errors.Is(err, tc.want) || p != nil {
				t.Fatal("unsupported source publishes a cusp", p, err, tc.want)
			}
		})
	}
	src := cubicGraphSourceFixture(t, big.NewRat(1, 4), big.NewRat(1, 2), new(big.Rat), big.NewRat(1, 3), false)
	translated := *src[1]
	translated.net = append([]exactHomogeneous(nil), translated.net...)
	for i, q := range translated.net {
		translated.net[i][0] = new(big.Rat).Add(q[0], big.NewRat(1, 16))
	}
	if p, err := DiscoverProjectedGraphCuspidalCubic(ctx, src[0], &translated, axes, 0, GraphPointContactOptions{}); !errors.Is(err, ErrDomain) || p != nil {
		t.Fatal("nonmatching original footprints publish a cusp", err)
	}
	lifted := *src[1]
	lifted.net = append([]exactHomogeneous(nil), lifted.net...)
	for i, q := range lifted.net {
		lifted.net[i][2] = new(big.Rat).Add(q[2], big.NewRat(1, 1024))
	}
	if p, err := DiscoverProjectedGraphCuspidalCubic(ctx, src[0], &lifted, axes, 0, GraphPointContactOptions{}); !errors.Is(err, ErrConvergence) || p != nil {
		t.Fatal("a changed free cubic constant publishes a cusp", err)
	}
	for _, cap := range []int{1, 50} {
		if p, err := DiscoverProjectedGraphCuspidalCubic(ctx, src[0], src[1], axes, 0, GraphPointContactOptions{MaxOperations: cap}); err == nil || p != nil {
			t.Fatal("truncated discovery publishes a cusp", p, err)
		}
	}
	if p, err := DiscoverProjectedGraphCuspidalCubic(ctx, src[0], src[1], [2]int{0, 2}, 0, GraphPointContactOptions{}); err == nil || p != nil {
		t.Fatal("wrong projection publishes a cusp", p, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if p, err := DiscoverProjectedGraphCuspidalCubic(canceled, src[0], src[1], axes, 0, GraphPointContactOptions{}); !errors.Is(err, context.Canceled) || p != nil {
		t.Fatal("cancelled discovery publishes a cusp", p, err)
	}
}
