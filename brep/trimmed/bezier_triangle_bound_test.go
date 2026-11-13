package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
	"testing"
)

func quarticBoundFixture(t *testing.T, rational bool) *geom.SurfacePatch {
	t.Helper()
	var poles []geom.Vec3
	var weights []float64
	for i := 0; i < 5; i++ {
		for j := 0; j < 5; j++ {
			z := float64(i*j) / 64
			if i == 4 {
				z++
			}
			if j == 4 {
				z++
			}
			poles = append(poles, geom.Vec3{X: float64(i) / 4, Y: float64(j) / 4, Z: z})
			w := 1.
			if rational {
				w = []float64{.75, 1, 1.5}[(i+2*j)%3]
			}
			weights = append(weights, w)
		}
	}
	s, e := geom.NewBSplineSurface(4, 4, 5, 5, poles, []float64{2, 2, 2, 2, 2, 5, 5, 5, 5, 5}, []float64{-1, -1, -1, -1, -1, 2, 2, 2, 2, 2}, weights)
	if e != nil {
		t.Fatal(e)
	}
	p, e := geom.NewSurfacePatch(s, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestQuarticRationalTriangleResidualIndependent(t *testing.T) {
	ctx := context.Background()
	for _, rational := range []bool{false, true} {
		p := quarticBoundFixture(t, rational)
		var prior *big.Rat
		for _, den := range []int64{2, 4} {
			a, b := big.NewRat(17, 7), big.NewRat(-4, 13)
			h := big.NewRat(1, den)
			uv := [3][2]*big.Rat{{a, b}, {new(big.Rat).Add(a, h), b}, {a, new(big.Rat).Add(b, h)}}
			var model [3][3]*big.Rat
			for i, q := range uv {
				jet, e := p.EvaluateRational(ctx, q[0], q[1], geom.SurfacePatchEvaluationOptions{})
				if e != nil {
					t.Fatal(e)
				}
				model[i] = jet.Point
			}
			bound, e := BoundRationalBezierTriangleDeviation(ctx, p, uv, model, BezierTriangleBoundOptions{})
			if e != nil || !bound.BothDirections || bound.SquaredDistance.Sign() <= 0 {
				t.Fatal(bound, e)
			}
			for i := int64(0); i <= 10; i++ {
				for j := int64(0); j <= 10-i; j++ {
					s, z := big.NewRat(i, 10), big.NewRat(j, 10)
					w := new(big.Rat).Sub(big.NewRat(1, 1), new(big.Rat).Add(s, z))
					q := [2]*big.Rat{}
					for axis := range q {
						q[axis] = new(big.Rat).Add(new(big.Rat).Mul(w, uv[0][axis]), new(big.Rat).Add(new(big.Rat).Mul(s, uv[1][axis]), new(big.Rat).Mul(z, uv[2][axis])))
					}
					jet, e := p.EvaluateRational(ctx, q[0], q[1], geom.SurfacePatchEvaluationOptions{})
					if e != nil {
						t.Fatal(e)
					}
					squared := new(big.Rat)
					for axis, x := range jet.Point {
						target := new(big.Rat).Add(new(big.Rat).Mul(w, model[0][axis]), new(big.Rat).Add(new(big.Rat).Mul(s, model[1][axis]), new(big.Rat).Mul(z, model[2][axis])))
						d := new(big.Rat).Sub(x, target)
						squared.Add(squared, new(big.Rat).Mul(d, d))
					}
					if squared.Cmp(bound.SquaredDistance) > 0 {
						t.Fatal("independent exact evaluator escaped bound", squared, bound.SquaredDistance)
					}
				}
			}
			if prior != nil && new(big.Rat).Mul(bound.SquaredDistance, big.NewRat(4, 1)).Cmp(prior) >= 0 {
				t.Fatal("bound fails meaningful refinement convergence")
			}
			prior = bound.SquaredDistance
			for _, options := range []BezierTriangleBoundOptions{{MaxOperations: 1}, {MaxDegree: 3}} {
				bad, e := BoundRationalBezierTriangleDeviation(ctx, p, uv, model, options)
				if e == nil || bad.SquaredDistance != nil || bad.BothDirections {
					t.Fatal("partial/unsupported certificate", e)
				}
			}
			wide := uv
			wide[0][0] = new(big.Rat).Add(big.NewRat(2, 1), new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 100)))
			if bad, e := BoundRationalBezierTriangleDeviation(ctx, p, wide, model, BezierTriangleBoundOptions{MaxBits: 64}); e == nil || bad.SquaredDistance != nil {
				t.Fatal("input bit budget", e)
			}
			t.Log(rational, den, bound.Operations)
		}
	}
}

func TestQuarticKnotTriangleOptIn(t *testing.T) {
	ctx := context.Background()
	p := quarticBoundFixture(t, true)
	var e error
	p, e = p.InsertUKnot(3.5, 4, geom.SurfacePatchOptions{})
	if e != nil {
		t.Fatal(e)
	}
	uv := [3]geom.Vec2{{X: 2.5, Y: -.5}, {X: 4.5, Y: -.5}, {X: 2.5, Y: 1.5}}
	var exact [3][2]*big.Rat
	var model [3]geom.Vec3
	for i, q := range uv {
		exact[i] = [2]*big.Rat{ratInset(q.X), ratInset(q.Y)}
		jet, e := p.EvaluateRational(ctx, exact[i][0], exact[i][1], geom.SurfacePatchEvaluationOptions{})
		if e != nil {
			t.Fatal(e)
		}
		x, _ := jet.Point[0].Float64()
		y, _ := jet.Point[1].Float64()
		z, _ := jet.Point[2].Float64()
		model[i] = geom.Vec3{X: x, Y: y, Z: z}
	}
	if _, e := BoundMultiSpanSplineTriangleDeviation(ctx, p, uv, model, MultiSpanTriangleBoundOptions{}); e == nil {
		t.Fatal("default degree gate")
	}
	result, e := BoundMultiSpanSplineTriangleDeviation(ctx, p, uv, model, MultiSpanTriangleBoundOptions{MaxDegree: 4})
	if e != nil || !result.BothDirections || result.Cells != 2 || len(result.Pieces) < 2 || result.ResidualOperations == 0 {
		t.Fatal(result, e)
	}
	if bad, e := BoundMultiSpanSplineTriangleDeviation(ctx, p, uv, model, MultiSpanTriangleBoundOptions{MaxDegree: 4, MaxResidualOperations: 1}); e == nil || bad.SquaredDistance != nil {
		t.Fatal("residual budget", e)
	}
	work := multiSpanMeshWork{options: MultiSpanCurvedTrimMeshOptions{Span: MultiSpanTriangleBoundOptions{MaxDegree: 4, MaxCells: 16, MaxPieces: 32}, MaxSpanCells: 64, MaxSpanPieces: 64, MaxResidualOperations: 1}}
	if a, b, e := work.lipschitz(ctx, p); e != nil || a.Sign() <= 0 || b.Sign() <= 0 {
		t.Fatal("quartic Jacobian", a, b, e)
	}
	if _, e := work.bound(ctx, p, exact, model); e == nil {
		t.Fatal("cumulative residual budget")
	}
	if _, e := BoundRationalBezierTriangleDeviation(ctx, &geom.SurfacePatch{}, exact, [3][3]*big.Rat{}, BezierTriangleBoundOptions{}); e == nil {
		t.Fatal("zero patch")
	}
	t.Log(result.ResidualOperations, len(result.Pieces))
}
