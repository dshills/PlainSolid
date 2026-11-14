package trimmed

import (
	"context"
	"math/big"
	"plainsolid/geom"
)

// AlgebraicBoundaryEmbeddingOptions bounds all segment-pair work, including
// rejected broadphase pairs. MaxPairs defaults to2000000, at most16000000.
type AlgebraicBoundaryEmbeddingOptions struct {
	Boundary AlgebraicBoundaryMeshOptions
	MaxPairs int
}

// EmbeddedAlgebraicBoundaryMesh proves only the represented one-dimensional
// display has no intersections beyond indexed shared endpoints. The retained
// chord bounds and original source ownership do not prove source embedding,
// separation of curved arcs, surface filling, or an embedded solid.
type EmbeddedAlgebraicBoundaryMesh struct {
	Boundary   CertifiedAlgebraicBoundaryMesh
	PairChecks int
}

// TessellateEmbeddedBoundaryCertified generates the independently certified
// shared boundary, then checks every display-segment pair exactly. Collinear
// overlap, unindexed coincident endpoints and proper crossings are rejected.
// Even arbitrarily close skew segments remain disjoint; no distance tolerance
// is used. Errors return no partial network or embedding certificate.
func (s AlgebraicSubdivision) TessellateEmbeddedBoundaryCertified(ctx context.Context, o AlgebraicBoundaryEmbeddingOptions) (out EmbeddedAlgebraicBoundaryMesh, err error) {
	if ctx == nil || o.MaxPairs < 0 || o.MaxPairs > 16000000 {
		return out, geom.ErrInvalid
	}
	if o.MaxPairs == 0 {
		o.MaxPairs = 2000000
	}
	network, e := s.TessellateBoundaryCertified(ctx, o.Boundary)
	if e != nil {
		return out, e
	}
	checks, e := checkBoundaryPolylineEmbedding(ctx, network.Vertices, network.Edges, o.MaxPairs)
	if e != nil {
		return out, e
	}
	return EmbeddedAlgebraicBoundaryMesh{Boundary: network, PairChecks: checks}, nil
}

func checkBoundaryPolylineEmbedding(ctx context.Context, vertices []geom.Vec3, edges [][]int, maxPairs int) (int, error) {
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

func boundarySegmentsUnexpected(a, b, c, d [3]*big.Rat, shared *[3]*big.Rat) bool {
	u, v, w := ddSub(b, a), ddSub(d, c), ddSub(c, a)
	n := ddCross(u, v)
	den := ddDot(n, n)
	if den.Sign() != 0 {
		if ddDot(w, n).Sign() != 0 {
			return false
		}
		t := new(big.Rat).Quo(ddDot(ddCross(w, v), n), den)
		s := new(big.Rat).Quo(ddDot(ddCross(w, u), n), den)
		one := big.NewRat(1, 1)
		if t.Sign() < 0 || t.Cmp(one) > 0 || s.Sign() < 0 || s.Cmp(one) > 0 {
			return false
		}
		if shared == nil {
			return true
		}
		for k := range a {
			p := new(big.Rat).Add(a[k], new(big.Rat).Mul(t, u[k]))
			if p.Cmp((*shared)[k]) != 0 {
				return true
			}
		}
		return false
	}
	cross := ddCross(w, u)
	if ddDot(cross, cross).Sign() != 0 {
		return false
	}
	axis := 0
	for u[axis].Sign() == 0 {
		axis++
	}
	lo1, hi1 := a[axis], b[axis]
	if lo1.Cmp(hi1) > 0 {
		lo1, hi1 = hi1, lo1
	}
	lo2, hi2 := c[axis], d[axis]
	if lo2.Cmp(hi2) > 0 {
		lo2, hi2 = hi2, lo2
	}
	lo, hi := lo1, hi1
	if lo2.Cmp(lo) > 0 {
		lo = lo2
	}
	if hi2.Cmp(hi) < 0 {
		hi = hi2
	}
	if lo.Cmp(hi) > 0 {
		return false
	}
	if lo.Cmp(hi) < 0 {
		return true
	}
	return shared == nil || lo.Cmp((*shared)[axis]) != 0
}
