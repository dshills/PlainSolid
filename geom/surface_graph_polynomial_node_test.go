package geom

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"testing"
)

func TestSplineGraphPolynomialNode(t *testing.T) {
	ctx := context.Background()
	for _, degree := range []int{3, 4} {
		for _, axis := range []int{0, 1} {
			x := big.NewRat(1, 2)
			negUpper := []*big.Rat{big.NewRat(-5, 16), big.NewRat(-1, 4), big.NewRat(3, 8), big.NewRat(-1, 8)}
			if degree == 4 {
				x = big.NewRat(1, 3)
				negUpper = []*big.Rat{big.NewRat(-13, 48), big.NewRat(1, 16), new(big.Rat), big.NewRat(-1, 48), big.NewRat(-1, 16)}
			}
			a, b, lower := bivariateGraphFixture(t, degree, axis, []*big.Rat{big.NewRat(1, 1)}, negUpper)
			for order, pair := range [][2]*SurfacePatch{{a, b}, {b, a}} {
				node, e := DiscoverSplineGraphPolynomialNode(ctx, pair[0], pair[1], axis, GraphPointContactOptions{})
				if e != nil || !node.Proven || node.FreeAxis != axis || len(node.CertifiedCells) != 4 || node.Parameters[axis].Cmp(x) != 0 {
					t.Fatal(degree, axis, order, node, e)
				}
				for i, q := range lower {
					if node.Power[0][i].Cmp(q) != 0 || node.Power[1][i].Cmp(new(big.Rat).Neg(negUpper[i])) != 0 {
						t.Fatal("branch ordering or coefficients", node.Power)
					}
				}
				checkPolynomialNodeGeometry(t, pair, node)
				cycle := [4]int{3, 1, 2, 0}
				if axis == 1 {
					cycle = [4]int{3, 0, 2, 1}
				}
				if node.RayCycle != cycle {
					t.Fatal("counterclockwise ray order", node.RayCycle, cycle)
				}
				for i, sector := range node.Sectors {
					want := -1
					if (i+axis+order)%2 != 0 {
						want = 1
					}
					if sector.HeightSide != want {
						t.Fatal("sector sign", sector, want)
					}
				}
				for _, limit := range []int{node.ExactOperations - 1, node.ExactOperations} {
					q, e := DiscoverSplineGraphPolynomialNode(ctx, pair[0], pair[1], axis, GraphPointContactOptions{MaxOperations: limit})
					if (e == nil) != (limit == node.ExactOperations) || (e != nil && !reflect.DeepEqual(q, SplineGraphPolynomialNode{ExactOperations: q.ExactOperations})) {
						t.Fatal("atomic quota", e)
					}
				}
				if r, e := DiscoverSplineGraphTwoPolynomials(ctx, pair[0], pair[1], axis, GraphPointContactOptions{}); e == nil || r.Proven {
					t.Fatal("ordered proof accepted a crossing", e)
				}
			}
		}
	}
}

func checkPolynomialNodeGeometry(t *testing.T, pair [2]*SurfacePatch, node SplineGraphPolynomialNode) {
	t.Helper()
	ctx := context.Background()
	projectionOrientation := node.Graph.ProjectionOrientation
	uvs := [5][2]*big.Rat{node.Parameters, node.Terminals[0], node.Terminals[1], node.Terminals[2], node.Terminals[3]}
	for i, uv := range uvs {
		ja, e := pair[0].EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		jb, e := pair[1].EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		for j := range ja.Point {
			if ja.Point[j].Cmp(jb.Point[j]) != 0 || (i == 0 && ja.Point[j].Cmp(node.Point[j]) != 0) {
				t.Fatal("original support intersection", i, ja.Point, jb.Point, node.Point)
			}
		}
	}
	axis := node.FreeAxis
	for k, tangent := range node.Tangents {
		slope := new(big.Rat)
		for i := len(node.Power[k]) - 1; i >= 1; i-- {
			slope.Mul(slope, node.Parameters[axis])
			slope.Add(slope, new(big.Rat).Mul(node.Power[k][i], big.NewRat(int64(i), 1)))
		}
		if tangent[axis].Cmp(big.NewRat(1, 1)) != 0 || tangent[1-axis].Cmp(slope) != 0 {
			t.Fatal("original branch tangent", tangent, slope)
		}
	}
	if node.Tangents[0][1-axis].Cmp(node.Tangents[1][1-axis]) <= 0 {
		t.Fatal("crossing must reverse branch order transversely", node.Tangents)
	}
	for i, sector := range node.Sectors {
		uv := sector.Witness
		ja, e := pair[0].EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		jb, e := pair[1].EvaluateRational(ctx, uv[0], uv[1], SurfacePatchEvaluationOptions{})
		if e != nil || ja.Point[2].Cmp(jb.Point[2]) != sector.HeightSide {
			t.Fatal("sector witness height gap", i, sector, e)
		}
		first, last := node.RayCycle[i], node.RayCycle[(i+1)%4]
		if sector.PositiveBoundary != (SaddleSectorBoundary{first, last, projectionOrientation}) || sector.NegativeBoundary != (SaddleSectorBoundary{last, first, -projectionOrientation}) {
			t.Fatal("oriented boundaries", sector)
		}
	}
}

