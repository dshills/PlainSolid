package trimmed

import (
	"context"
	"errors"
	"math/big"
	"math/rand"
	"plainsolid/geom"
	"testing"
)

func quadraticBoundaryReference(ctx context.Context, vertices []geom.Vec3, edges [][]int, maxPairs int) (int, error) {
	if ctx == nil || maxPairs < 1 {
		return 0, geom.ErrInvalid
	}
	if e := ctx.Err(); e != nil {
		return 0, e
	}
	type segment struct{ a, b int }
	var segments []segment
	for _, edge := range edges {
		if len(edge) < 2 {
			return 0, geom.ErrInvalid
		}
		for i := 1; i < len(edge); i++ {
			a, b := edge[i-1], edge[i]
			if a < 0 || b < 0 || a >= len(vertices) || b >= len(vertices) || !vertices[a].IsFinite() || !vertices[b].IsFinite() {
				return 0, geom.ErrInvalid
			}
			if a == b || vertices[a] == vertices[b] {
				return 0, geom.ErrDegenerate
			}
			segments = append(segments, segment{a, b})
		}
	}
	// The bounded producer controls memory; reject impossible pair quotas before
	// preparing rational coordinates. This count includes broadphase exclusions.
	count := len(segments)
	if count > 1 && int64(count)*int64(count-1)/2 > int64(maxPairs) {
		return 0, geom.ErrConvergence
	}
	points := make([][3]*big.Rat, len(vertices))
	for i, p := range vertices {
		if !p.IsFinite() {
			return 0, geom.ErrInvalid
		}
		points[i] = exactBoundPoint(p)
	}
	checks := 0
	for i, a := range segments {
		for _, b := range segments[i+1:] {
			if e := ctx.Err(); e != nil {
				return checks, e
			}
			checks++
			av, bv, cv, dv := vertices[a.a], vertices[a.b], vertices[b.a], vertices[b.b]
			if max(av.X, bv.X) < min(cv.X, dv.X) || max(cv.X, dv.X) < min(av.X, bv.X) || max(av.Y, bv.Y) < min(cv.Y, dv.Y) || max(cv.Y, dv.Y) < min(av.Y, bv.Y) || max(av.Z, bv.Z) < min(cv.Z, dv.Z) || max(cv.Z, dv.Z) < min(av.Z, bv.Z) {
				continue
			}
			common := -1
			for _, x := range []int{a.a, a.b} {
				if x == b.a || x == b.b {
					common = x
				}
			}
			var shared *[3]*big.Rat
			if common >= 0 {
				shared = &points[common]
			}
			if boundarySegmentsUnexpected(points[a.a], points[a.b], points[b.a], points[b.b], shared) {
				return checks, geom.ErrDegenerate
			}
		}
	}
	return checks, nil
}

func TestBoundarySweepAgainstQuadratic(t *testing.T) {
	rng := rand.New(rand.NewSource(37))
	for run := 0; run < 100; run++ {
		v := make([]geom.Vec3, 24)
		for i := range v {
			v[i] = geom.Vec3{X: float64(rng.Intn(11)), Y: float64(rng.Intn(11)), Z: float64(rng.Intn(11))}
		}
		edges := make([][]int, 12)
		for i := range edges {
			edges[i] = []int{2 * i, 2*i + 1}
		}
		a, ea := quadraticBoundaryReference(context.Background(), v, edges, 1000)
		b, eb := checkBoundaryPolylineEmbedding(context.Background(), v, edges, 1000)
		if !errors.Is(ea, eb) || (ea == nil && a != b) {
			t.Fatal(run, a, b, ea, eb)
		}
	}
}

func BenchmarkSeparatedBoundarySweep(b *testing.B) {
	v := make([]geom.Vec3, 3000)
	edges := make([][]int, 1500)
	for i := range edges {
		v[2*i] = geom.Vec3{X: float64(3 * i)}
		v[2*i+1] = geom.Vec3{X: float64(3*i + 1), Y: 1}
		edges[i] = []int{2 * i, 2*i + 1}
	}
	for _, fast := range []bool{false, true} {
		name := "quadratic"
		if fast {
			name = "sweep"
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				fn := quadraticBoundaryReference
				if fast {
					fn = checkBoundaryPolylineEmbedding
				}
				n, e := fn(context.Background(), v, edges, 2000000)
				if e != nil || n != 1500*1499/2 {
					b.Fatal(n, e)
				}
			}
		})
	}
}