func TestSplineGraphPolynomialNodeOriginalUnitsAndOrientation(t *testing.T) {
	ctx := context.Background()
	a, b, _ := bivariateGraphFixture(t, 3, 0, []*big.Rat{big.NewRat(1, 1)}, []*big.Rat{big.NewRat(-5, 16), big.NewRat(-1, 4), big.NewRat(3, 8), big.NewRat(-1, 8)})
	pair := [2]*SurfacePatch{a, b}
	for i, p := range pair {
		for j := range p.knotsU {
			p.knotsU[j] = 2 + 3*p.knotsU[j]
		}
		for j := range p.knotsV {
			p.knotsV[j] = -1 + 2*p.knotsV[j]
		}
		var e error
		pair[i], e = p.Transformed(Scaling(Vec3{X: -1, Y: 1, Z: 1}))
		if e != nil {
			t.Fatal(e)
		}
	}
	node, e := DiscoverSplineGraphPolynomialNode(ctx, pair[0], pair[1], 0, GraphPointContactOptions{})
	if e != nil || !node.Proven || node.Parameters[0].Cmp(big.NewRat(7, 2)) != 0 || node.Parameters[1].Cmp(big.NewRat(-9, 32)) != 0 || node.Graph.ProjectionOrientation != -1 || node.RayCycle != ([4]int{3, 0, 2, 1}) {
		t.Fatal(node, e)
	}
	checkPolynomialNodeGeometry(t, pair, node)
}

func TestSplineGraphPolynomialNodeRejectsIncompleteCrossings(t *testing.T) {
	ctx := context.Background()
	for name, coefficients := range map[string][]*big.Rat{
		"coincident":           {},
		"endpoint":             {big.NewRat(-1, 16), big.NewRat(1, 16)},
		"separated":            {big.NewRat(1, 16)},
		"double":               {big.NewRat(1, 32), big.NewRat(-1, 8), big.NewRat(1, 8)},
		"triple":               {big.NewRat(-1, 64), big.NewRat(3, 32), big.NewRat(-3, 16), big.NewRat(1, 8)},
		"three crossings":      {big.NewRat(-3, 128), big.NewRat(11, 64), big.NewRat(-3, 8), big.NewRat(1, 4)},
		"irrational quadratic": {big.NewRat(-1, 16), new(big.Rat), big.NewRat(1, 8)},
		"irrational cubic":     {big.NewRat(-1, 16), new(big.Rat), new(big.Rat), big.NewRat(1, 8)},
		"outside":              {big.NewRat(1, 1), big.NewRat(-2, 1)},
	} {
		t.Run(name, func(t *testing.T) {
			negUpper := []*big.Rat{big.NewRat(-1, 4), new(big.Rat), new(big.Rat), new(big.Rat), big.NewRat(-1, 8)}
			for i, q := range coefficients {
				negUpper[i].Sub(negUpper[i], q)
			}
			a, b, _ := bivariateGraphFixture(t, 4, 0, []*big.Rat{big.NewRat(1, 1)}, negUpper)
			node, e := DiscoverSplineGraphPolynomialNode(ctx, a, b, 0, GraphPointContactOptions{})
			if !errors.Is(e, ErrConvergence) || !reflect.DeepEqual(node, SplineGraphPolynomialNode{ExactOperations: node.ExactOperations}) {
				t.Fatal("unproved crossing accepted or leaked", node, e)
			}
		})
	}
	a, b, _ := bivariateGraphFixture(t, 3, 0, []*big.Rat{big.NewRat(1, 1)}, []*big.Rat{big.NewRat(-5, 16), big.NewRat(-1, 4), big.NewRat(3, 8), big.NewRat(-1, 8)})
	for _, opt := range []GraphPointContactOptions{{MaxOperations: 1}, {MaxCells: 4}} {
		node, e := DiscoverSplineGraphPolynomialNode(ctx, a, b, 0, opt)
		if !errors.Is(e, ErrConvergence) || !reflect.DeepEqual(node, SplineGraphPolynomialNode{ExactOperations: node.ExactOperations}) {
			t.Fatal("incomplete budget", node, e)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if r, e := DiscoverSplineGraphPolynomialNode(canceled, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, context.Canceled) || r.Proven {
		t.Fatal("cancel", r, e)
	}
	// The last free-axis pole changes only the later source span, so the
	// first cell still proposes the exact two factors of the unmodified pair.
	last := (a.countU - 1) * a.countV
	a.net[last][2].Add(a.net[last][2], big.NewRat(1, 1024))
	if r, e := DiscoverSplineGraphPolynomialNode(ctx, a, b, 0, GraphPointContactOptions{}); !errors.Is(e, ErrConvergence) || r.Proven || len(r.CertifiedCells) != 0 {
		t.Fatal("later source cell change accepted", r, e)
	}
}
